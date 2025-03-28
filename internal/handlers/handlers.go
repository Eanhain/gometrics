package handlers

import (
	"net/http"
	"strings"
)

type handlerService struct {
	storage repositories
}

type repositories interface {
	gaugeInsert(key string, value string) error
	counterInsert(key string, value string) error
}

func newHandlerService(storage repositories) *handlerService {
	return &handlerService{
		storage: storage,
	}
}

func (h *handlerService) createHandler(funcType string) error {
	switch funcType {
	case "/update/":
		http.HandleFunc(funcType, h.updateMetrics)
	}
	return nil
}

func (h *handlerService) updateMetrics(res http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		path := strings.Split(req.URL.Path, "/")[1:]
		action := path[0]
		if len(path) != 4 {
			res.WriteHeader(http.StatusNotFound)
		} else if action != "/update/" {
			res.WriteHeader(http.StatusBadRequest)
		} else {
			typeMetric := path[1]
			nameMetric := path[2]
			valueMetric := path[3]
			switch typeMetric {
			case "gauge":
				h.storage.gaugeInsert(nameMetric, valueMetric)
			case "counter":
				h.storage.counterInsert(nameMetric, valueMetric)
			default:
				res.WriteHeader(http.StatusBadRequest)
			}
		}
	}
}
