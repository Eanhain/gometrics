package main

import (
	"fmt"
	myCompress "gometrics/internal/compress"
	"gometrics/internal/db"
	"gometrics/internal/handlers"
	"gometrics/internal/logger"
	"gometrics/internal/persist"
	"gometrics/internal/serverconfig"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"log"
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
	log.Printf("attempting DB connection %v", f.DatabaseDSN)
	newDB, err := db.CreateConnection("postgres", f.DatabaseDSN)
	if err != nil {
		log.Fatalf("DB conn error:: %v", err)
	}

	newMux := chi.NewMux()
	newMux.Use(newLogger.WithLogging)
	newMux.Use(myCompress.GzipHandleReader)
	newMux.Use(myCompress.GzipHandleWriter)

	newService := service.NewService(newStorage, pstore, newDB)

	defer newService.StorageCloser()
	defer newService.DBCloser()

	newHandler := handlers.NewHandlerService(newService, newMux)

	// fmt.Println(f.Restore, f.StoreInter)

	if f.Restore {
		if err := newService.PersistRestore(); err != nil {
			panic(fmt.Errorf("restore persisted metrics: %w", err))
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
