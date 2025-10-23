package main

import (
	"context"
	"fmt"
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
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	f := serverconfig.InitialFlags()
	f.ParseFlags()

	ctx := context.Background()

	newLogger, err := logger.CreateLoggerRequest()
	if err != nil {
		panic(fmt.Errorf("init request logger: %w", err))
	}

	retryCfg := retry.DefaultConfig()
	retryCfg.OnRetry = func(err error, attempt int, delay time.Duration) {
		newLogger.Warnf("retry attempt %d failed: %v; next retry in %v", attempt, err, delay)
	}

	newStorage := storage.NewMemStorage()

	var (
		pstore  *persist.PersistStorage
		dbStore *db.DBStorage
	)

	if f.DatabaseDSN != "" {
		newLogger.Infoln("attempting DB connection", f.DatabaseDSN)
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
			if dbStore != nil {
				f.StoreInter = 0
			}
		}
	}

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

	var newService *service.Service
	if dbStore != nil {
		newService = service.NewService(newStorage, dbStore)
	} else {
		newService = service.NewService(newStorage, pstore)
	}

	newMux := chi.NewMux()

	newMux.Use(newLogger.WithLogging)

	if f.Key != "" {
		newMux.Use(signature.SignatureHandler(f.Key))
	}

	newMux.Use(myCompress.GzipHandleWriter)

	newMux.Use(myCompress.GzipHandleReader)

	defer newService.StorageCloser()

	newHandler := handlers.NewHandlerService(newService, newMux)

	if f.Restore {
		if err := newService.PersistRestore(ctx); err != nil {
			newLogger.Warnln("restore persisted metrics: ", err)
		}
	}

	if f.StoreInter > 0 {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := newService.LoopFlush(); err != nil {
				panic(fmt.Errorf("run flush loop: %w", err))
			}
		}()

		go func() {
			defer wg.Done()
			defer newLogger.Sync()

			newHandler.CreateHandlers()
			r := newHandler.GetRouter()

			if err := http.ListenAndServe(f.GetAddr(), r); err != nil {
				panic(fmt.Errorf("listen and serve on %s: %w", f.GetAddr(), err))
			}
		}()
		wg.Wait()

	} else if f.StoreInter == 0 {
		newHandler.CreateHandlers()
		r := newHandler.GetRouter()

		if err := http.ListenAndServe(f.GetAddr(), r); err != nil {
			panic(fmt.Errorf("listen and serve on %s: %w", f.GetAddr(), err))
		}
	} else {
		panic(fmt.Errorf("please, set STORE_INTERVAL >= 0"))
	}
}
