// Package main is the entry point for the metrics collection server.
// It initializes storage, configuration, logging, and starts the HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	_ "gometrics/swagger"
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
func main() {
	fmt.Println(configs.BuildVerPrint())

	// 1. Initialize configuration
	f := serverconfig.InitialFlags()
	f.ParseFlags()

	// 2. Initialize Logger
	newLogger, err := logger.CreateLoggerRequest()
	if err != nil {
		panic(fmt.Errorf("init request logger: %w", err))
	}
	defer newLogger.Sync()

	// 3. Context for Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Configure Retry Logic (for DB/File storage connections)
	retryCfg := retry.DefaultConfig()
	retryCfg.OnRetry = func(err error, attempt int, delay time.Duration) {
		newLogger.Warnf("retry attempt %d failed: %v; next retry in %v", attempt, err, delay)
	}

	// 5. Initialize In-Memory Storage (Primary Storage)
	newStorage := storage.NewMemStorage()

	var (
		pstore  *persist.PersistStorage
		dbStore *db.DBStorage
	)

	// 6. Initialize Persistent Storage (Database or File)
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
			// If DB is active, disable file flush interval
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

	// 7. Initialize Business Logic Service
	var newService *service.Service
	if dbStore != nil {
		newService = service.NewService(newStorage, dbStore)
	} else {
		newService = service.NewService(newStorage, pstore)
	}

	// 8. Setup HTTP Router & Middleware
	newMux := chi.NewMux()

	newMux.Use(signature.DecryptRSAHandler(f.CryptoKey))
	newMux.Use(newLogger.WithLogging)       // Logging middleware
	newMux.Use(myCompress.GzipHandleWriter) // Response compression

	if f.Key != "" && f.Key != "none" {
		newMux.Use(signature.SignatureHandler(f.Key)) // HMAC Signature verification
	}

	newMux.Use(myCompress.GzipHandleReader) // Request decompression

	newMux.Mount("/swagger", httpSwagger.WrapHandler)

	// Mount profiler for debugging
	newMux.Mount("/debug", middleware.Profiler())

	// 9. Initialize Handlers
	newHandler := handlers.NewHandlerService(newService, newMux)
	newHandler.CreateHandlers()
	r := newHandler.GetRouter()

	// 10. Restore Metrics from persistent storage if enabled
	if f.Restore {
		if err := newService.PersistRestore(ctx); err != nil {
			newLogger.Warnln("restore persisted metrics: ", err)
		}
	}

	// 11. Validate StoreInter configuration
	if f.StoreInter < 0 {
		panic(fmt.Errorf("STORE_INTERVAL must be >= 0, got %d", f.StoreInter))
	}

	// 12. Start Server and Background Tasks
	srv := &http.Server{
		Addr:    f.GetAddr(),
		Handler: r,
	}

	var wg sync.WaitGroup

	// --- Task A: Periodic Flush Loop (File Storage Only) ---
	if f.StoreInter > 0 && dbStore == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newLogger.Infof("Starting background flush loop with interval %d sec", f.StoreInter)

			ticker := time.NewTicker(time.Duration(f.StoreInter) * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					newLogger.Info("Flush loop stopped by context cancellation")
					return
				case <-ticker.C:
					if err := newService.PersistFlush(ctx); err != nil {
						newLogger.Errorf("Periodic flush error: %v", err)
					} else {
						newLogger.Debug("Metrics flushed to disk")
					}
				}
			}
		}()
	}

	// --- Task B: HTTP Server ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		newLogger.Infoln("Starting HTTP server on", f.GetAddr())

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			newLogger.Errorf("HTTP server error: %v", err)
			cancel() // Trigger emergency shutdown
		}
	}()

	// --- Task C: Wait for Shutdown Signal ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-quit:
		newLogger.Infof("Received signal %v, initiating graceful shutdown...", sig)
	case <-ctx.Done():
		newLogger.Info("Context cancelled, shutting down...")
	}

	// --- Shutdown Sequence ---

	// Step 1: Stop accepting new HTTP connections
	newLogger.Info("Shutting down HTTP server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		newLogger.Errorf("HTTP server shutdown error: %v", err)
	} else {
		newLogger.Info("HTTP server stopped gracefully")
	}

	// Step 2: Cancel context to stop background tasks (flush loop)
	cancel()

	// Step 3: Wait for all goroutines to finish
	newLogger.Info("Waiting for background tasks to complete...")
	wg.Wait()

	// Step 4: Final flush to ensure no data loss
	newLogger.Info("Performing final flush to persistent storage...")
	if f.StoreInter > 0 && dbStore == nil {
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer flushCancel()

		if err := newService.PersistFlush(flushCtx); err != nil {
			newLogger.Errorf("Final flush error: %v", err)
		} else {
			newLogger.Info("Final flush completed successfully")
		}
	}

	// Step 5: Close storage connections
	newLogger.Info("Closing storage connections...")
	if err := newService.StorageCloser(); err != nil {
		newLogger.Errorf("Storage close error: %v", err)
	}

	newLogger.Info("Server exited successfully")
}
