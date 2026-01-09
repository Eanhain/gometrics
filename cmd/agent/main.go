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

	"gometrics/configs"
	"gometrics/internal/clientconfig"
	"gometrics/internal/persist"
	"gometrics/internal/retry"
	"gometrics/internal/runtimemetrics"
	"gometrics/internal/service"
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
	// 1. Предварительная проверка окружения
	if err := checkDependencies(); err != nil {
		panic(err)
	}

	// 2. Инициализация конфигурации
	// Используем ваш пакет clientconfig
	cfg := clientconfig.InitialFlags()
	cfg.ParseFlags()

	// 3. Инициализация контекста и сервисного слоя
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc, err := initService(ctx)
	if err != nil {
		panic(err)
	}

	// 4. Подготовка каналов и генератора метрик
	metricsGen := runtimemetrics.NewRuntimeUpdater(svc, cfg.RateLimit)

	// Каналы для сигналов от тикеров
	pollCh1 := make(chan struct{})
	pollCh2 := make(chan struct{})
	reportCh := make(chan struct{})

	// Канал задач для воркеров
	jobs := make(chan func(), cfg.RateLimit)

	var wg sync.WaitGroup

	// Подготовка интервалов и URL
	pollInterval := time.Duration(cfg.PollInterval) * time.Second
	reportInterval := time.Duration(cfg.ReportInterval) * time.Second
	targetURL := fmt.Sprintf("http://%v%v/updates/", cfg.GetHost(), cfg.GetPort())

	// 5. Запуск фоновых процессов

	// Тикеры (отвечают за тайминг)
	runTickers(ctx, &wg, pollInterval, reportInterval, pollCh1, pollCh2, reportCh)

	// Сборщики метрик (реагируют на тики сбора)
	runCollectors(ctx, &wg, metricsGen, pollCh1, pollCh2, reportCh)

	// Пул воркеров (обрабатывает очередь jobs)
	runWorkerPool(ctx, &wg, cfg.RateLimit, jobs)

	// Диспетчер задач (создает job для отправки по тику reportCh)
	// Замыкание захватывает cfg и targetURL
	senderTaskFactory := func() func() {
		return func() {
			metricsGen.Sender(ctx, targetURL, cfg)
		}
	}
	runJobDispatcher(ctx, &wg, jobs, senderTaskFactory)

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

	// Инициализация "заглушки" персистентности (как в исходном коде)
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

// runTickers запускает таймеры и распределяет сигналы по каналам
func runTickers(
	ctx context.Context,
	wg *sync.WaitGroup,
	pollInterval, reportInterval time.Duration,
	poll1, poll2, report chan<- struct{},
) {
	tickerPoll := time.NewTicker(pollInterval)
	tickerReport := time.NewTicker(reportInterval)

	// Fan-out для Poll тикера (раздает сигнал в два канала)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tickerPoll.Stop()
		defer close(poll1)
		defer close(poll2)

		for {
			select {
			case <-tickerPoll.C:
				var wgFanOut sync.WaitGroup
				wgFanOut.Add(2)
				go func() { defer wgFanOut.Done(); poll1 <- struct{}{} }()
				go func() { defer wgFanOut.Done(); poll2 <- struct{}{} }()
				wgFanOut.Wait()
			case <-ctx.Done():
				log.Println("ticker pool fanout closed")
				return
			}
		}
	}()

	// Fan-out для Report тикера
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer tickerReport.Stop()
		defer close(report)

		for {
			select {
			case <-tickerReport.C:
				report <- struct{}{}
			case <-ctx.Done():
				log.Println("ticker report fanout closed")
				return
			}
		}
	}()
}

// runCollectors слушает каналы сигналов и запускает сбор метрик
func runCollectors(
	ctx context.Context,
	wg *sync.WaitGroup,
	metricsGen *runtimemetrics.RuntimeUpdate,
	inExt, inStd, inBatch <-chan struct{},
) {
	// Worker 1: Сбор расширенных метрик (gopsutil)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range inExt {
			if ctx.Err() != nil {
				return
			}
			if err := metricsGen.GetMetrics(ctx, extMetrics, true); err != nil {
				log.Printf("error getting ext metrics: %v", err)
			}
			log.Println("read common metrics")
		}
		log.Println("Graceful shutdown common metric sender")
	}()

	// Worker 2: Сбор стандартных метрик Go
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
			log.Println("read ext metrics")
		}
		log.Println("Graceful shutdown ext metric sender")
	}()

	// Worker 3: Генерация батчей для отправки
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer metricsGen.CloseChannel(ctx)
		for range inBatch {
			if ctx.Err() != nil {
				return
			}
			if err := metricsGen.GetMetricsBatch(ctx); err != nil {
				log.Printf("error batching metrics: %v", err)
			}
			log.Println("generate done")
		}
		log.Println("Graceful shutdown metric generator")
	}()
}

// runWorkerPool запускает фиксированное количество воркеров
func runWorkerPool(ctx context.Context, wg *sync.WaitGroup, limit int, jobs <-chan func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		var wgWorkers sync.WaitGroup

		for i := 0; i < limit; i++ {
			wgWorkers.Add(1)
			go func(id int) {
				defer wgWorkers.Done()
				workerPayload(ctx, id, jobs)
			}(i)
		}
		wgWorkers.Wait()
		log.Println("all workers closed")
	}()
}

func workerPayload(ctx context.Context, id int, jobs <-chan func()) {
	log.Printf("worker %d started", id)
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}
			job()
		}
	}
}

// runJobDispatcher создает задачи на отправку и кладет их в канал jobs
func runJobDispatcher(
	ctx context.Context,
	wg *sync.WaitGroup,
	jobs chan<- func(),
	taskFactory func() func(),
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(jobs)

		for {
			select {
			case <-ctx.Done():
				log.Println("jobs sender closed")
				return
			// Создаем задачу и пытаемся положить в канал
			case jobs <- taskFactory():
				// Задача успешно добавлена в очередь
			}
		}
	}()
}

func waitShutdown(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	log.Println("Graceful shutdown is initialized")
	cancel() // Отменяем контекст, это сигнал всем горутинам на выход

	wg.Wait()
	log.Println("Application stopped")
}
