package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type handlerService struct {
	storage repositories
	router  *chi.Mux
}

type repositories interface {
	GaugeInsert(key string, value string) int
	CounterInsert(key string, value string) int
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetAllMetrics() map[string]string
}

func NewHandlerService(storage repositories) *handlerService {
	return &handlerService{
		storage: storage,
		router:  chi.NewRouter(),
	}
}

func (h *handlerService) GetRouter() *chi.Mux {
	return h.router
}

func (h *handlerService) CreateHandlers() {
	h.router.Group(func(r chi.Router) {
		r.Get("/", h.showAllMetrics)
		r.Post("/update/{type}/{name}/{value}", h.UpdateMetrics)
		r.Get("/value/{type}/{name}", h.GetMetrics)
	})
}

func (h *handlerService) showAllMetrics(res http.ResponseWriter, req *http.Request) {
	metrics := h.storage.GetAllMetrics()
	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	format := "%s: %s<br>"
	for key, value := range metrics {
		fmt.Fprintf(res, format, key, value)
	}
}

func (h *handlerService) GetMetrics(res http.ResponseWriter, req *http.Request) {
	typeMetric := chi.URLParam(req, "type")
	nameMetric := chi.URLParam(req, "name")
	format := "%v"
	switch typeMetric {
	case "gauge":
		value, err := h.storage.GetGauge(nameMetric)
		if err != nil {
			http.Error(res, "gauge metric not found", http.StatusNotFound)
			return
		}
		fmt.Fprintf(res, format, value)
	case "counter":
		value, err := h.storage.GetCounter(nameMetric)
		if err != nil {
			http.Error(res, "counter metric not found", http.StatusNotFound)
			return
		}
		fmt.Fprintf(res, format, value)
	default:
		http.Error(res, "invalid metric type", http.StatusBadRequest)
		return
	}
}

func (h *handlerService) UpdateMetrics(res http.ResponseWriter, req *http.Request) {
	typeMetric := chi.URLParam(req, "type")
	nameMetric := chi.URLParam(req, "name")
	valueMetric := chi.URLParam(req, "value")
	switch typeMetric {
	case "gauge":
		res.WriteHeader(h.storage.GaugeInsert(nameMetric, valueMetric))
	case "counter":
		res.WriteHeader(h.storage.CounterInsert(nameMetric, valueMetric))
	default:
		http.Error(res, "invalid action type", http.StatusBadRequest)
		return

	}

}
