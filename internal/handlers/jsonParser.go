package handlers

import (
	"bytes"
	"fmt"
	"net/http"

	metricsdto "gometrics/internal/api/metricsdto"

	easyjson "github.com/mailru/easyjson"
)

func (h *handlerService) PostJSON(res http.ResponseWriter, req *http.Request) {
	var metric metricsdto.Metrics
	var buf bytes.Buffer

	res.Header().Set("Content-Type", "application/json")
	// читаем тело запроса
	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	// десериализуем JSON в Metrics
	if err = easyjson.Unmarshal(buf.Bytes(), &metric); err != nil {
		http.Error(res, fmt.Sprintf("failed to decode metric: %v", err), http.StatusBadRequest)
		return
	}
	switch metric.MType {
	case "gauge":
		if metric.Value == nil {
			http.Error(res, "field Value is required for counter", http.StatusBadRequest)
			return
		}
		if err = h.service.GaugeInsert(req.Context(), metric.ID, *metric.Value); err != nil {
			http.Error(res, fmt.Sprintf("could not store gauge metric: %v", err), http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
	case "counter":
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

func (h *handlerService) GetJSON(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	var metric metricsdto.Metrics
	var buf bytes.Buffer
	// читаем тело запроса
	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	// десериализуем JSON в Metrics
	if err = easyjson.Unmarshal(buf.Bytes(), &metric); err != nil {
		http.Error(res, fmt.Sprintf("failed to decode metric: %v", err), http.StatusBadRequest)
		return
	}
	switch metric.MType {
	case "gauge":
		lVar, err := h.service.GetGauge(req.Context(), metric.ID)
		if err != nil {
			http.Error(res, err.Error(), http.StatusNotFound)
			return
		}
		metric.Value = &lVar
		res.WriteHeader(http.StatusOK)
	case "counter":
		lVar, err := h.service.GetCounter(req.Context(), metric.ID)
		if err != nil {
			http.Error(res, err.Error(), http.StatusNotFound)
			return
		}
		lVar64 := int64(lVar)
		metric.Delta = &lVar64
		res.WriteHeader(http.StatusOK)
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

func (h *handlerService) PostArrayJSON(res http.ResponseWriter, req *http.Request) {
	var metrics metricsdto.MetricsArray
	var returnBuf bytes.Buffer

	res.Header().Set("Content-Type", "application/json")
	// читаем тело запроса
	_, err := returnBuf.ReadFrom(req.Body)
	if err != nil {
		http.Error(res, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	// десериализуем JSON в Metrics
	if err = easyjson.Unmarshal(returnBuf.Bytes(), &metrics); err != nil {
		http.Error(res, fmt.Sprintf("failed to decode metric: %v", err), http.StatusBadRequest)
		return
	}
	res.Write(returnBuf.Bytes())

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
