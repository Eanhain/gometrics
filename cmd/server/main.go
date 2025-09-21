package main

import (
	myCompress "gometrics/internal/compress"
	"gometrics/internal/handlers"
	"gometrics/internal/logger"
	"gometrics/internal/serverconfig"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {
	newStorage := storage.NewMemStorage()
	newLogger := logger.CreateLoggerRequest()
	newMux := chi.NewMux()
	newMux.Use(newLogger.WithLogging)
	newMux.Use(myCompress.GzipHandleReader)
	newMux.Use(myCompress.GzipHandleWriter)
	newHandler := handlers.NewHandlerService(service.NewService(newStorage), newMux)
	defer newLogger.Sync()
	f := serverconfig.InitialFlags()
	f.ParseFlags()
	newHandler.CreateHandlers()
	r := newHandler.GetRouter()
	print(f.GetAddr())
	err := http.ListenAndServe(f.GetAddr(), r)
	if err != nil {
		panic(err)
	}
}
