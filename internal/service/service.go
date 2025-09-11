package service

import (
	"strings"
)

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
			panic(err)
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
			panic(err)
		}
		s.Metrics[key] = metrics
	}
	return returnCode
}
