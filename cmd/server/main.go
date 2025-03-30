package main

import (
	"gometrics/internal/handlers"
	"gometrics/internal/storage"
	"net/http"
)

func main() {
	newStorage := storage.NewMemStorage()
	newHandler := handlers.NewHandlerService(newStorage)

	newHandler.CreateHandlers()
	r := newHandler.GetRouter()

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		panic(err)
	}
}
