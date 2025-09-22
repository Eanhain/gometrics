package service

import (
	"fmt"
	"gometrics/internal/api/metricsdto"
	"sort"
	"strings"
)

type storage interface {
	GaugeInsert(key string, value float64) error
	CounterInsert(key string, value int) error
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetGaugeMap() map[string]float64
	GetCounterMap() map[string]int
	ClearStorage() error
}

type persistStorage interface {
	GaugeInsert(key string, value float64) error
	CounterInsert(key string, value int) error
	ImportLogs() ([]metricsdto.Metrics, error)
}

type Service struct {
	store  storage
	pstore persistStorage
}

func NewService(inst storage, inst2 persistStorage) *Service {
	return &Service{store: inst, pstore: inst2}
}

func (s *Service) GetGauge(key string) (float64, error) {
	key = strings.ToLower(key)
	return s.store.GetGauge(key)
}

func (s *Service) GetCounter(key string) (int, error) {
	key = strings.ToLower(key)
	return s.store.GetCounter(key)
}

func (s *Service) GetAllMetrics() ([]string, []string, map[string]string) {
	result := make(map[string]string)

	gaugeKeys := make([]string, 0, len(result))
	counterKeys := make([]string, 0, len(result))

	for key, gauge := range s.store.GetGaugeMap() {
		result[key] = fmt.Sprintf("%v", gauge)
		gaugeKeys = append(gaugeKeys, key)
	}

	sort.Strings(gaugeKeys)

	for key, counter := range s.store.GetCounterMap() {
		result[key] = fmt.Sprintf("%v", counter)
		counterKeys = append(counterKeys, key)
	}

	sort.Strings(counterKeys)

	return gaugeKeys, counterKeys, result
}

func (s *Service) GetAllGauges() map[string]float64 {
	gaugeMap := s.store.GetGaugeMap()
	return gaugeMap
}

func (s *Service) GetAllCounters() map[string]int {
	gaugeMap := s.store.GetCounterMap()
	return gaugeMap
}

func (s *Service) GaugeInsert(key string, value float64) error {
	key = strings.ToLower(key)
	return s.store.GaugeInsert(key, value)
}

func (s *Service) CounterInsert(key string, value int) error {
	key = strings.ToLower(key)
	return s.store.CounterInsert(key, value)
}

func (s *Service) PersistRestore() error {
	err := s.store.ClearStorage()
	if err != nil {
		return err
	}
	metrics, err := s.pstore.ImportLogs()
	if err != nil {
		return err
	}
	for _, metric := range metrics {
		err = s.FromStructToStore(metric)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Service) FromStructToStore(metric metricsdto.Metrics) error {
	switch metric.MType {
	case "gauge":
		err := h.GaugeInsert(metric.ID, *metric.Value)
		if err != nil {
			return err
		}
	case "counter":
		err := h.CounterInsert(metric.ID, int(*metric.Delta))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid action type")
	}
	return nil
}
