package main

import (
	"errors"
	"flag"
	"gometrics/internal/runtimemetrics"
	"gometrics/internal/storage"
	"strings"
	"sync"
)

var ErrNotCorrect = errors.New("неправильно введен host:port")

type Address struct {
	ReportInterval int
	PollInterval   int
	addr           string
}

func (o *Address) parseFlags() {
	flag.IntVar(&o.ReportInterval, "r", 10, "Send to server interval")
	flag.IntVar(&o.PollInterval, "p", 2, "Refresh metrics interval")
	flag.StringVar(&o.addr, "a", "localhost:8080", "Refresh metrics interval")
	flag.Parse()
}

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

	inter := new(Address)
	inter.parseFlags()

	addr := strings.Split(inter.addr, ":")
	if len(addr) != 2 {
		panic(ErrNotCorrect)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		metricsGen.GetLoopMetrics(inter.PollInterval, metrics)
	}()

	go func() {
		defer wg.Done()
		metricsGen.SendMetrics(addr[0], ":"+addr[1], inter.ReportInterval)
	}()

	wg.Wait()
}
