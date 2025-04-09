package main

import (
	"gometrics/internal/clientflags"
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
	metricsGen := runtimemetrics.NewRuntimeUpdater(service.NewService(storage.NewMemStorage()))
	f := clientflags.InitialFlags()
	f.ParseFlags()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		metricsGen.GetLoopMetrics(f.PollInterval, metrics)
	}()

	go func() {
		defer wg.Done()
		metricsGen.SendMetrics(f.GetHost(), f.GetPort(), f.ReportInterval)
	}()

	wg.Wait()
}
