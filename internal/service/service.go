package service

import (
	"fmt"
	"sort"
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
	store *storage
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

func (s *Service) GetAllMetrics() ([]string, []string, map[string]string) {
	result := make(map[string]string)

	gaugeKeys := make([]string, 0, len(result))
	counterKeys := make([]string, 0, len(result))

	for key, gauge := range (*s.store).GetGaugeMap() {
		result[key] = fmt.Sprintf("%v", gauge)
		gaugeKeys = append(gaugeKeys, key)
	}

	sort.Strings(gaugeKeys)

	for key, counter := range (*s.store).GetCounterMap() {
		result[key] = fmt.Sprintf("%v", counter)
		counterKeys = append(counterKeys, key)
	}

	sort.Strings(counterKeys)

	return gaugeKeys, counterKeys, result
}

func (s *Service) GetAllGauges() map[string]float64 {
	gaugeMap := (*s.store).GetGaugeMap()
	return gaugeMap
}

func (s *Service) GetAllCounters() map[string]int {
	gaugeMap := (*s.store).GetCounterMap()
	return gaugeMap
}

func (s *Service) GaugeInsert(key string, value float64) int {
	key = strings.ToLower(key)
	return (*s.store).GaugeInsert(key, value)
}

func (s *Service) CounterInsert(key string, value int) int {
	key = strings.ToLower(key)
	return (*s.store).CounterInsert(key, value)
}
