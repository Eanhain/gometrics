// Package main is the entry point for the metrics collection server.
package main

import (
	"context"
	"errors"
	"fmt"
	_ "gometrics/swagger"
	"net/http"
	_ "net/http/pprof"
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

func main() {
	fmt.Println(configs.BuildVerPrint())

	f := serverconfig.InitialFlags()
	f.ParseFlags()

	newLogger, err := logger.CreateLoggerRequest()
	if err != nil {
		panic(fmt.Errorf("init request logger: %w", err))
	}
	defer newLogger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retryCfg := retry.DefaultConfig()
	retryCfg.OnRetry = func(err error, attempt int, delay time.Duration) {
		newLogger.Warnf("retry attempt %d failed: %v; next retry in %v", attempt, err, delay)
	}

	// 4. Storage Init
	newStorage := storage.NewMemStorage()
	var (
		pstore  *persist.PersistStorage
		dbStore *db.DBStorage
	)

	// DB Connection
	if f.DatabaseDSN != "" {
		newLogger.Infoln("attempting DB connection", f.DatabaseDSN)
		dbResult, connErr := retryCfg.Retry(ctx, func(args ...any) (any, error) {
			return db.CreateConnection(ctx, args[0].(string), args[1].(string))
		}, "postgres", f.DatabaseDSN)

		if connErr != nil {
			panic(fmt.Errorf("DB conn error %v", connErr))
		}
		if dbResult != nil {
			dbStore, _ = dbResult.(*db.DBStorage)
			if dbStore != nil {
				f.StoreInter = 0
			}
		}
	}

	// File Storage
	if dbStore == nil {
		persistResult, persistErr := retryCfg.Retry(ctx, func(args ...any) (any, error) {
			return persist.NewPersistStorage(args[0].(string), args[1].(int))
		}, f.FilePath, f.StoreInter)

		if persistErr != nil {
			panic(fmt.Errorf("init persist storage: %w", persistErr))
		}
		pstore = persistResult.(*persist.PersistStorage)
	}

	// 5. Service
	var newService *service.Service
	if dbStore != nil {
		newService = service.NewService(newStorage, dbStore)
	} else {
		newService = service.NewService(newStorage, pstore)
	}

	// 6. Router & Middleware
	newMux := chi.NewMux()

	// --- ИСПРАВЛЕННЫЙ ПОРЯДОК MIDDLEWARE ---

	// 1. Сначала расшифровка (если есть RSA)
	newMux.Use(signature.DecryptRSAHandler(f.CryptoKey))

	// 2. Логирование (видит сырой запрос, это ок)
	newMux.Use(newLogger.WithLogging)

	// 3. ВАЖНО: Сначала разжимаем запрос (Gzip Reader), чтобы дальше работать с JSON
	newMux.Use(myCompress.GzipHandleReader)

	// 4. Подготавливаем Writer для сжатия ответа
	newMux.Use(myCompress.GzipHandleWriter)

	// 5. Проверяем подпись (теперь она проверяет чистый JSON, а не GZIP)
	if f.Key != "" && f.Key != "none" {
		newMux.Use(signature.SignatureHandler(f.Key))
	}

	// --- КОНЕЦ ИСПРАВЛЕНИЙ ---

	newMux.Mount("/swagger", httpSwagger.WrapHandler)
	newMux.Mount("/debug", middleware.Profiler())

	newHandler := handlers.NewHandlerService(newService, newMux)
	newHandler.CreateHandlers()
	r := newHandler.GetRouter()

	// 7. Restore
	if f.Restore {
		if err := newService.PersistRestore(ctx); err != nil {
			newLogger.Warnln("restore persisted metrics: ", err)
		}
	}

	// 8. Server Start
	if f.StoreInter < 0 {
		panic(fmt.Errorf("STORE_INTERVAL must be >= 0"))
	}

	srv := &http.Server{
		Addr:    f.GetAddr(),
		Handler: r,
	}

	var wg sync.WaitGroup

	// Task A: Flush Loop
	if f.StoreInter > 0 && dbStore == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newLogger.Infof("Starting background flush loop (%d sec)", f.StoreInter)
			ticker := time.NewTicker(time.Duration(f.StoreInter) * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := newService.PersistFlush(ctx); err != nil {
						newLogger.Errorf("Flush error: %v", err)
					}
				}
			}
		}()
	}

	// Task B: HTTP Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		newLogger.Infoln("Starting server on", f.GetAddr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			newLogger.Errorf("HTTP server error: %v", err)
			cancel()
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-quit:
		newLogger.Infof("Received signal %v", sig)
	case <-ctx.Done():
		newLogger.Info("Context cancelled")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		newLogger.Errorf("Shutdown error: %v", err)
	}

	cancel()
	wg.Wait()

	if f.StoreInter > 0 && dbStore == nil {
		newLogger.Info("Final flush...")
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer flushCancel()
		if err := newService.PersistFlush(flushCtx); err != nil {
			newLogger.Errorf("Final flush error: %v", err)
		}
	}

	if err := newService.StorageCloser(); err != nil {
		newLogger.Errorf("Storage close error: %v", err)
	}
	newLogger.Info("Server exited.")
}
