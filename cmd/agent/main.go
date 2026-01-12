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

var extMetrics = []string{
	"TotalMemory", "FreeMemory", "CPUutilization1",
}

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

	pollCh1 := make(chan struct{})
	pollCh2 := make(chan struct{})
	reportCh := make(chan struct{})

	var wg sync.WaitGroup

	pollInterval := time.Duration(cfg.PollInterval) * time.Second
	reportInterval := time.Duration(cfg.ReportInterval) * time.Second
	targetURL := fmt.Sprintf("http://%v%v/updates/", cfg.GetHost(), cfg.GetPort())

	runTickers(ctx, &wg, pollInterval, reportInterval, pollCh1, pollCh2, reportCh)
	runCollectors(ctx, &wg, metricsGen, pollCh1, pollCh2, reportCh)

	for i := 0; i < cfg.RateLimit; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			log.Printf("sender worker %d started", id)
			if err := metricsGen.SendMetricGobCh(ctx, targetURL, cfg.Compress, cfg.Key); err != nil {
				log.Printf("sender worker %d error: %v", id, err)
			}
			log.Printf("sender worker %d stopped", id)
		}(i)
	}

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
		log.Printf("agent retry attempt %d failed: %v; next retry in %v", attempt, err, delay)
	}

	persistResult, err := retryCfg.Retry(ctx, func(args ...any) (any, error) {
		path := args[0].(string)
		interval := args[1].(int)
		return persist.NewPersistStorage(path, interval)
	}, "agent", -100)

	if err != nil {
		return nil, fmt.Errorf("init agent persist storage: %w", err)
	}

	agentPersist := persistResult.(*persist.PersistStorage)
	return service.NewService(storage.NewMemStorage(), agentPersist), nil
}

func runTickers(
	ctx context.Context,
	wg *sync.WaitGroup,
	pollInterval, reportInterval time.Duration,
	poll1, poll2, report chan<- struct{},
) {
	tickerPoll := time.NewTicker(pollInterval)
	tickerReport := time.NewTicker(reportInterval)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tickerPoll.Stop()
		defer close(poll1)
		defer close(poll2)

		for {
			select {
			case <-tickerPoll.C:
				var wgFanOut sync.WaitGroup
				wgFanOut.Add(2)
				go func() { defer wgFanOut.Done(); poll1 <- struct{}{} }()
				go func() { defer wgFanOut.Done(); poll2 <- struct{}{} }()
				wgFanOut.Wait()
			case <-ctx.Done():
				log.Println("ticker pool fanout closed")
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tickerReport.Stop()
		defer close(report)

		for {
			select {
			case <-tickerReport.C:
				report <- struct{}{}
			case <-ctx.Done():
				log.Println("ticker report fanout closed")
				return
			}
		}
	}()
}

func runCollectors(
	ctx context.Context,
	wg *sync.WaitGroup,
	metricsGen *runtimemetrics.RuntimeUpdate,
	inExt, inStd, inBatch <-chan struct{},
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range inExt {
			if ctx.Err() != nil {
				return
			}
			if err := metricsGen.GetMetrics(ctx, extMetrics, true); err != nil {
				log.Printf("error getting ext metrics: %v", err)
			}
			log.Println("read common metrics")
		}
		log.Println("Graceful shutdown common metric sender")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range inStd {
			if ctx.Err() != nil {
				return
			}
			if err := metricsGen.GetMetrics(ctx, metrics, false); err != nil {
				log.Printf("error getting std metrics: %v", err)
			}
			log.Println("read ext metrics")
		}
		log.Println("Graceful shutdown ext metric sender")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer metricsGen.CloseChannel(ctx)
		for range inBatch {
			if ctx.Err() != nil {
				return
			}
			if err := metricsGen.GetMetricsBatch(ctx); err != nil {
				log.Printf("error batching metrics: %v", err)
			}
			log.Println("generate done")
		}
		log.Println("Graceful shutdown metric generator")
	}()
}

func waitShutdown(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	log.Println("Graceful shutdown is initialized")
	cancel()

	wg.Wait()
	log.Println("Application stopped")
}
