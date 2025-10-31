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

	"github.com/shirou/gopsutil/v4/cpu"
)

var metrics = []string{
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

var extMetrics = []string{
	"TotalMemory",
	"FreeMemory",
	"CPUutilization1",
}

func main() {

	if _, err := cpu.Percent(0, false); err != nil {
		panic(err)
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

	f := clientconfig.InitialFlags()
	f.ParseFlags()
	metricsGen := runtimemetrics.NewRuntimeUpdater(newService, f.RateLimit)

	var wg sync.WaitGroup

	// go func() {
	// 	defer wg.Done()
	// 	ticker := time.NewTicker(time.Duration(f.PollInterval) * time.Second)
	// 	defer ticker.Stop()
	// 	for range ticker.C {
	// 		metricsGen.ParseMetrics(ctx, f, extMetrics, true)
	// 	}
	// }()

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(f.PollInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			metricsGen.ParseMetrics(ctx, f, metrics, false)
		}
	}()

	wg.Add(1)
	go func() {
		ticker := time.NewTicker(time.Duration(f.ReportInterval) * time.Second)
		curl := fmt.Sprintf("http://%v%v/updates/", f.GetHost(), f.GetPort())
		defer ticker.Stop()
		for range ticker.C {
			for worker := range metricsGen.GetRateLimit() {
				wg.Add(1)
				workerIt := worker
				go metricsGen.Sender(ctx, &wg, workerIt, ticker, retryCfg, curl, f)
			}
		}
	}()

	wg.Wait()
}
