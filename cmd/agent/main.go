package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gometrics/configs"
	"gometrics/internal/clientconfig"
	"gometrics/internal/persist"
	"gometrics/internal/retry"
	"gometrics/internal/runtimemetrics"
	"gometrics/internal/service"
	"gometrics/internal/signature"
	"gometrics/internal/storage"

	"github.com/shirou/gopsutil/v4/cpu"
)

var metrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc", "HeapIdle",
	"HeapInuse", "HeapObjects", "HeapReleased", "HeapSys", "LastGC", "Lookups",
	"MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys", "Mallocs", "NextGC",
	"NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
	"Sys", "TotalAlloc",
}
var extMetrics = []string{"TotalMemory", "FreeMemory", "CPUutilization1"}

func main() {
	fmt.Println(configs.BuildVerPrint())
	if err := checkDependencies(); err != nil {
		panic(err)
	}

	cfg := clientconfig.InitialFlags()
	cfg.ParseFlags()

	var pubKey *rsa.PublicKey
	if cfg.CryptoKey != "" {
		var err error
		pubKey, err = signature.GetRSAPubKey(cfg.CryptoKey)
		if err != nil {
			panic(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := initService(ctx)
	if err != nil {
		panic(err)
	}

	metricsGen := runtimemetrics.NewRuntimeUpdater(svc, cfg.RateLimit, pubKey)

	// ИЗМЕНЕНИЕ: Один канал для поллинга, чтобы избежать гонки
	pollCh := make(chan struct{})
	reportCh := make(chan struct{})
	jobs := make(chan func(), cfg.RateLimit)

	var wg sync.WaitGroup
	pollInterval := time.Duration(cfg.PollInterval) * time.Second
	reportInterval := time.Duration(cfg.ReportInterval) * time.Second
	targetURL := fmt.Sprintf("http://%v%v/updates/", cfg.GetHost(), cfg.GetPort())

	senderTaskFactory := func() func() {
		return func() {
			metricsGen.Sender(ctx, targetURL, cfg)
		}
	}

	// Запускаем
	runTickers(ctx, &wg, pollInterval, reportInterval, pollCh, reportCh)

	// Передаем один pollCh
	runCollectors(ctx, &wg, metricsGen, pollCh, reportCh, jobs, senderTaskFactory)

	runWorkerPool(ctx, &wg, cfg.RateLimit, jobs)

	waitShutdown(ctx, cancel, &wg)
}

func checkDependencies() error {
	if _, err := cpu.Percent(0, false); err != nil {
		return fmt.Errorf("cpu check failed: %w", err)
	}
	return nil
}

func initService(ctx context.Context) (*service.Service, error) {
	retryCfg := retry.DefaultConfig()
	retryCfg.OnRetry = func(err error, attempt int, delay time.Duration) {
		log.Printf("agent retry attempt %d failed: %v", attempt, err)
	}
	persistResult, err := retryCfg.Retry(ctx, func(args ...any) (any, error) {
		return persist.NewPersistStorage(args[0].(string), args[1].(int))
	}, "agent", -100)
	if err != nil {
		return nil, fmt.Errorf("init agent persist: %w", err)
	}
	return service.NewService(storage.NewMemStorage(), persistResult.(*persist.PersistStorage)), nil
}

func runTickers(ctx context.Context, wg *sync.WaitGroup, pollI, reportI time.Duration, poll, report chan<- struct{}) {
	tPoll := time.NewTicker(pollI)
	tReport := time.NewTicker(reportI)

	// Poll Ticker
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tPoll.Stop()
		defer close(poll)

		send := func() {
			select {
			case poll <- struct{}{}:
			case <-ctx.Done():
			}
		}

		// ВАЖНО: Мгновенный первый сбор для тестов
		send()

		for {
			select {
			case <-tPoll.C:
				send()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Report Ticker
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tReport.Stop()
		defer close(report)

		for {
			select {
			case <-tReport.C:
				select {
				case report <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func runCollectors(
	ctx context.Context,
	wg *sync.WaitGroup,
	gen *runtimemetrics.RuntimeUpdate,
	inPoll, inReport <-chan struct{}, // Один канал inPoll
	jobs chan<- func(),
	taskFactory func() func(),
) {
	// Worker 1: ПОСЛЕДОВАТЕЛЬНЫЙ сбор метрик
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range inPoll {
			if ctx.Err() != nil {
				return
			}

			// 1. Ext metrics
			if err := gen.GetMetrics(ctx, extMetrics, true); err != nil {
				log.Printf("err ext: %v", err)
			}
			// 2. Std metrics (тут обновляется PollCount)
			// Выполняется строго после Ext, никакой гонки!
			if err := gen.GetMetrics(ctx, metrics, false); err != nil {
				log.Printf("err std: %v", err)
			}
		}
	}()

	// Worker 2: Batch Generator
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(jobs)
		defer gen.CloseChannel(ctx)

		for range inReport {
			if ctx.Err() != nil {
				return
			}

			if err := gen.GetMetricsBatch(ctx); err != nil {
				log.Printf("err batch: %v", err)
				continue
			}

			select {
			case jobs <- taskFactory():
				log.Println("send task scheduled")
			case <-ctx.Done():
				return
			}
		}
	}()
}

func runWorkerPool(ctx context.Context, wg *sync.WaitGroup, limit int, jobs <-chan func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		var wgW sync.WaitGroup
		for i := 0; i < limit; i++ {
			wgW.Add(1)
			go func(id int) {
				defer wgW.Done()
				log.Printf("worker %d started", id)
				for job := range jobs {
					if ctx.Err() != nil {
						return
					}
					job()
				}
				log.Printf("worker %d stopped", id)
			}(i)
		}
		wgW.Wait()
		log.Println("worker pool stopped")
	}()
}

func waitShutdown(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Printf("Signal %v. Shutdown...", sig)
	cancel()
	wg.Wait()
	log.Println("Agent stopped.")
}
