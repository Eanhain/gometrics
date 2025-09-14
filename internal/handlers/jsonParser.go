package handlers

import (
	"bytes"
	"io"
	"net/http"

	easyjson "github.com/mailru/easyjson"
)

type Metrics struct {
	ID    string   `json:"id"`              // имя метрики
	MType string   `json:"type"`            // параметр, принимающий значение gauge или counter
	Delta *int64   `json:"delta,omitempty"` // значение метрики в случае передачи counter
	Value *float64 `json:"value,omitempty"` // значение метрики в случае передачи gauge
}

func (h *handlerService) readJSON(body io.ReadCloser) {

}

func (h *handlerService) PostJSON(res http.ResponseWriter, req *http.Request) {
	var metric Metrics
	var buf bytes.Buffer
	// читаем тело запроса
	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	// десериализуем JSON в Metrics
	if err = easyjson.Unmarshal(buf.Bytes(), &metric); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	switch metric.MType {
	case "gauge":
		res.WriteHeader(h.service.GaugeInsert(metric.ID, *metric.Value))
	case "counter":
		res.WriteHeader(h.service.CounterInsert(metric.ID, int(*metric.Delta)))
	default:
		http.Error(res, "invalid action type", http.StatusBadRequest)
		return
	}
}

func (h *handlerService) GetJSON(res http.ResponseWriter, req *http.Request) {
	var metric Metrics
	var buf bytes.Buffer
	// читаем тело запроса
	_, err := buf.ReadFrom(req.Body)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	// десериализуем JSON в Metrics
	if err = easyjson.Unmarshal(buf.Bytes(), &metric); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	switch metric.MType {
	case "gauge":
		lVar, err := h.service.GetGauge(metric.ID)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		metric.Value = &lVar
	case "counter":
		lVar, err := h.service.GetCounter(metric.ID)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		lVar64 := int64(lVar)
		metric.Delta = &lVar64
	default:
		http.Error(res, "invalid action type", http.StatusBadRequest)
		return
	}

}
