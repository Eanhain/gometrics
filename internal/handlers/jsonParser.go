package handlers

import (
	"bytes"
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
		http.Error(res, "failed to read request body", http.StatusInternalServerError)
		return
	}
	// десериализуем JSON в Metrics
	if err = easyjson.Unmarshal(buf.Bytes(), &metric); err != nil {
		http.Error(res, "failed to decode metric", http.StatusBadRequest)
		return
	}
	switch metric.MType {
	case "gauge":
		if metric.Value == nil {
			http.Error(res, "field Value is required for counter", http.StatusBadRequest)
			return
		}
		if err = h.service.GaugeInsert(metric.ID, *metric.Value); err != nil {
			http.Error(res, "could not store gauge metric", http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
	case "counter":
		if metric.Delta == nil {
			http.Error(res, "delta is required for counter", http.StatusBadRequest)
			return
		}
		if err = h.service.CounterInsert(metric.ID, int(*metric.Delta)); err != nil {
			http.Error(res, "could not store counter metric", http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
	default:
		http.Error(res, "invalid action type", http.StatusBadRequest)
		return
	}
	out, err := easyjson.Marshal(metric)
	if err != nil {
		http.Error(res, "cannot marshal metric", http.StatusInternalServerError)
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
		http.Error(res, "failed to read request body", http.StatusInternalServerError)
		return
	}
	// десериализуем JSON в Metrics
	if err = easyjson.Unmarshal(buf.Bytes(), &metric); err != nil {
		http.Error(res, "failed to decode metric", http.StatusBadRequest)
		return
	}
	switch metric.MType {
	case "gauge":
		lVar, err := h.service.GetGauge(metric.ID)
		if err != nil {
			http.Error(res, err.Error(), http.StatusNotFound)
			return
		}
		metric.Value = &lVar
		res.WriteHeader(http.StatusOK)
	case "counter":
		lVar, err := h.service.GetCounter(metric.ID)
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
		http.Error(res, "cannot marshal metric", http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
	res.Write(out)

}
