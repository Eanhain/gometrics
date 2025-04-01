package main

import (
	"gometrics/internal/flags"
	"gometrics/internal/handlers"
	"gometrics/internal/storage"
	"net/http"
)

var server = true

type flagCustom interface {
	InitialFlags()
	ParseFlags(server bool)
}

func main() {
	newStorage := storage.NewMemStorage()
	newHandler := handlers.NewHandlerService(newStorage)
	f := flags.InitialFlags()
	f.ParseFlags(server)

	newHandler.CreateHandlers()
	r := newHandler.GetRouter()

	err := http.ListenAndServe(f.GetAddr().String(), r)
	if err != nil {
		panic(err)
	}
}
