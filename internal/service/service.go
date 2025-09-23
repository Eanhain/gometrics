package service

import (
	"fmt"
	"gometrics/internal/api/metricsdto"
	"os"
	"sort"
	"strings"
	"time"
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
	FormattingLogs(map[string]float64, map[string]int) error
	ImportLogs() ([]metricsdto.Metrics, error)
	GetFile() *os.File
	GetLoopTime() int
	Close() error
	Flush() error
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
	err := s.store.GaugeInsert(key, value)
	if err != nil {
		return err
	}
	if s.pstore.GetFile() != nil {
		gauges := s.GetAllGauges()
		counters := s.GetAllCounters()
		err := s.pstore.FormattingLogs(gauges, counters)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CounterInsert(key string, value int) error {
	key = strings.ToLower(key)
	err := s.store.CounterInsert(key, value)
	if err != nil {
		return err
	}
	if s.pstore.GetFile() != nil {
		gauges := s.GetAllGauges()
		counters := s.GetAllCounters()
		err := s.pstore.FormattingLogs(gauges, counters)
		if err != nil {
			return err
		}
	}
	return nil
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

func (s *Service) FromStructToStore(metric metricsdto.Metrics) error {
	switch metric.MType {
	case "gauge":
		err := s.GaugeInsert(metric.ID, *metric.Value)
		if err != nil {
			return err
		}
	case "counter":
		err := s.CounterInsert(metric.ID, int(*metric.Delta))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid action type")
	}
	return nil
}

func (s *Service) StorageCloser() error {
	return s.pstore.Close()
}

func (s *Service) LoopFlush() error {
	sendTimeDuration := time.Duration(s.pstore.GetLoopTime())

	for {
		time.Sleep(sendTimeDuration * time.Second)
		err := s.pstore.Flush()
		if err != nil {
			return err
		}
	}
}
