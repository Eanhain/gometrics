package handlers

import (
	"bytes"
	"context"
	"fmt"
	metricsdto "gometrics/internal/api/metricsdto"
	"log"
	"net/http"
	"strconv"
	"strings"

	"encoding/gob"

	"github.com/go-chi/chi/v5"
)

type handlerService struct {
	service serviceInt
	router  *chi.Mux
}

type serviceInt interface {
	GaugeInsert(key string, value float64) error
	CounterInsert(key string, value int) error
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetAllMetrics() ([]string, []string, map[string]string)
	Ping(ctx context.Context) error
	FromStructToStoreBatch(metrics []metricsdto.Metrics) error
}

func NewHandlerService(service serviceInt, router *chi.Mux) *handlerService {
	return &handlerService{
		service: service,
		router:  router,
	}
}

func (h *handlerService) GetRouter() *chi.Mux {
	return h.router
}

func (h *handlerService) CreateHandlers() {
	h.router.Group(func(r chi.Router) {
		r.Get("/", h.showAllMetrics)
		r.Get("/value/{type}/{name}", h.GetMetrics)
		r.Get("/ping", h.Ping)
		r.Post("/update/", h.PostJSON)
		r.Post("/updates/", h.PostMetrics)
		r.Post("/value/", h.GetJSON)
		r.Post("/update/{type}/{name}/{value}", h.UpdateMetrics)
	})
}

func (h *handlerService) PostMetrics(res http.ResponseWriter, req *http.Request) {
	if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
		h.PostArrayJSON(res, req)
		return
	} else if strings.Contains(req.Header.Get("Content-Type"), "application/x-gob") {
		h.PostMetricsArray(res, req)
		return
	}
}

func (h *handlerService) PostMetricsArray(res http.ResponseWriter, req *http.Request) {
	var metrics []metricsdto.Metrics
	var returnBuf bytes.Buffer

	res.Header().Set("Content-Type", "application/x-gob")

	decoder := gob.NewDecoder(req.Body)
	err := decoder.Decode(&metrics)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to read request body with gob: %v", err), http.StatusInternalServerError)
		return
	}

	buf := gob.NewEncoder(&returnBuf)
	err = buf.Encode(metrics)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to write request body with gob: %v", err), http.StatusInternalServerError)
		return
	}
	res.Write(returnBuf.Bytes())
	err = h.service.FromStructToStoreBatch(metrics)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to write request body: %v", err), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
}

func (h *handlerService) Ping(res http.ResponseWriter, req *http.Request) {
	err := h.service.Ping(req.Context())
	if err != nil {
		log.Printf("cannot ping db: %v", err)
		http.Error(res, fmt.Sprintf("cannot ping db: %v", err), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
}

func (h *handlerService) showAllMetrics(res http.ResponseWriter, req *http.Request) {
	keysGauge, keysCounter, metrics := h.service.GetAllMetrics()
	keys := append(keysGauge, keysCounter...)
	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	format := "%s: %s<br>"
	for _, key := range keys {
		if _, err := fmt.Fprintf(res, format, key, metrics[key]); err != nil {
			http.Error(res, fmt.Sprintf("cannot render metric: %v", err), http.StatusBadRequest)
			return
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
			http.Error(res, fmt.Sprintf("gauge metric not found: %v", err), http.StatusNotFound)
			return
		}
		if _, err = fmt.Fprintf(res, format, value); err != nil {
			http.Error(res, fmt.Sprintf("cannot render metric: %v", err), http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
	case "counter":
		value, err := h.service.GetCounter(nameMetric)
		if err != nil {
			http.Error(res, fmt.Sprintf("counter metric not found: %v", err), http.StatusNotFound)
			return
		}
		if _, err = fmt.Fprintf(res, format, value); err != nil {
			http.Error(res, fmt.Sprintf("cannot render metric: %v", err), http.StatusBadRequest)
			return
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
			http.Error(res, fmt.Sprintf("could not parse gauge metric: %v", err), http.StatusBadRequest)
			return
		}
		err = h.service.GaugeInsert(nameMetric, value)
		if err != nil {
			http.Error(res, fmt.Sprintf("could not insert gauge metric: %v", err), http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
	case "counter":
		value, err := strconv.Atoi(valueMetric)
		if err != nil {
			http.Error(res, fmt.Sprintf("could not parse counter metric: %v", err), http.StatusBadRequest)
			return
		}
		err = h.service.CounterInsert(nameMetric, value)
		if err != nil {
			http.Error(res, fmt.Sprintf("could not insert counter metric: %v", err), http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
	default:
		http.Error(res, "invalid action type", http.StatusBadRequest)
		return

	}

}
