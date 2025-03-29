package main

import (
	"gometrics/internal/runtimegen"
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
	metricsGen := runtimegen.NewRuntimeUpdater(newStorage)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		metricsGen.GetLoopMetrics(2, metrics)
	}()

	go func() {
		defer wg.Done()
		metricsGen.SendMetrics("localhost", ":8080", 10)
	}()

	wg.Wait()
}
