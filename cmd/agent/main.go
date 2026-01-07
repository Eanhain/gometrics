// Package main is the entry point for the metrics collection agent.
// It collects runtime metrics and sends them to the server periodically.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gometrics/internal/clientconfig"
	"gometrics/internal/persist"
	"gometrics/internal/retry"
	"gometrics/internal/runtimemetrics"
	"gometrics/internal/service"
	"gometrics/internal/storage"

	"github.com/shirou/gopsutil/v4/cpu"
)

// List of standard Go runtime metrics to collect
var metrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc", "HeapIdle",
	"HeapInuse", "HeapObjects", "HeapReleased", "HeapSys", "LastGC", "Lookups",
	"MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys", "Mallocs", "NextGC",
	"NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
	"Sys", "TotalAlloc",
}

// List of extended system metrics to collect (gopsutil)
var extMetrics = []string{
	"TotalMemory",
	"FreeMemory",
	"CPUutilization1",
}

// parseMetrics manages the concurrent collection of metrics based on triggers from tickers.
func parseMetrics(ctx context.Context, wg *sync.WaitGroup, metricsGen *runtimemetrics.RuntimeUpdate, t1 chan struct{}, t2 chan struct{}, t3 chan struct{}) {
	// Worker 1: Collect Extended Metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range t1 {
			select {
			case <-ctx.Done():
				log.Println("Graceful shutdown common metric sender")
				return
			default:
				if err := metricsGen.GetMetrics(ctx, extMetrics, true); err != nil {
					panic(err)
				}
				log.Println("read common metrics")
			}
		}
		<-ctx.Done()
		log.Println("Graceful shutdown common metric sender")
	}()

	// Worker 2: Collect Standard Runtime Metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range t2 {
			select {
			case <-ctx.Done():
				log.Println("Graceful shutdown ext metric sender")
				return
			default:
				if err := metricsGen.GetMetrics(ctx, metrics, false); err != nil {
					panic(err)
				}
				log.Println("read ext metrics")
			}
		}
		<-ctx.Done()
		log.Println("Graceful shutdown ext metric sender")
	}()

	// Worker 3: Generate Batches for Sending
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer metricsGen.CloseChannel(ctx) // Close channel when generator stops
		for range t3 {
			select {
			case <-ctx.Done():
				log.Println("Graceful shutdown metric generator")
				return
			default:
				// Prepares batches and puts them into ChIn
				if err := metricsGen.GetMetricsBatch(ctx); err != nil {
					panic(err)
				}
				log.Println("generate done")
			}
		}
		<-ctx.Done()
		log.Println("Graceful shutdown metric generator")
	}()
}

// workerInital executes sending jobs from the jobs channel.
func workerInital(ctx context.Context, wg *sync.WaitGroup, id int, jobs <-chan func()) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			log.Println("run worker ", id)
			for j := range jobs {
				j()
			}
			log.Println("jobs done", id)
		}
	}
}

func main() {
	// 0. Pre-check external dependencies
	if _, err := cpu.Percent(0, false); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 1. Initialize Retry Configuration
	retryCfg := retry.DefaultConfig()
	retryCfg.OnRetry = func(err error, attempt int, delay time.Duration) {
		log.Printf("agent retry attempt %d failed: %v; next retry in %v", attempt, err, delay)
	}

	// 2. Initialize Dummy Persistence (Agent doesn't persist to disk typically)
	persistResult, err := retryCfg.Retry(ctx, func(args ...any) (any, error) {
		path := args[0].(string)
		interval := args[1].(int)
		return persist.NewPersistStorage(path, interval)
	}, "agent", -100) // "agent" mode

	if err != nil {
		panic(fmt.Errorf("init agent persist storage: %w", err))
	}

	agentPersist := persistResult.(*persist.PersistStorage)
	newService := service.NewService(storage.NewMemStorage(), agentPersist)

	// 3. Parse Configuration Flags/Env
	f := clientconfig.InitialFlags()
	f.ParseFlags()

	var wg sync.WaitGroup

	// 4. Setup Tickers for Polling and Reporting
	tickerReport := time.NewTicker(time.Duration(f.ReportInterval) * time.Second)
	tickerPoll := time.NewTicker(time.Duration(f.PollInterval) * time.Second)

	// Channels to fan-out ticker events
	tickerPoll1 := make(chan struct{})
	tickerPoll2 := make(chan struct{})
	tickerReport1 := make(chan struct{})

	stop := make(chan os.Signal, 1)

	// Fan-out goroutine for Poll Ticker
	wg.Add(1)
	go func() {
		defer wg.Done()
		var wgIns sync.WaitGroup
		for {
			select {
			case <-tickerPoll.C:
				// Fan-out to two collectors (Standard + Extended)
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
			case <-ctx.Done():
				close(tickerPoll1)
				close(tickerPoll2)
				log.Println("ticker pool fanout closed")
				return
			}
		}
	}()

	// Fan-out goroutine for Report Ticker
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-tickerReport.C:
				tickerReport1 <- struct{}{}
			case <-ctx.Done():
				close(tickerReport1)
				log.Println("ticker report fanout closed")
				return
			}
		}
	}()

	// 5. Initialize Metrics Generator
	metricsGen := runtimemetrics.NewRuntimeUpdater(newService, f.RateLimit)

	// Start collection routines
	parseMetrics(ctx, &wg, metricsGen, tickerPoll1, tickerPoll2, tickerReport1)

	// 6. Start Worker Pool for Sending
	jobs := make(chan func(), f.RateLimit)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var wgIt sync.WaitGroup
		// Create workers based on RateLimit
		for worker := 0; worker < metricsGen.GetRateLimit(); worker++ {
			wgIt.Add(1)
			workerIt := worker
			go workerInital(ctx, &wgIt, workerIt, jobs)
		}
		wgIt.Wait()
		log.Println("all workers closed")
	}()

	// 7. Job Dispatcher
	// Creates jobs to send metrics when triggered by Report Ticker (via metricsGen logic indirectly)
	// Actually, metricsGen.Sender is called here continuously?
	// Note: Logic here seems to push jobs into channel.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(jobs)
		curl := fmt.Sprintf("http://%v%v/updates/", f.GetHost(), f.GetPort())
	sendLoop:
		for {
			select {
			case <-ctx.Done():
				break sendLoop
			// Push a sender job. This blocks until a worker is free.
			// Ideally this should consume from metricsGen.ChIn?
			// The metricsGen.Sender reads from ChIn. So we launch it as a job.
			case jobs <- func() {
				metricsGen.Sender(ctx, curl, f)
			}:
			}
		}
		log.Println("jobs sender closed")
	}()

	// 8. Graceful Shutdown
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("Graceful shutdown is initialized")
	cancel() // Cancel context to stop all goroutines
	tickerReport.Stop()
	tickerPoll.Stop()

	wg.Wait() // Wait for cleanup
}
