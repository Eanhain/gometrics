package handlers

import (
	"fmt"
	"net/http"
	"strings"
)

type handlerService struct {
	storage repositories
}

type repositories interface {
	GaugeInsert(key string, value string) int
	CounterInsert(key string, value string) int
	GetGauge(key string) float64
	GetCounter(key string) int
}

func NewHandlerService(storage repositories) *handlerService {

	return &handlerService{
		storage: storage,
	}
}

func (h *handlerService) CreateHandler(funcType string) error {
	switch funcType {
	case "/update/":
		http.HandleFunc(funcType, h.UpdateMetrics)
	}
	return nil
}

func (h *handlerService) UpdateMetrics(res http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		path := strings.Split(req.URL.Path, "/")[1:]
		action := path[0]
		if len(path) < 4 || path[len(path)-1] == "" {
			res.WriteHeader(http.StatusNotFound)
		} else if len(path) > 4 {
			res.WriteHeader(http.StatusBadRequest)
		} else if action != "update" {
			res.WriteHeader(http.StatusBadRequest)
		} else {
			typeMetric := path[1]
			nameMetric := path[2]
			valueMetric := path[3]
			fmt.Printf("Инсертим %s: %s = %s \n", typeMetric, nameMetric, valueMetric)
			switch typeMetric {
			case "gauge":
				res.WriteHeader(h.storage.GaugeInsert(nameMetric, valueMetric))
			case "counter":
				res.WriteHeader(h.storage.CounterInsert(nameMetric, valueMetric))
			default:
				res.WriteHeader(http.StatusBadRequest)
			}
		}
	} else {
		res.WriteHeader(http.StatusBadRequest)
	}
}
