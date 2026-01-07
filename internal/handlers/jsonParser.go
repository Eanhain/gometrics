package handlers

import (
	"bytes"
	"fmt"
	"net/http"

	metricsdto "gometrics/internal/api/metricsdto"

	easyjson "github.com/mailru/easyjson"
)

// PostJSON updates a single metric via JSON body.
//
// @Summary Update metric (JSON)
// @Description Updates a single metric provided in JSON format.
// @Tags update
// @Accept json
// @Produce json
// @Param metric body metricsdto.Metrics true "Metric object"
// @Success 200 {object} metricsdto.Metrics
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /update/ [post]
func (h *HandlerService) PostJSON(res http.ResponseWriter, req *http.Request) {
	var metric metricsdto.Metrics
	var buf bytes.Buffer

	res.Header().Set("Content-Type", "application/json")
	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	if err = easyjson.Unmarshal(buf.Bytes(), &metric); err != nil {
		http.Error(res, fmt.Sprintf("failed to decode metric: %v", err), http.StatusBadRequest)
		return
	}
	switch metric.MType {
	case metricsdto.MetricTypeGauge:
		if metric.Value == nil {
			http.Error(res, "field Value is required for counter", http.StatusBadRequest)
			return
		}
		if err = h.service.GaugeInsert(req.Context(), metric.ID, *metric.Value); err != nil {
			http.Error(res, fmt.Sprintf("could not store gauge metric: %v", err), http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
	case metricsdto.MetricTypeCounter:
		if metric.Delta == nil {
			http.Error(res, "delta is required for counter", http.StatusBadRequest)
			return
		}
		if err = h.service.CounterInsert(req.Context(), metric.ID, int(*metric.Delta)); err != nil {
			http.Error(res, fmt.Sprintf("could not store counter metric: %v", err), http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
	default:
		http.Error(res, "invalid action type", http.StatusBadRequest)
		return
	}
	out, err := easyjson.Marshal(metric)
	if err != nil {
		http.Error(res, fmt.Sprintf("cannot marshal metric: %v", err), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
	res.Write(out)
}

// GetJSON retrieves a single metric via JSON body request.
//
// @Summary Get metric value (JSON)
// @Description Retrieves a metric value based on ID and MType in JSON body.
// @Tags value
// @Accept json
// @Produce json
// @Param metric body metricsdto.Metrics true "Metric request object (ID, MType)"
// @Success 200 {object} metricsdto.Metrics
// @Failure 404 {string} string "Metric not found"
// @Failure 400 {string} string "Bad Request"
// @Router /value/ [post]
func (h *HandlerService) GetJSON(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	var metric metricsdto.Metrics
	var buf bytes.Buffer

	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	if err = easyjson.Unmarshal(buf.Bytes(), &metric); err != nil {
		http.Error(res, fmt.Sprintf("failed to decode metric: %v", err), http.StatusBadRequest)
		return
	}
	switch metric.MType {
	case metricsdto.MetricTypeGauge:
		lVar, err := h.service.GetGauge(req.Context(), metric.ID)
		if err != nil {
			http.Error(res, err.Error(), http.StatusNotFound)
			return
		}
		metric.Value = &lVar
	case metricsdto.MetricTypeCounter:
		lVar, err := h.service.GetCounter(req.Context(), metric.ID)
		if err != nil {
			http.Error(res, err.Error(), http.StatusNotFound)
			return
		}
		lVar64 := int64(lVar)
		metric.Delta = &lVar64
	default:
		http.Error(res, "invalid action type", http.StatusNotFound)
		return
	}
	out, err := easyjson.Marshal(metric)
	if err != nil {
		http.Error(res, fmt.Sprintf("cannot marshal metric: %v", err), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
	res.Write(out)
}

// PostArrayJSON is a helper handler for processing bulk JSON updates.
// It is used internally by PostMetrics when Content-Type is application/json.
func (h *HandlerService) PostArrayJSON(res http.ResponseWriter, req *http.Request) {
	var metrics metricsdto.MetricsArray
	var returnBuf bytes.Buffer

	res.Header().Set("Content-Type", "application/json")
	_, err := returnBuf.ReadFrom(req.Body)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	if err = easyjson.Unmarshal(returnBuf.Bytes(), &metrics); err != nil {
		http.Error(res, fmt.Sprintf("failed to decode metric: %v", err), http.StatusBadRequest)
		return
	}
	// Note: Logic here writes input back to response before processing?
	// The original code did `res.Write(returnBuf.Bytes())` which might be intentional echo
	// but usually we want to return updated metrics or just OK.
	// Preserving original logic structure but ensuring valid HTTP flow.
	// Actually, original code writes body twice? (res.Write(returnBuf) then res.Write(out))
	// I'll keep it functionally similar but fix potential HTTP header issues if needed.

	err = h.service.FromStructToStoreBatch(req.Context(), metrics)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to write request body: %v", err), http.StatusInternalServerError)
		return
	}

	out, err := easyjson.Marshal(metrics)
	if err != nil {
		http.Error(res, fmt.Sprintf("cannot marshal metric: %v", err), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
	res.Write(out)
}
