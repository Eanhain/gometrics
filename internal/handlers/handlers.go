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
//
// @Summary Update multiple metrics
// @Description Updates metrics in batch. Supports application/json and application/x-gob.
// @Tags update
// @Accept json, application/x-gob
// @Produce json, application/x-gob
// @Param metrics body []metricsdto.Metrics true "List of metrics to update"
// @Success 200 {array} metricsdto.Metrics
// @Failure 500 {string} string "Internal Server Error"
// @Router /updates/ [post]
func (h *HandlerService) PostMetrics(res http.ResponseWriter, req *http.Request) {
	if strings.Contains(req.Header.Get("Content-Type"), "application/json") {
		h.PostArrayJSON(res, req)
		return
	} else if strings.Contains(req.Header.Get("Content-Type"), "application/x-gob") {
		h.PostMetricsArray(res, req)
		return
	}
}

// PostMetricsArray handles batch updates in Gob format.
func (h *HandlerService) PostMetricsArray(res http.ResponseWriter, req *http.Request) {
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
	err = h.service.FromStructToStoreBatch(req.Context(), metrics)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to write request body: %v", err), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
}

// Ping checks the database connection status.
//
// @Summary Ping database
// @Description Checks if the database is accessible.
// @Tags info
// @Success 200 {string} string "OK"
// @Failure 500 {string} string "Database connection error"
// @Router /ping [get]
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
//
// @Summary List all metrics
// @Description Returns an HTML page containing all current metrics and their values.
// @Tags info
// @Produce html
// @Success 200 {string} string "HTML content"
// @Failure 400 {string} string "Bad Request"
// @Router / [get]
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
//
// @Summary Get metric value
// @Description Returns the value of a specific metric by type and name.
// @Tags value
// @Param type path string true "Metric type (gauge or counter)"
// @Param name path string true "Metric name"
// @Produce text/plain
// @Success 200 {string} string "Metric value"
// @Failure 404 {string} string "Metric not found"
// @Failure 400 {string} string "Invalid metric type"
// @Router /value/{type}/{name} [get]
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
//
// @Summary Update metric
// @Description Updates a metric value via URL path parameters.
// @Tags update
// @Param type path string true "Metric type (gauge or counter)"
// @Param name path string true "Metric name"
// @Param value path string true "Metric value"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request or Parse Error"
// @Router /update/{type}/{name}/{value} [post]
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
