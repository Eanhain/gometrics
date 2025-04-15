package main

import (
	"gometrics/internal/handlers"
	"gometrics/internal/logger"
	"gometrics/internal/serverconfig"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"net/http"
)

func main() {
	newStorage := storage.NewMemStorage()
	newLogger := logger.CreateLoggerRequest()
	newHandler := handlers.NewHandlerService(service.NewService(newStorage), newLogger)
	defer newHandler.SyncLogger()
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
