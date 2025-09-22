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

	"github.com/go-chi/chi/v5"
)

func main() {
	f := serverconfig.InitialFlags()
	f.ParseFlags()
	file := persist.NewPersistStorage(f.FilePath, f.Restore, f.StoreInter)
	fmt.Println(file)
	newStorage := storage.NewMemStorage()
	newLogger := logger.CreateLoggerRequest()
	newMux := chi.NewMux()
	newMux.Use(newLogger.WithLogging)
	newMux.Use(myCompress.GzipHandleReader)
	newMux.Use(myCompress.GzipHandleWriter)
	newHandler := handlers.NewHandlerService(service.NewService(newStorage), newMux)
	defer newLogger.Sync()
	newHandler.CreateHandlers()
	r := newHandler.GetRouter()
	print(f.GetAddr())
	err := http.ListenAndServe(f.GetAddr(), r)
	if err != nil {
		panic(err)
	}
}
