package service

import (
	"fmt"
	"strings"
)

type Metrics struct {
	ID    string   `json:"id"`              // имя метрики
	MType string   `json:"type"`            // параметр, принимающий значение gauge или counter
	Delta *int64   `json:"delta,omitempty"` // значение метрики в случае передачи counter
	Value *float64 `json:"value,omitempty"` // значение метрики в случае передачи gauge
}

type storage interface {
	GaugeInsert(key string, value float64) int
	CounterInsert(key string, value int) int
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetGaugeMap() map[string]float64
	GetCounterMap() map[string]int
}

type Service struct {
	store   *storage
	Metrics map[string]Metrics `json:"Metrics"`
}

func NewService(inst storage) *Service {
	return &Service{store: &inst}
}

func (s *Service) GetGauge(key string) (float64, error) {
	key = strings.ToLower(key)
	return (*s.store).GetGauge(key)
}

func (s *Service) GetCounter(key string) (int, error) {
	key = strings.ToLower(key)
	return (*s.store).GetCounter(key)
}

func (s *Service) GaugeInsert(key string, value float64) int {
	key = strings.ToLower(key)
	returnCode := (*s.store).GaugeInsert(key, value)
	if returnCode == 200 {
		metrics, err := s.FormatMetric(key, "gauge")
		if err != nil {
			returnCode = 500
		}
		s.Metrics[key] = metrics
	}
	return returnCode
}

func (s *Service) CounterInsert(key string, value int) int {
	key = strings.ToLower(key)
	returnCode := (*s.store).CounterInsert(key, value)
	if returnCode == 200 {
		metrics, err := s.FormatMetric(key, "counter")
		if err != nil {
			returnCode = 500
		}
		s.Metrics[key] = metrics
	}
	return returnCode
}

func (s *Service) FormatMetric(valueType, key string) (Metrics, error) {
	switch key {
	case "gauge":
		value, err := (*s.store).GetGauge(key)
		if err != nil {
			return Metrics{}, err
		}
		return Metrics{ID: key, MType: "gauge", Value: &value}, nil
	case "counter":
		value, err := (*s.store).GetCounter(key)
		value64 := int64(value)
		if err != nil {
			return Metrics{}, err
		}
		return Metrics{ID: key, MType: "gauge", Delta: &value64}, nil
	default:
		return Metrics{}, fmt.Errorf("this type doesn't found %s", key)
	}
}
