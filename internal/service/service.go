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
	value, err := s.store.GetGauge(key)
	if err != nil {
		return 0, fmt.Errorf("get gauge %s: %w", key, err)
	}
	return value, nil
}

func (s *Service) GetCounter(key string) (int, error) {
	key = strings.ToLower(key)
	value, err := s.store.GetCounter(key)
	if err != nil {
		return 0, fmt.Errorf("get counter %s: %w", key, err)
	}
	return value, nil
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
	if err := s.store.GaugeInsert(key, value); err != nil {
		return fmt.Errorf("store gauge %s: %w", key, err)
	}
	if s.pstore.GetFile() != nil {
		gauges := s.GetAllGauges()
		counters := s.GetAllCounters()
		if err := s.pstore.FormattingLogs(gauges, counters); err != nil {
			return fmt.Errorf("persist gauge %s: %w", key, err)
		}
	}
	return nil
}

func (s *Service) CounterInsert(key string, value int) error {
	key = strings.ToLower(key)
	if err := s.store.CounterInsert(key, value); err != nil {
		return fmt.Errorf("store counter %s: %w", key, err)
	}
	if s.pstore.GetFile() != nil {
		gauges := s.GetAllGauges()
		counters := s.GetAllCounters()
		if err := s.pstore.FormattingLogs(gauges, counters); err != nil {
			return fmt.Errorf("persist counter %s: %w", key, err)
		}
	}
	return nil
}

func (s *Service) PersistRestore() error {
	// err := s.store.ClearStorage()
	// if err != nil {
	// 	return err
	// }
	metrics, err := s.pstore.ImportLogs()
	if err != nil {
		return fmt.Errorf("import persisted metrics: %w", err)
	}
	for _, metric := range metrics {
		if err = s.FromStructToStore(metric); err != nil {
			return fmt.Errorf("restore metric %s: %w", metric.ID, err)
		}
	}
	return nil
}

func (s *Service) FromStructToStore(metric metricsdto.Metrics) error {
	switch metric.MType {
	case "gauge":
		if err := s.GaugeInsert(metric.ID, *metric.Value); err != nil {
			return fmt.Errorf("insert gauge %s: %w", metric.ID, err)
		}
	case "counter":
		if err := s.CounterInsert(metric.ID, int(*metric.Delta)); err != nil {
			return fmt.Errorf("insert counter %s: %w", metric.ID, err)
		}
	default:
		return fmt.Errorf("invalid action type")
	}
	return nil
}

func (s *Service) StorageCloser() error {
	if err := s.pstore.Close(); err != nil {
		return fmt.Errorf("close persist storage: %w", err)
	}
	return nil
}

func (s *Service) LoopFlush() error {
	sendTimeDuration := time.Duration(s.pstore.GetLoopTime())

	for {
		if err := s.pstore.Flush(); err != nil {
			return fmt.Errorf("flush persist storage: %w", err)
		}
		time.Sleep(sendTimeDuration * time.Second)
	}
}
