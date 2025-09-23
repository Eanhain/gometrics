package main

import (
	"fmt"
	myCompress "gometrics/internal/compress"
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
		panic(err)
	}

	newStorage := storage.NewMemStorage()
	newLogger := logger.CreateLoggerRequest()

	newMux := chi.NewMux()
	newMux.Use(newLogger.WithLogging)
	newMux.Use(myCompress.GzipHandleReader)
	newMux.Use(myCompress.GzipHandleWriter)

	newService := service.NewService(newStorage, pstore)

	defer newService.StorageCloser()

	newHandler := handlers.NewHandlerService(newService, newMux)

	if f.Restore {
		err := newService.PersistRestore()
		if err != nil {
			panic(err)
		}
	}

	if f.StoreInter > 0 {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			newService.LoopFlush()
		}()

		go func() {
			defer wg.Done()
			defer newLogger.Sync()

			newHandler.CreateHandlers()
			r := newHandler.GetRouter()

			err = http.ListenAndServe(f.GetAddr(), r)
			if err != nil {
				panic(err)
			}
		}()
		wg.Wait()

	} else if f.StoreInter == 0 {
		newHandler.CreateHandlers()
		r := newHandler.GetRouter()

		err = http.ListenAndServe(f.GetAddr(), r)
		if err != nil {
			panic(err)
		}
	} else {
		panic(fmt.Errorf("please, set STORE_INTERVAL >= 0"))
	}

}
