package storage

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

var ErrNotFound = errors.New("resource was not found")

type MemStorage struct {
	mu      sync.RWMutex
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

	storage.mu.RLock()
	defer storage.mu.RUnlock()

	key = strings.ToLower(key)
	val, ok := storage.gauge[key]
	if ok {
		return val, nil
	}
	return val, ErrNotFound
}

func (storage *MemStorage) GetCounter(key string) (int, error) {

	storage.mu.RLock()
	defer storage.mu.RUnlock()

	key = strings.ToLower(key)
	val, ok := storage.counter[key]
	if ok {
		return val, nil
	}
	return val, ErrNotFound
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

func (storage *MemStorage) GaugeInsert(key string, value float64) int {

	storage.mu.Lock()
	defer storage.mu.Unlock()

	storage.gauge[key] = value
	return http.StatusOK
}

func (storage *MemStorage) CounterInsert(key string, value int) int {

	storage.mu.Lock()
	defer storage.mu.Unlock()

	storage.counter[key] += value
	return http.StatusOK
}
