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

func parseMetrics(ctx context.Context, wg *sync.WaitGroup, metricsGen *runtimemetrics.RuntimeUpdate, t1 chan struct{}, t2 chan struct{}, t3 chan struct{}) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range t1 {
			if err := metricsGen.GetMetrics(ctx, metrics, true); err != nil {
				panic(err)
			}

		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range t2 {
			if err := metricsGen.GetMetrics(ctx, metrics, false); err != nil {
				panic(err)
			}

		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range t3 {
			metricsGen.GeneratorBatch(ctx)
		}
	}()

}

func workerInital(ctx context.Context, id int, jobs <-chan func()) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			log.Println("run worker ", id)
			for j := range jobs {
				log.Println("worker ", id, "run job")
				j()
				log.Println("complete job by worker", id)
			}
		}
	}

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

	var wg sync.WaitGroup

	tickerReport := time.NewTicker(time.Duration(f.ReportInterval) * time.Second)
	tickerPoll := time.NewTicker(time.Duration(f.PollInterval) * time.Second)

	defer tickerReport.Stop()
	defer tickerPoll.Stop()

	tickerPoll1 := make(chan struct{})
	tickerPoll2 := make(chan struct{})
	tickerReport1 := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		var wgIns sync.WaitGroup
		for range tickerReport.C {
			wgIns.Add(1)
			go func() {
				defer wgIns.Done()
				tickerPoll1 <- struct{}{}
			}()
			wgIns.Add(1)
			go func() {
				defer wgIns.Done()
				tickerPoll2 <- struct{}{}
			}()
			wgIns.Wait()
			wgIns.Add(1)
			go func() {
				defer wgIns.Done()
				tickerReport1 <- struct{}{}
			}()
			wgIns.Wait()
		}

	}()

	metricsGen := runtimemetrics.NewRuntimeUpdater(newService, f.RateLimit)

	parseMetrics(ctx, &wg, metricsGen, tickerPoll1, tickerPoll2, tickerReport1)

	jobs := make(chan func())

	wg.Add(1)
	go func() {
		defer wg.Done()
		var wgIt sync.WaitGroup
		for worker := range metricsGen.GetRateLimit() {
			wgIt.Add(1)
			workerIt := worker
			go workerInital(ctx, workerIt, jobs)
		}
		wgIt.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		curl := fmt.Sprintf("http://%v%v/updates/", f.GetHost(), f.GetPort())
	sendLoop:
		for {
			select {
			case <-ctx.Done():
				close(jobs)
				break sendLoop
			default:
				jobs <- func() {
					metricsGen.Sender(ctx, &wg, retryCfg, curl, f)
				}
			}
		}
	}()

	wg.Wait()
}
