package main

import (
	"context"
	"fmt"
	"gometrics/internal/clientconfig"
	"gometrics/internal/persist"
	"gometrics/internal/retry"
	"gometrics/internal/runtimemetrics"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"log"
	"sync"
	"time"
)

func main() {
	metrics := []string{
		"Alloc",
		"BuckHashSys",
		"Frees",
		"GCCPUFraction",
		"GCSys",
		"HeapAlloc",
		"HeapIdle",
		"HeapInuse",
		"HeapObjects",
		"HeapReleased",
		"HeapSys",
		"LastGC",
		"Lookups",
		"MCacheInuse",
		"MCacheSys",
		"MSpanInuse",
		"MSpanSys",
		"Mallocs",
		"NextGC",
		"NumForcedGC",
		"NumGC",
		"OtherSys",
		"PauseTotalNs",
		"StackInuse",
		"StackSys",
		"Sys",
		"TotalAlloc",
	}
	ctx := context.Background()

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
		panic(fmt.Errorf("init agent persist storage: %w", err))
	}
	agentPersist := persistResult.(*persist.PersistStorage)
	newService := service.NewService(storage.NewMemStorage(), agentPersist)
	metricsGen := runtimemetrics.NewRuntimeUpdater(newService)
	f := clientconfig.InitialFlags()
	f.ParseFlags()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(f.PollInterval) * time.Second)
		defer ticker.Stop()
		for {
			if err := metricsGen.GetMetrics(ctx, ticker, metrics); err != nil {
				panic(fmt.Errorf("runtime metrics loop: %w", err))
			}
		}
	}()

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(f.ReportInterval) * time.Second)
		defer ticker.Stop()
		for {
			if _, err := retryCfg.Retry(ctx, func(_ ...any) (any, error) {
				return nil, metricsGen.SendMetricsGob(ctx, ticker, f.GetHost(), f.GetPort(), f.Compress, f.Key)
			}); err != nil {
				panic(fmt.Errorf("send metrics to %s:%s: %w", f.GetHost(), f.GetPort(), err))
			}
		}
	}()

	wg.Wait()
}
