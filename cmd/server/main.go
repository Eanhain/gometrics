package main

import (
	"gometrics/internal/handlers"
	"gometrics/internal/serverflags"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"net/http"
)

func main() {
	newHandler := handlers.NewHandlerService(service.NewService(storage.NewMemStorage()))
	f := serverflags.InitialFlags()
	f.ParseFlags()

	newHandler.CreateHandlers()
	r := newHandler.GetRouter()
	print(f.GetAddr())
	err := http.ListenAndServe(f.GetAddr(), r)
	if err != nil {
		panic(err)
	}
}
