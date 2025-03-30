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

func (h *handlerService) routeUpdateFunc(r chi.Router) {
	r.Post("/{type}/{name}/{value}", h.UpdateMetrics)
}

func (h *handlerService) routeValueFunc(r chi.Router) {
	r.Get("/{type}/{name}", h.GetMetrics)
}

func (h *handlerService) routeRootFunc(r chi.Router) {
	r.Get("/", h.showAllMetrics)
}

func (h *handlerService) CreateHandlers() {
	h.router.Route("/", h.routeRootFunc)
	h.router.Route("/update/", h.routeUpdateFunc)
	h.router.Route("/value/", h.routeValueFunc)
}

func (h *handlerService) showAllMetrics(res http.ResponseWriter, req *http.Request) {
	metrics := h.storage.GetAllMetrics()
	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	// fmt.Fprintf(res, "<ul>")
	format := "%s: %s<br>"
	for key, value := range metrics {
		fmt.Fprintf(res, format, key, value)
	}
	// fmt.Fprintf(res, "</ul>")
}

func (h *handlerService) GetMetrics(res http.ResponseWriter, req *http.Request) {
	typeMetric := chi.URLParam(req, "type")
	nameMetric := chi.URLParam(req, "name")
	format := "%v"
	switch typeMetric {
	case "gauge":
		value, err := h.storage.GetGauge(nameMetric)
		if err != nil {
			res.WriteHeader(http.StatusNotFound)
		} else {
			fmt.Fprintf(res, format, value)
		}
	case "counter":
		value, err := h.storage.GetCounter(nameMetric)
		if err != nil {
			res.WriteHeader(http.StatusNotFound)
		} else {
			fmt.Fprintf(res, format, value)
		}
	default:
		res.WriteHeader(http.StatusBadRequest)
	}
}

func (h *handlerService) UpdateMetrics(res http.ResponseWriter, req *http.Request) {
	typeMetric := chi.URLParam(req, "type")
	nameMetric := chi.URLParam(req, "name")
	valueMetric := chi.URLParam(req, "value")
	// fmt.Printf("Инсертим %s: %s = %s \n", typeMetric, nameMetric, valueMetric)
	switch typeMetric {
	case "gauge":
		res.WriteHeader(h.storage.GaugeInsert(nameMetric, valueMetric))
	case "counter":
		res.WriteHeader(h.storage.CounterInsert(nameMetric, valueMetric))
	default:
		res.WriteHeader(http.StatusBadRequest)

	}

}
