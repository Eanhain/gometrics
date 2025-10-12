package storage

import (
	"errors"
	"strings"
	"sync"
)

var ErrNotFound = errors.New("resource was not found")

type MemStorage struct {
	mu      sync.RWMutex
	gauge   map[string]float64
	counter map[string]int
	gaugeID map[string]string
	countID map[string]string
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int),
		gaugeID: make(map[string]string),
		countID: make(map[string]string),
	}
}

func (storage *MemStorage) GetGauge(key string) (float64, error) {
	key = strings.ToLower(key)
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	val, ok := storage.gauge[key]
	if ok {
		return val, nil
	}
	return val, ErrNotFound
}

func (storage *MemStorage) GetCounter(key string) (int, error) {
	key = strings.ToLower(key)
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	val, ok := storage.counter[key]
	if ok {
		return val, nil
	}
	return val, ErrNotFound
}

func (storage *MemStorage) GaugeInsert(key string, value float64) error {
	normKey := strings.ToLower(key)
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.gauge[normKey] = value
	storage.gaugeID[normKey] = key
	return nil
}

func (storage *MemStorage) CounterInsert(key string, value int) error {

	normKey := strings.ToLower(key)
	storage.mu.Lock()
	defer storage.mu.Unlock()

	storage.counter[normKey] += value
	storage.countID[normKey] = key
	return nil
}

func (storage *MemStorage) GetGaugeMap() map[string]float64 {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	copyMap := make(map[string]float64, len(storage.gauge))
	for k, v := range storage.gauge {
		orig := storage.gaugeID[k]
		if orig == "" {
			orig = k
		}
		copyMap[orig] = v
	}
	return copyMap
}

func (storage *MemStorage) GetCounterMap() map[string]int {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	copyMap := make(map[string]int, len(storage.counter))
	for k, v := range storage.counter {
		orig := storage.countID[k]
		if orig == "" {
			orig = k
		}
		copyMap[orig] = v
	}
	return copyMap
}

func (storage *MemStorage) ClearStorage() error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.gauge = make(map[string]float64)
	storage.counter = make(map[string]int)
	storage.gaugeID = make(map[string]string)
	storage.countID = make(map[string]string)
	return nil
}
