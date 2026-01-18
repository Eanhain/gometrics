// Package main is the entry point for the metrics collection server.
// It initializes storage, configuration, logging, and starts the HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof" // Import pprof for profiling
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"

	"gometrics/configs"
	myCompress "gometrics/internal/compress"
	"gometrics/internal/db"
	"gometrics/internal/handlers"
	"gometrics/internal/logger"
	"gometrics/internal/persist"
	"gometrics/internal/retry"
	"gometrics/internal/serverconfig"
	"gometrics/internal/service"
	"gometrics/internal/signature"
	"gometrics/internal/storage"
	"gometrics/internal/trustedsubnet" // Новый импорт для middleware
	_ "gometrics/swagger"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// @title           GoMetrics API
// @version         1.0
// @description     API service for collecting runtime metrics.
// @termsOfService  [http://swagger.io/terms/](http://swagger.io/terms/)

// @contact.name    API Support
// @contact.url     [http://www.swagger.io/support](http://www.swagger.io/support)
// @contact.email   [support@swagger.io](mailto:support@swagger.io)

// @license.name    Apache 2.0
// @license.url     [http://www.apache.org/licenses/LICENSE-2.0.html](http://www.apache.org/licenses/LICENSE-2.0.html)

// @host            localhost:8080
// @BasePath        /

// shutdownTimeout определяет максимальное время ожидания graceful shutdown
const shutdownTimeout = 10 * time.Second

