package main

import (
	myCompress "gometrics/internal/compress"
	"gometrics/internal/handlers"
	"gometrics/internal/logger"
	"gometrics/internal/persist"
	"gometrics/internal/serverconfig"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {
	f := serverconfig.InitialFlags()
	f.ParseFlags()

	pstore := persist.NewPersistStorage(f.FilePath)

	newStorage := storage.NewMemStorage()
	newLogger := logger.CreateLoggerRequest()

	newMux := chi.NewMux()
	newMux.Use(newLogger.WithLogging)
	newMux.Use(myCompress.GzipHandleReader)
	newMux.Use(myCompress.GzipHandleWriter)

	newService := service.NewService(newStorage, pstore)

	newHandler := handlers.NewHandlerService(newService, newMux)

	if f.Restore {
		newService.PersistRestore()
	}

	defer newLogger.Sync()
	newHandler.CreateHandlers()
	r := newHandler.GetRouter()
	print(f.GetAddr())
	err := http.ListenAndServe(f.GetAddr(), r)
	if err != nil {
		panic(err)
	}
}
