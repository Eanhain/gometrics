package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gometrics/configs"
	"gometrics/internal/clientconfig"
	"gometrics/internal/persist"
	"gometrics/internal/retry"
	"gometrics/internal/runtimemetrics"
	"gometrics/internal/service"
	"gometrics/internal/signature"
	"gometrics/internal/storage"

	"github.com/shirou/gopsutil/v4/cpu"
)

// --- Constants ---

var metrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc", "HeapIdle",
	"HeapInuse", "HeapObjects", "HeapReleased", "HeapSys", "LastGC", "Lookups",
	"MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys", "Mallocs", "NextGC",
	"NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
	"Sys", "TotalAlloc",
}

var extMetrics = []string{
	"TotalMemory", "FreeMemory", "CPUutilization1",
}

// --- Main Entry Point ---

func main() {
	fmt.Println(configs.BuildVerPrint())

	if err := checkDependencies(); err != nil {
		panic(err)
	}

	cfg := clientconfig.InitialFlags()
	cfg.ParseFlags()

	var pubKey *rsa.PublicKey
	if cfg.CryptoKey != "" {
		var err error
		pubKey, err = signature.GetRSAPubKey(cfg.CryptoKey)
		if err != nil {
			panic(err)
		}
	}

	// 3. Инициализация контекста
	// Используем cancel для сигналов остановки всем горутинам
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := initService(ctx)
	if err != nil {
		panic(err)
	}

	metricsGen := runtimemetrics.NewRuntimeUpdater(svc, cfg.RateLimit, pubKey)

	// Каналы
	pollCh1 := make(chan struct{})
	pollCh2 := make(chan struct{})
	reportCh := make(chan struct{})
	jobs := make(chan func(), cfg.RateLimit)

	var wg sync.WaitGroup

	pollInterval := time.Duration(cfg.PollInterval) * time.Second
	reportInterval := time.Duration(cfg.ReportInterval) * time.Second
	targetURL := fmt.Sprintf("http://%v%v/updates/", cfg.GetHost(), cfg.GetPort())

	// Задача на отправку (Task Factory)
	senderTaskFactory := func() func() {
		return func() {
			metricsGen.Sender(ctx, targetURL, cfg)
		}
	}

	// 5. Запуск фоновых процессов

	// Тикеры
	runTickers(ctx, &wg, pollInterval, reportInterval, pollCh1, pollCh2, reportCh)

	// Сборщики метрик + Генератор задач для воркеров
	// Мы передаем jobs сюда, чтобы задачи создавались по тику reportCh, а не в бесконечном цикле
	runCollectors(ctx, &wg, metricsGen, pollCh1, pollCh2, reportCh, jobs, senderTaskFactory)

	// Пул воркеров
	runWorkerPool(ctx, &wg, cfg.RateLimit, jobs)

	// 6. Ожидание завершения
	waitShutdown(ctx, cancel, &wg)
}

// --- Initialization Helpers ---

func checkDependencies() error {
	if _, err := cpu.Percent(0, false); err != nil {
		return fmt.Errorf("cpu check failed: %w", err)
	}
	return nil
}

func initService(ctx context.Context) (*service.Service, error) {
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
		return nil, fmt.Errorf("init agent persist storage: %w", err)
	}

	agentPersist := persistResult.(*persist.PersistStorage)
	return service.NewService(storage.NewMemStorage(), agentPersist), nil
}

// --- Concurrency Helpers ---

