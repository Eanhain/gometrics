package main

import (
	"context"
	"fmt"
	"gometrics/internal/clientconfig"
	"gometrics/internal/persist"
	"gometrics/internal/runtimemetrics"
	"gometrics/internal/service"
	"gometrics/internal/storage"
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
	agentPersist, err := persist.NewPersistStorage("agent", -100)
	if err != nil {
		panic(fmt.Errorf("init agent persist storage: %w", err))
	}
	newService := service.NewService(storage.NewMemStorage(), agentPersist)
	metricsGen := runtimemetrics.NewRuntimeUpdater(newService)
	f := clientconfig.InitialFlags()
	f.ParseFlags()
	ctx := context.Background()

	defer ctx.Done()
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
			if err := metricsGen.SendMetrics(ctx, ticker, f.GetHost(), f.GetPort(), f.Compress); err != nil {
				panic(fmt.Errorf("send metrics to %s:%s: %w", f.GetHost(), f.GetPort(), err))
			}
		}
	}()

	wg.Wait()
}
