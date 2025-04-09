package main

import (
	"gometrics/internal/handlers"
	"gometrics/internal/serverflags"
	"gometrics/internal/storage"
	"net/http"
)

func main() {
	newStorage := storage.NewMemStorage()
	newHandler := handlers.NewHandlerService(newStorage)
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
