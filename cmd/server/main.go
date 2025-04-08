package main

import (
	"gometrics/internal/confserver"
	"gometrics/internal/handlers"
	"gometrics/internal/storage"
	"net/http"
)

func main() {
	newStorage := storage.NewMemStorage()
	newHandler := handlers.NewHandlerService(newStorage)
	print("Work test")
	f := confserver.InitialFlags()
	f.ParseFlags()

	newHandler.CreateHandlers()
	r := newHandler.GetRouter()
	err := http.ListenAndServe(f.Addr.GetAddr(), r)
	if err != nil {
		panic(err)
	}
}
