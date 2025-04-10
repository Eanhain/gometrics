package main

import (
	"gometrics/internal/handlers"
	"gometrics/internal/serverconfig"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"net/http"
)

func main() {
	newStorage := storage.NewMemStorage()
	newHandler := handlers.NewHandlerService(service.NewService(newStorage))
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
