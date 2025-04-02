package storage

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var ErrNotFound = errors.New("resource was not found")

type MemStorage struct {
	gauge   map[string]float64
	counter map[string]int
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int),
	}
}

func (storage *MemStorage) GetGauge(key string) (float64, error) {
	key = strings.ToLower(key)
	val, ok := storage.gauge[key]
	if ok {
		return val, nil
	} else {
		return val, ErrNotFound
	}

}

func (storage *MemStorage) GetCounter(key string) (int, error) {
	key = strings.ToLower(key)
	val, ok := storage.counter[key]
	if ok {
		return val, nil
	} else {
		return val, ErrNotFound
	}

}

func (storage *MemStorage) GetAllMetrics() map[string]string {
	output := make(map[string]string)
	for key, value := range storage.gauge {
		output["gauge "+key] = fmt.Sprintf("%v", value)
	}
	for key, value := range storage.counter {
		output["counter "+key] = fmt.Sprintf("%v", value)
	}
	return output
}

func (storage *MemStorage) GetUpdateUrls(host string, port string) []string {
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

func (storage *MemStorage) GaugeInsert(key string, rawValue string) int {
	key = strings.ToLower(key)
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return http.StatusBadRequest
	} else {
		storage.gauge[key] = value
		return http.StatusOK
	}

}

func (storage *MemStorage) CounterInsert(key string, rawValue string) int {
	key = strings.ToLower(key)
	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return http.StatusBadRequest
	} else {
		storage.counter[key] += value
		return http.StatusOK
	}
}
