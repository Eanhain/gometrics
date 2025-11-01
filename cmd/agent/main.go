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
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

var metrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys",
	"HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased",
	"HeapSys", "LastGC", "Lookups", "MCacheInuse", "MCacheSys",
	"MSpanInuse", "MSpanSys", "Mallocs", "NextGC", "NumForcedGC",
	"NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
	"Sys", "TotalAlloc",
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

	// Контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

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

	// Горутина 1: сбор extended метрик
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(f.PollInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metricsGen.ParseMetrics(ctx, f, extMetrics, true)
			}
		}
	}()

	// Горутина 2: сбор runtime метрик
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(f.PollInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metricsGen.ParseMetrics(ctx, f, metrics, false)
			}
		}
	}()

	// Горутина 3: отправка метрик
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(f.ReportInterval) * time.Second)
		defer ticker.Stop()
		curl := fmt.Sprintf("http://%v%v/updates/", f.GetHost(), f.GetPort())

		// WaitGroup для воркеров отправки
		var sendersWg sync.WaitGroup

		for {
			select {
			case <-ctx.Done():
				// Ждем завершения всех воркеров перед выходом
				sendersWg.Wait()
				return
			case <-ticker.C:
				// Добавляем воркеров ДО запуска горутин
				rateLimit := metricsGen.GetRateLimit()
				for worker := 0; worker < rateLimit; worker++ {
					sendersWg.Add(1)
					workerIt := worker
					go func(workerID int) {
						defer sendersWg.Done()
						metricsGen.Sender(ctx, workerID, retryCfg, curl, f)
					}(workerIt)
				}
			}
		}
	}()

	// Ждем сигнала завершения
	go func() {
		<-sigChan
		log.Println("Получен сигнал завершения, останавливаем agent...")
		cancel() // Отменяем контекст, чтобы все горутины завершились
	}()

	// Ждем завершения всех горутин
	wg.Wait()
	log.Println("Agent остановлен")
}
