package main

import (
	"context"
	"fmt"
	myCompress "gometrics/internal/compress"
	"gometrics/internal/db"
	"gometrics/internal/handlers"
	"gometrics/internal/logger"
	"gometrics/internal/persist"
	"gometrics/internal/serverconfig"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

func main() {
	f := serverconfig.InitialFlags()
	f.ParseFlags()

	pstore, err := persist.NewPersistStorage(f.FilePath, f.StoreInter)

	if err != nil {
		panic(fmt.Errorf("init persist storage: %w", err))
	}

	newStorage := storage.NewMemStorage()
	newLogger, err := logger.CreateLoggerRequest()
	if err != nil {
		panic(fmt.Errorf("init request logger: %w", err))
	}
	newLogger.Infoln("attempting DB connection", f.DatabaseDSN)
	newDB, err := db.CreateConnection(context.Background(), "postgres", f.DatabaseDSN)

	var newService *service.Service

	if f.DatabaseDSN == "" {
		newLogger.Errorf("DB conn error, DatabaseDSN is empty, return to file storage %v", err)
		newService = service.NewService(newStorage, pstore)
	} else if err != nil {
		panic(fmt.Errorf("cannot connect to postgres %v", err))
	} else {
		newService = service.NewService(newStorage, newDB)
		f.StoreInter = 0
	}

	newMux := chi.NewMux()
	newMux.Use(newLogger.WithLogging)
	newMux.Use(myCompress.GzipHandleReader)
	newMux.Use(myCompress.GzipHandleWriter)

	defer newService.StorageCloser()

	newHandler := handlers.NewHandlerService(newService, newMux)

	// fmt.Println(f.Restore, f.StoreInter)

	if f.Restore {
		if err := newService.PersistRestore(); err != nil {
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

			err = http.ListenAndServe(f.GetAddr(), r)
			if err != nil {
				panic(fmt.Errorf("listen and serve on %s: %w", f.GetAddr(), err))
			}
		}()
		wg.Wait()

	} else if f.StoreInter == 0 {
		newHandler.CreateHandlers()
		r := newHandler.GetRouter()

		err = http.ListenAndServe(f.GetAddr(), r)
		if err != nil {
			panic(fmt.Errorf("listen and serve on %s: %w", f.GetAddr(), err))
		}
	} else {
		panic(fmt.Errorf("please, set STORE_INTERVAL >= 0"))
	}

}
