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
		panic(fmt.Errorf("init logger: %w", err))
	}
	defer newLogger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retryCfg := retry.DefaultConfig()
	retryCfg.OnRetry = func(err error, attempt int, delay time.Duration) {
		newLogger.Warnf("retry attempt %d failed: %v", attempt, err)
	}

	newStorage := storage.NewMemStorage()
	var (
		pstore  *persist.PersistStorage
		dbStore *db.DBStorage
	)

	if f.DatabaseDSN != "" {
		newLogger.Infoln("attempting DB connection", f.DatabaseDSN)
		dbResult, connErr := retryCfg.Retry(ctx, func(args ...any) (any, error) {
			return db.CreateConnection(ctx, args[0].(string), args[1].(string))
		}, "postgres", f.DatabaseDSN)

		if connErr != nil {
			panic(fmt.Errorf("DB error %v", connErr))
		}
		if dbResult != nil {
			dbStore, _ = dbResult.(*db.DBStorage)
			if dbStore != nil {
				f.StoreInter = 0
			}
		}
	}

	if dbStore == nil {
		persistResult, persistErr := retryCfg.Retry(ctx, func(args ...any) (any, error) {
			return persist.NewPersistStorage(args[0].(string), args[1].(int))
		}, f.FilePath, f.StoreInter)

		if persistErr != nil {
			panic(fmt.Errorf("init persist: %w", persistErr))
		}
		pstore = persistResult.(*persist.PersistStorage)
	}

	var newService *service.Service
	if dbStore != nil {
		newService = service.NewService(newStorage, dbStore)
	} else {
		newService = service.NewService(newStorage, pstore)
	}

	// ROUTER & MIDDLEWARE
	newMux := chi.NewMux()

	// 1. Decrypt
	newMux.Use(signature.DecryptRSAHandler(f.CryptoKey))
	// 2. Log
	newMux.Use(newLogger.WithLogging)
	// 3. Decompress Request (ВАЖНО: ДО проверки подписи)
	newMux.Use(myCompress.GzipHandleReader)
	// 4. Compress Response
	newMux.Use(myCompress.GzipHandleWriter)
	// 5. Verify Signature (работает с уже разжатым JSON)
	if f.Key != "" && f.Key != "none" {
		newMux.Use(signature.SignatureHandler(f.Key))
	}

	newMux.Mount("/swagger", httpSwagger.WrapHandler)
	newMux.Mount("/debug", middleware.Profiler())

	newHandler := handlers.NewHandlerService(newService, newMux)
	newHandler.CreateHandlers()
	r := newHandler.GetRouter()

	if f.Restore {
		if err := newService.PersistRestore(ctx); err != nil {
			newLogger.Warnln("restore error: ", err)
		}
	}

	if f.StoreInter < 0 {
		panic(fmt.Errorf("STORE_INTERVAL < 0"))
	}

	srv := &http.Server{Addr: f.GetAddr(), Handler: r}
	var wg sync.WaitGroup

	if f.StoreInter > 0 && dbStore == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(time.Duration(f.StoreInter) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = newService.PersistFlush(ctx)
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		newLogger.Infoln("Starting server on", f.GetAddr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			newLogger.Errorf("HTTP error: %v", err)
			cancel()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case <-quit:
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)

	cancel()
	wg.Wait()

	if f.StoreInter > 0 && dbStore == nil {
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer flushCancel()
		_ = newService.PersistFlush(flushCtx)
	}

	_ = newService.StorageCloser()
	newLogger.Info("Server exited.")
}
