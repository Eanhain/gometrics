// Package storage provides an in-memory storage implementation for metrics.
// It supports concurrent access and case-insensitive keys.
package storage

import (
	"errors"
	"strings"
	"sync"
)

// ErrNotFound is returned when a requested metric key does not exist.
var ErrNotFound = errors.New("resource was not found")

// MemStorage implements an in-memory key-value store for gauge and counter metrics.
// It is safe for concurrent use by multiple goroutines.
type MemStorage struct {
	mu      sync.RWMutex
	gauge   map[string]float64
	counter map[string]int
	// Maps normalized (lowercase) keys to original keys for display purposes.
	gaugeID map[string]string
	countID map[string]string
}

// NewMemStorage creates and initializes a new empty MemStorage.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int),
		gaugeID: make(map[string]string),
		countID: make(map[string]string),
	}
}

// GetGauge retrieves the value of a gauge metric by key.
// The key lookup is case-insensitive.
// Returns the value and nil error if found, otherwise 0 and ErrNotFound.
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

// GetCounter retrieves the value of a counter metric by key.
// The key lookup is case-insensitive.
// Returns the value and nil error if found, otherwise 0 and ErrNotFound.
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

// GaugeInsert sets the value of a gauge metric.
// If the metric already exists, its value is overwritten.
// The key is stored in a case-insensitive manner, but the original case is preserved for display.
func (storage *MemStorage) GaugeInsert(key string, value float64) error {
	normKey := strings.ToLower(key)
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.gauge[normKey] = value
	storage.gaugeID[normKey] = key
	return nil
}

// CounterInsert adds the provided value to an existing counter metric.
// If the metric does not exist, it is initialized with the value.
// The key is stored in a case-insensitive manner, but the original case is preserved for display.
func (storage *MemStorage) CounterInsert(key string, value int) error {
	normKey := strings.ToLower(key)
	storage.mu.Lock()
	defer storage.mu.Unlock()

	storage.counter[normKey] += value
	storage.countID[normKey] = key
	return nil
}

// GetGaugeMap returns a copy of all gauge metrics.
// The keys in the returned map match the original casing used during insertion.
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

// GetCounterMap returns a copy of all counter metrics.
// The keys in the returned map match the original casing used during insertion.
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

// ClearStorage removes all metrics from the storage, resetting it to an empty state.
func (storage *MemStorage) ClearStorage() error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.gauge = make(map[string]float64)
	storage.counter = make(map[string]int)
	storage.gaugeID = make(map[string]string)
	storage.countID = make(map[string]string)
	return nil
}
