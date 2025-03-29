package storage

import (
	"fmt"
	"net/http"
	"strconv"
)

type memStorage struct {
	gauge   map[string]float64
	counter map[string]int
}

func NewMemStorage() *memStorage {
	return &memStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int),
	}
}

func (storage *memStorage) getGauge(key string) float64 {
	return storage.gauge[key]
}

func (storage *memStorage) getCounter(key string) int {
	return storage.counter[key]
}

func (storage *memStorage) GetUpdateUrls(host string, port string) []string {
	allUrls := []string{}
	for key, value := range storage.gauge {
		url := fmt.Sprintf("http://%s%s/update/gauge/%s/%v", host, port, key, value)
		allUrls = append(allUrls, url)
	}
	for key, value := range storage.counter {
		url := fmt.Sprintf("http://%s%s/update/counter/%s/%v", host, port, key, value)
		allUrls = append(allUrls, url)
	}
	return allUrls
}

func (storage *memStorage) GaugeInsert(key string, rawValue string) int {
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return http.StatusBadRequest
	} else {
		storage.gauge[key] = value
		return http.StatusOK
	}

}

func (storage *memStorage) CounterInsert(key string, rawValue string) int {
	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return http.StatusBadRequest
	} else {
		storage.counter[key] += value
		return http.StatusOK
	}
}
