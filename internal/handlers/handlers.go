// Package handlers implements HTTP handlers for the metrics collection service.
// It uses chi router for routing and supports JSON and plain text formats.
package handlers

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	metricsdto "gometrics/internal/api/metricsdto"

	"github.com/go-chi/chi/v5"
)

// HandlerService manages HTTP request handling and routing.
type HandlerService struct {
	service Service
	router  *chi.Mux
}

// Service defines the business logic interface for metrics manipulation.
type Service interface {
	GaugeInsert(ctx context.Context, key string, value float64) error
	CounterInsert(ctx context.Context, key string, value int) error
	GetGauge(ctx context.Context, key string) (float64, error)
	GetCounter(ctx context.Context, key string) (int, error)
	GetAllMetrics(ctx context.Context) ([]string, []string, map[string]string)
	Ping(ctx context.Context) error
	FromStructToStoreBatch(ctx context.Context, metrics []metricsdto.Metrics) error
}

// NewHandlerService creates a new HandlerService instance.
func NewHandlerService(service Service, router *chi.Mux) *HandlerService {
	return &HandlerService{
		service: service,
		router:  router,
	}
}

// GetRouter returns the underlying chi.Mux router.
func (h *HandlerService) GetRouter() *chi.Mux {
	return h.router
}

// CreateHandlers registers all API routes for the service.
func (h *HandlerService) CreateHandlers() {
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

// PostMetrics handles bulk updates of metrics.
// It supports both JSON array and Gob formats based on Content-Type header.
func (h *HandlerService) PostMetrics(res http.ResponseWriter, req *http.Request) {
	contentType := req.Header.Get("Content-Type")
	log.Printf("PostMetrics Content-Type: %s", contentType)

	if strings.Contains(contentType, "application/json") {
		h.PostArrayJSON(res, req)
		return
	}

	if strings.Contains(contentType, "application/x-gob") {
		h.PostMetricsArray(res, req)
		return
	}

	http.Error(res, "unsupported content type", http.StatusBadRequest)
}

// PostMetricsArray handles batch updates in Gob format.
func (h *HandlerService) PostMetricsArray(res http.ResponseWriter, req *http.Request) {
	var metrics []metricsdto.Metrics

	decoder := gob.NewDecoder(req.Body)
	if err := decoder.Decode(&metrics); err != nil {
		log.Printf("failed to decode gob: %v", err)
		http.Error(res, fmt.Sprintf("failed to decode gob: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.service.FromStructToStoreBatch(req.Context(), metrics); err != nil {
		log.Printf("failed to store metrics: %v", err)
		http.Error(res, fmt.Sprintf("failed to store metrics: %v", err), http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/x-gob")

	var returnBuf bytes.Buffer
	if err := gob.NewEncoder(&returnBuf).Encode(metrics); err != nil {
		log.Printf("failed to encode response: %v", err)
		http.Error(res, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
	res.Write(returnBuf.Bytes())
}

// Ping checks the database connection status.
func (h *HandlerService) Ping(res http.ResponseWriter, req *http.Request) {
	err := h.service.Ping(req.Context())
	if err != nil {
		log.Printf("cannot ping db: %v", err)
		http.Error(res, fmt.Sprintf("cannot ping db: %v", err), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
}

// showAllMetrics renders an HTML page with all stored metrics.
func (h *HandlerService) showAllMetrics(res http.ResponseWriter, req *http.Request) {
	keysGauge, keysCounter, metrics := h.service.GetAllMetrics(req.Context())
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

// GetMetrics retrieves a specific metric value via URL path parameters.
func (h *HandlerService) GetMetrics(res http.ResponseWriter, req *http.Request) {
	typeMetric := chi.URLParam(req, "type")
	nameMetric := chi.URLParam(req, "name")
	format := "%v"
	switch typeMetric {
	case metricsdto.MetricTypeGauge:
		value, err := h.service.GetGauge(req.Context(), nameMetric)
		if err != nil {
			http.Error(res, fmt.Sprintf("gauge metric not found: %v", err), http.StatusNotFound)
			return
		}
		if _, err = fmt.Fprintf(res, format, value); err != nil {
			http.Error(res, fmt.Sprintf("cannot render metric: %v", err), http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
	case metricsdto.MetricTypeCounter:
		value, err := h.service.GetCounter(req.Context(), nameMetric)
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

// UpdateMetrics updates a single metric via URL path parameters.
func (h *HandlerService) UpdateMetrics(res http.ResponseWriter, req *http.Request) {
	typeMetric := chi.URLParam(req, "type")
	nameMetric := chi.URLParam(req, "name")
	valueMetric := chi.URLParam(req, "value")
	switch typeMetric {
	case metricsdto.MetricTypeGauge:
		value, err := strconv.ParseFloat(valueMetric, 64)
		if err != nil {
			http.Error(res, fmt.Sprintf("could not parse gauge metric: %v", err), http.StatusBadRequest)
			return
		}
		err = h.service.GaugeInsert(req.Context(), nameMetric, value)
		if err != nil {
			http.Error(res, fmt.Sprintf("could not insert gauge metric: %v", err), http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
	case metricsdto.MetricTypeCounter:
		value, err := strconv.Atoi(valueMetric)
		if err != nil {
			http.Error(res, fmt.Sprintf("could not parse counter metric: %v", err), http.StatusBadRequest)
			return
		}
		err = h.service.CounterInsert(req.Context(), nameMetric, value)
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
