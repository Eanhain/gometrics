package main

import (
	"net/http"
)

func main() {
	storageMetrics = newMemStorage()
	http.HandleFunc(`/update/`, updateMetrics)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