func main() {
	fmt.Println(configs.BuildVerPrint())
	// 1. Initialize configuration
	f := serverconfig.InitialFlags()
	f.ParseFlags()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Initialize Logger
	newLogger, err := logger.CreateLoggerRequest()
	if err != nil {
		panic(fmt.Errorf("init request logger: %w", err))
	}

	// 3. Configure Retry Logic (for DB/File storage connections)
	retryCfg := retry.DefaultConfig()
	retryCfg.OnRetry = func(err error, attempt int, delay time.Duration) {
		newLogger.Warnf("retry attempt %d failed: %v; next retry in %v", attempt, err, delay)
	}

	// 4. Initialize In-Memory Storage (Primary Storage)
	newStorage := storage.NewMemStorage()

	var (
		pstore  *persist.PersistStorage
		dbStore *db.DBStorage
	)

	// 5. Initialize Persistent Storage (Database or File)
	// Priority: Database > File > None
	if f.DatabaseDSN != "" {
		newLogger.Infoln("attempting DB connection", f.DatabaseDSN)

		// Attempt to connect to DB with retries
		dbResult, connErr := retryCfg.Retry(ctx, func(args ...any) (any, error) {
			driver := args[0].(string)
			dsn := args[1].(string)
			return db.CreateConnection(ctx, driver, dsn)
		}, "postgres", f.DatabaseDSN)

		if connErr != nil {
			panic(fmt.Errorf("DB conn error %v", connErr))
		}
		if dbResult != nil {
			dbStore, _ = dbResult.(*db.DBStorage)
			// If DB is active, disable file flush interval (storeInter = 0 usually means sync write,
			// but logic here implies "don't use file flush loop")
			if dbStore != nil {
				f.StoreInter = 0
			}
		}
	}

	// Fallback to File Storage if DB is not available
	if dbStore == nil {
		persistResult, persistErr := retryCfg.Retry(ctx, func(args ...any) (any, error) {
			path := args[0].(string)
			interval := args[1].(int)
			return persist.NewPersistStorage(path, interval)
		}, f.FilePath, f.StoreInter)

		if persistErr != nil {
			panic(fmt.Errorf("init persist storage: %w", persistErr))
		}

		pstore = persistResult.(*persist.PersistStorage)
	}

	// 6. Initialize Business Logic Service
	var newService *service.Service
	if dbStore != nil {
		newService = service.NewService(newStorage, dbStore)
	} else {
		newService = service.NewService(newStorage, pstore)
	}

	// 7. Setup HTTP Router & Middleware
	newMux := chi.NewMux()

	newMux.Use(signature.DecryptRSAHandler(f.CryptoKey))
	newMux.Use(newLogger.WithLogging)       // Logging middleware
	newMux.Use(myCompress.GzipHandleWriter) // Response compression

	if f.Key != "" && f.Key != "none" {
		newMux.Use(signature.SignatureHandler(f.Key)) // HMAC Signature verification
	}

	newMux.Use(myCompress.GzipHandleReader) // Request decompression

	if f.TrustedSubnet != "" {
		newMux.Use(trustedsubnet.TrustedSubnetMiddleware(f.TrustedSubnet))
	}

	newMux.Mount("/swagger", httpSwagger.WrapHandler)

	// Mount profiler for debugging
	newMux.Mount("/debug", middleware.Profiler())

	// 8. Initialize Handlers
	newHandler := handlers.NewHandlerService(newService, newMux)

	// 9. Restore Metrics from persistent storage if enabled
	if f.Restore {
		if err := newService.PersistRestore(ctx); err != nil {
			newLogger.Warnln("restore persisted metrics: ", err)
		}
	}

	// 10. Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// 11. Start Server and Background Tasks
	if f.StoreInter > 0 {
		// Asynchronous flushing mode
		var wg sync.WaitGroup

		// Создаём контекст для управления flush loop
		flushCtx, flushCancel := context.WithCancel(ctx)

		// Task A: Periodic Flush Loop
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := newService.LoopFlushWithContext(flushCtx); err != nil {
				// Игнорируем ошибку отмены контекста
				if !errors.Is(err, context.Canceled) {
					newLogger.Errorln("flush loop error:", err)
				}
			}
		}()

		// Task B: HTTP Server
		newHandler.CreateHandlers()
		r := newHandler.GetRouter()

		server := &http.Server{
			Addr:    f.GetAddr(),
			Handler: r,
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			newLogger.Infoln("Starting server on", f.GetAddr())
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				newLogger.Errorln("listen and serve error:", err)
			}
		}()

		// Ожидаем сигнал завершения
		sig := <-sigChan
		newLogger.Infof("Received signal %v, initiating graceful shutdown...", sig)

		// Останавливаем flush loop
		flushCancel()

		// Graceful shutdown HTTP сервера
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			newLogger.Errorln("server shutdown error:", err)
		}

		// Ждём завершения всех горутин
		wg.Wait()

		// Финальный flush данных перед закрытием
		newLogger.Infoln("Flushing remaining data...")
		if err := newService.PersistFlush(context.Background()); err != nil {
			newLogger.Errorln("final flush error:", err)
		}

		// Закрываем хранилище
		newService.StorageCloser()
		newLogger.Sync()
		newLogger.Infoln("Server stopped gracefully")

	} else if f.StoreInter == 0 {
		// Synchronous mode (or DB mode)
		newHandler.CreateHandlers()
		r := newHandler.GetRouter()

		server := &http.Server{
			Addr:    f.GetAddr(),
			Handler: r,
		}

		// Запускаем сервер в горутине
		go func() {
			newLogger.Infoln("Starting server on", f.GetAddr())
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				newLogger.Errorln("listen and serve error:", err)
			}
		}()

		// Ожидаем сигнал завершения
		sig := <-sigChan
		newLogger.Infof("Received signal %v, initiating graceful shutdown...", sig)

		// Graceful shutdown HTTP сервера
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			newLogger.Errorln("server shutdown error:", err)
		}

		// Финальный flush данных (для DB режима - сохраняем всё что в памяти)
		newLogger.Infoln("Flushing remaining data...")
		if err := newService.PersistFlush(context.Background()); err != nil {
			newLogger.Errorln("final flush error:", err)
		}

		// Закрываем хранилище
		newService.StorageCloser()
		newLogger.Sync()
		newLogger.Infoln("Server stopped gracefully")

	} else {
		panic(fmt.Errorf("please, set STORE_INTERVAL >= 0"))
	}
}
