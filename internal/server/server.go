package server

import (
	"net/http"
)

type server struct {
	port    string
	handler http.Handler
}

// type repositories interface {
// 	gaugeInsert(key string, value string) error
// 	counterInsert(key string, value string) error
// }

// type handlerService interface {
// 	newHandlerService(storage repositories) *handlerService
// 	updateMetrics(res http.ResponseWriter, req http.Request) error
// }

func createServer(port string, handler http.Handler) *server {
	return &server{
		port:    port,
		handler: handler,
	}
}

func (h *server) initalServer() error {
	return http.ListenAndServe(h.port, h.handler)
}
