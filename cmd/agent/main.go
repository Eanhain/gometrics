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

// --- Constants ---
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

	// Channels
	pollCh1 := make(chan struct{})
	pollCh2 := make(chan struct{})
	reportCh := make(chan struct{})
	jobs := make(chan func(), cfg.RateLimit)

	var wg sync.WaitGroup
	pollInterval := time.Duration(cfg.PollInterval) * time.Second
	reportInterval := time.Duration(cfg.ReportInterval) * time.Second
	targetURL := fmt.Sprintf("http://%v%v/updates/", cfg.GetHost(), cfg.GetPort())

	// Task Factory
	senderTaskFactory := func() func() {
		return func() {
			metricsGen.Sender(ctx, targetURL, cfg)
		}
	}

	// 5. Start Background Processes
	runTickers(ctx, &wg, pollInterval, reportInterval, pollCh1, pollCh2, reportCh)

	// ВАЖНО: передаем jobs сюда, так как Dispatcher удален (он был источником бесконечных задач)
	runCollectors(ctx, &wg, metricsGen, pollCh1, pollCh2, reportCh, jobs, senderTaskFactory)

	runWorkerPool(ctx, &wg, cfg.RateLimit, jobs)

	// 6. Wait Shutdown
	waitShutdown(ctx, cancel, &wg)
}

// --- Helpers ---

func checkDependencies() error {
	if _, err := cpu.Percent(0, false); err != nil {
		return fmt.Errorf("cpu check failed: %w", err)
	}
	return nil
}

func initService(ctx context.Context) (*service.Service, error) {
	retryCfg := retry.DefaultConfig()
	retryCfg.OnRetry = func(err error, attempt int, delay time.Duration) {
		log.Printf("agent retry attempt %d failed: %v; next retry in %v", attempt, err, delay)
	}
	persistResult, err := retryCfg.Retry(ctx, func(args ...any) (any, error) {
		return persist.NewPersistStorage(args[0].(string), args[1].(int))
	}, "agent", -100)
	if err != nil {
		return nil, fmt.Errorf("init agent persist storage: %w", err)
	}
	agentPersist := persistResult.(*persist.PersistStorage)
	return service.NewService(storage.NewMemStorage(), agentPersist), nil
}

func runTickers(ctx context.Context, wg *sync.WaitGroup, pollI, reportI time.Duration, p1, p2, r chan<- struct{}) {
	tPoll := time.NewTicker(pollI)
	tReport := time.NewTicker(reportI)

	// Poll Ticker
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tPoll.Stop()
		defer close(p1)
		defer close(p2)

		for {
			select {
			case <-tPoll.C:
				var wgFanOut sync.WaitGroup
				wgFanOut.Add(2)
				// Safe send with select
				send := func(ch chan<- struct{}) {
					defer wgFanOut.Done()
					select {
					case ch <- struct{}{}:
					case <-ctx.Done():
					}
				}
				go send(p1)
				go send(p2)
				wgFanOut.Wait()
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
		defer close(r)

		for {
			select {
			case <-tReport.C:
				select {
				case r <- struct{}{}:
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
	inExt, inStd, inBatch <-chan struct{},
	jobs chan<- func(),
	taskFactory func() func(),
) {
	// 1. Ext Metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range inExt {
			if ctx.Err() != nil {
				return
			}
			if err := gen.GetMetrics(ctx, extMetrics, true); err != nil {
				log.Printf("err ext metrics: %v", err)
			}
		}
	}()

	// 2. Std Metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range inStd {
			if ctx.Err() != nil {
				return
			}
			if err := gen.GetMetrics(ctx, metrics, false); err != nil {
				log.Printf("err std metrics: %v", err)
			}
		}
	}()

	// 3. Batch & Schedule Task
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(jobs) // Closing jobs stops workers
		defer gen.CloseChannel(ctx)

		for range inBatch {
			if ctx.Err() != nil {
				return
			}

			if err := gen.GetMetricsBatch(ctx); err != nil {
				log.Printf("err batch: %v", err)
				continue
			}

			// Schedule Send Task (One per tick, not infinite loop!)
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
	log.Printf("Signal %v received. Shutdown...", sig)
	cancel()
	wg.Wait()
	log.Println("Agent stopped.")
}
