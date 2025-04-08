package main

import (
	"gometrics/internal/confclient"
	"gometrics/internal/runtimemetrics"
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

	newStorage := storage.NewMemStorage()
	metricsGen := runtimemetrics.NewRuntimeUpdater(newStorage)
	f := confclient.InitialFlags()
	f.ParseFlags()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		metricsGen.GetLoopMetrics(f.PollInterval, metrics)
	}()

	go func() {
		defer wg.Done()
		metricsGen.SendMetrics(f.Addr.GetAddr(), f.Addr.GetPort(), f.ReportInterval)
	}()

	wg.Wait()
}
