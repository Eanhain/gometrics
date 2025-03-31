package main

import (
	"flag"
	"gometrics/internal/handlers"
	"gometrics/internal/storage"
	"net/http"
)

var addr string

func main() {
	newStorage := storage.NewMemStorage()
	newHandler := handlers.NewHandlerService(newStorage)

	flag.StringVar(&addr, "a", ":8080", "Net address host:port")

	flag.Parse()

	newHandler.CreateHandlers()
	r := newHandler.GetRouter()

	err := http.ListenAndServe(addr, r)
	if err != nil {
		panic(err)
	}
}
