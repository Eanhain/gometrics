package storage

import (
	"errors"
	"net/http"
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
	val, ok := storage.gauge[key]
	if ok {
		return val, nil
	}
	return val, ErrNotFound
}

func (storage *MemStorage) GetCounter(key string) (int, error) {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	val, ok := storage.counter[key]
	if ok {
		return val, nil
	}
	return val, ErrNotFound
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

func (storage *MemStorage) GetGaugeMap() map[string]float64 {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	return storage.gauge
}

func (storage *MemStorage) GetCounterMap() map[string]int {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	return storage.counter
}
