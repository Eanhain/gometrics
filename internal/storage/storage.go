package storage

import (
	"net/http"
	"strconv"
)

type memStorage struct {
	gauge   map[string]int
	counter map[string]float64
}

func newMemStorage() *memStorage {
	return &memStorage{
		gauge:   make(map[string]int),
		counter: make(map[string]float64),
	}
}

func (storage *memStorage) gaugeInsert(key string, rawValue string) int {
	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return http.StatusBadRequest
	} else {
		storage.gauge[key] = value
		return http.StatusOK
	}

}

func (storage *memStorage) counterInsert(key string, rawValue string) int {
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return http.StatusBadRequest
	} else {
		storage.counter[key] += value
		return http.StatusOK
	}
}
