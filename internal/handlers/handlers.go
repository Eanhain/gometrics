package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type handlerService struct {
	service serviceInt
	router  *chi.Mux
	logger  loggerServer
}

type serviceInt interface {
	GaugeInsert(key string, value float64) int
	CounterInsert(key string, value int) int
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetAllMetrics() ([]string, []string, map[string]string)
}

type loggerServer interface {
	WithLogging(h http.HandlerFunc) http.HandlerFunc
	Sync() error
}

func NewHandlerService(service serviceInt, logger loggerServer) *handlerService {
	return &handlerService{
		service: service,
		router:  chi.NewRouter(),
		logger:  logger,
	}
}

func (h *handlerService) SyncLogger() error {
	return h.logger.Sync()
}

func (h *handlerService) GetRouter() *chi.Mux {
	return h.router
}

func (h *handlerService) CreateHandlers() {
	h.router.Group(func(r chi.Router) {
		r.Get("/", h.logger.WithLogging(http.HandlerFunc(h.showAllMetrics)))
		r.Post("/update/", h.logger.WithLogging(http.HandlerFunc(h.PostJSON)))
		r.Post("/value/", h.logger.WithLogging(http.HandlerFunc(h.GetJSON)))
		r.Post("/update/{type}/{name}/{value}", h.logger.WithLogging(http.HandlerFunc(h.UpdateMetrics)))
		r.Get("/value/{type}/{name}", h.logger.WithLogging(http.HandlerFunc(h.GetMetrics)))
	})
}

func (h *handlerService) showAllMetrics(res http.ResponseWriter, req *http.Request) {
	keysGauge, keysCounter, metrics := h.service.GetAllMetrics()
	keys := append(keysGauge, keysCounter...)
	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	format := "%s: %s<br>"
	for _, key := range keys {
		_, err := fmt.Fprintf(res, format, key, metrics[key])
		if err != nil {
			http.Error(res, "cannot parse metric", http.StatusBadRequest)
			panic(err)
		}
	}
	res.WriteHeader(http.StatusOK)
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
			http.Error(res, "cannot parse metric", http.StatusBadRequest)
			panic(err)
		}
		res.WriteHeader(http.StatusOK)
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
		res.WriteHeader(http.StatusOK)
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
		value, err := strconv.ParseFloat(valueMetric, 64)
		if err != nil {
			http.Error(res, "could not parse gaude metric", http.StatusBadRequest)
			return
		}
		res.WriteHeader(h.service.GaugeInsert(nameMetric, value))
	case "counter":
		value, err := strconv.Atoi(valueMetric)
		if err != nil {
			http.Error(res, "could not parse counter metric", http.StatusBadRequest)
			return
		}
		res.WriteHeader(h.service.CounterInsert(nameMetric, value))
	default:
		http.Error(res, "invalid action type", http.StatusBadRequest)
		return

	}

}