func runTickers(
	ctx context.Context,
	wg *sync.WaitGroup,
	pollInterval, reportInterval time.Duration,
	poll1, poll2, report chan<- struct{},
) {
	tickerPoll := time.NewTicker(pollInterval)
	tickerReport := time.NewTicker(reportInterval)

	// Fan-out Ticker Poll
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tickerPoll.Stop()
		// Закрываем каналы при выходе, чтобы runCollectors вышли из range циклов
		defer close(poll1)
		defer close(poll2)

		for {
			select {
			case <-tickerPoll.C:
				var wgFanOut sync.WaitGroup
				wgFanOut.Add(2)

				// ИСПРАВЛЕНИЕ: Используем select с ctx.Done() при отправке.
				// Если получатель умер, мы не зависнем.
				sendSafe := func(ch chan<- struct{}) {
					defer wgFanOut.Done()
					select {
					case ch <- struct{}{}:
					case <-ctx.Done(): // Выход, если контекст отменен
					}
				}

				go sendSafe(poll1)
				go sendSafe(poll2)

				wgFanOut.Wait()
			case <-ctx.Done():
				log.Println("ticker pool fanout closed")
				return
			}
		}
	}()

	// Ticker Report
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tickerReport.Stop()
		defer close(report)

		for {
			select {
			case <-tickerReport.C:
				// ИСПРАВЛЕНИЕ: Безопасная отправка
				select {
				case report <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				log.Println("ticker report fanout closed")
				return
			}
		}
	}()
}

func runCollectors(
	ctx context.Context,
	wg *sync.WaitGroup,
	metricsGen *runtimemetrics.RuntimeUpdate,
	inExt, inStd, inBatch <-chan struct{},
	jobs chan<- func(), // Добавили канал для задач
	taskFactory func() func(), // Фабрика задач
) {
	// Worker 1: Ext metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range inExt {
			// Проверка контекста не обязательна внутри range (цикл сам прервется при закрытии канала),
			// но полезна для раннего выхода, если операция тяжелая.
			if ctx.Err() != nil {
				return
			}

			if err := metricsGen.GetMetrics(ctx, extMetrics, true); err != nil {
				log.Printf("error getting ext metrics: %v", err)
			}
		}
		log.Println("Graceful shutdown: ext metric collector stopped")
	}()

	// Worker 2: Std metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range inStd {
			if ctx.Err() != nil {
				return
			}
			if err := metricsGen.GetMetrics(ctx, metrics, false); err != nil {
				log.Printf("error getting std metrics: %v", err)
			}
		}
		log.Println("Graceful shutdown: std metric collector stopped")
	}()

	// Worker 3: Batch generator AND Job submitter
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Важно: закрываем jobs только когда этот воркер (поставщик задач) закончил работу
		defer close(jobs)
		defer metricsGen.CloseChannel(ctx)

		for range inBatch {
			if ctx.Err() != nil {
				return
			}

			// 1. Подготовка батча
			if err := metricsGen.GetMetricsBatch(ctx); err != nil {
				log.Printf("error batching metrics: %v", err)
				continue
			}

			// 2. ИСПРАВЛЕНИЕ: Создание задачи на отправку ТОЛЬКО по тику
			// Ранее runJobDispatcher спамил задачами. Теперь мы ставим задачу только когда готов батч.
			select {
			case jobs <- taskFactory():
				log.Println("send task scheduled")
			case <-ctx.Done():
				return
			}
		}
		log.Println("Graceful shutdown: batch generator stopped")
	}()
}

func runWorkerPool(ctx context.Context, wg *sync.WaitGroup, limit int, jobs <-chan func()) {
	// Запускаем limit воркеров
	wg.Add(1)
	go func() {
		defer wg.Done()
		var wgWorkers sync.WaitGroup

		for i := 0; i < limit; i++ {
			wgWorkers.Add(1)
			go func(id int) {
				defer wgWorkers.Done()
				log.Printf("worker %d started", id)

				// Читаем из jobs пока канал не закроется (в runCollectors)
				for job := range jobs {
					// Доп. проверка контекста на случай долгой работы
					if ctx.Err() != nil {
						return
					}
					job()
				}
				log.Printf("worker %d stopped", id)
			}(i)
		}
		wgWorkers.Wait()
		log.Println("worker pool shutdown complete")
	}()
}

func waitShutdown(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Блокируемся до сигнала ОС
	sig := <-stop
	log.Printf("Received signal %v. Graceful shutdown initialized...", sig)

	// 1. Отменяем контекст -> Tickers останавливаются -> Каналы закрываются
	cancel()

	// 2. Ждем пока все горутины (collectors, workers) завершат работу
	wg.Wait()

	log.Println("Application stopped successfully")
}
