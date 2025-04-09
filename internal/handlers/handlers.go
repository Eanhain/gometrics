package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type handlerService struct {
	service serviceInt
	router  *chi.Mux
}

type serviceInt interface {
	GaugeInsert(key string, value float64) int
	CounterInsert(key string, value int) int
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetAllMetrics() map[string]string
}

func NewHandlerService(service serviceInt) *handlerService {
	return &handlerService{
		service: service,
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
	metrics := h.service.GetAllMetrics()
	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	format := "%s: %s<br>"
	for key, value := range metrics {
		_, err := fmt.Fprintf(res, format, key, value)
		if err != nil {
			panic(err)
		}
	}
}

func (h *handlerService) GetMetrics(res http.ResponseWriter, req *http.Request) {
	typeMetric := chi.URLParam(req, "type")
	nameMetric := chi.URLParam(req, "name")
	format := "%v"
	switch typeMetric {
	case "gauge":
		value, err := h.service.GetGauge(nameMetric)
		if err != nil {
			http.Error(res, "gauge metric not found", http.StatusNotFound)
			return
		}
		_, err = fmt.Fprintf(res, format, value)
		if err != nil {
			panic(err)
		}
	case "counter":
		value, err := h.service.GetCounter(nameMetric)
		if err != nil {
			http.Error(res, "counter metric not found", http.StatusNotFound)
			return
		}
		_, err = fmt.Fprintf(res, format, value)
		if err != nil {
			panic(err)
		}
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
		key := strings.ToLower(nameMetric)
		value, err := strconv.ParseFloat(valueMetric, 64)
		if err != nil {
			http.Error(res, "could not parse gaude metric", http.StatusBadRequest)
			return
		}
		res.WriteHeader(h.service.GaugeInsert(key, value))
	case "counter":
		key := strings.ToLower(nameMetric)
		value, err := strconv.Atoi(valueMetric)
		if err != nil {
			http.Error(res, "could not parse counter metric", http.StatusBadRequest)
			return
		}
		res.WriteHeader(h.service.CounterInsert(key, value))
	default:
		http.Error(res, "invalid action type", http.StatusBadRequest)
		return

	}

}
