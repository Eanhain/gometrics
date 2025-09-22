package main

import (
	"gometrics/internal/clientconfig"
	"gometrics/internal/persist"
	"gometrics/internal/runtimemetrics"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"sync"
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
	agentPersist := persist.NewPersistStorage("agent")
	newService := service.NewService(storage.NewMemStorage(), agentPersist)
	metricsGen := runtimemetrics.NewRuntimeUpdater(newService)
	f := clientconfig.InitialFlags()
	f.ParseFlags()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		metricsGen.GetLoopMetrics(f.PollInterval, metrics)
	}()

	go func() {
		defer wg.Done()
		err := metricsGen.SendMetrics(f.GetHost(), f.GetPort(), f.ReportInterval, f.Compress)
		if err != nil {
			panic(err)
		}
	}()

	wg.Wait()
}
