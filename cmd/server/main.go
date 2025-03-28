package main

import (
	"gometrics/internal/handlers"
	"gometrics/internal/server"
	"gometrics/internal/storage"
)

func main() {
	newStorage := storage.NewMemStorage()
	newHandler := handlers.NewHandlerService(newStorage)
	newHandler.CreateHandler("/update/")
	server := server.CreateServer(":8080", nil)
	err := server.InitalServer()
	if err != nil {
		panic(err)
	}
}
