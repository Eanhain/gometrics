// Package service implements the core business logic for metric processing and storage management.
// It acts as an intermediary between the HTTP handlers and the data storage layers (memory/DB/file).
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"gometrics/internal/api/metricsdto"
)

// storage defines the interface for in-memory or database metric operations.
type storage interface {
	GaugeInsert(key string, value float64) error
	CounterInsert(key string, value int) error
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetGaugeMap() map[string]float64
	GetCounterMap() map[string]int
	ClearStorage() error
}

// persistStorage defines the interface for persistent storage (file or database).
type persistStorage interface {
	FormattingLogs(context.Context, map[string]float64, map[string]int) error
	ImportLogs(context.Context) ([]metricsdto.Metrics, error)
	GetLoopTime() int
	Close() error
	Flush() error
	Ping(ctx context.Context) error
}

// Service aggregates the main storage and persistent storage to manage application state.
type Service struct {
	store  storage
	pstore persistStorage
}

// NewService creates a new Service instance with the provided storage backends.
func NewService(inst storage, inst2 persistStorage) *Service {
	return &Service{store: inst, pstore: inst2}
}

// Ping checks the availability of the persistent storage (e.g., database connection).
func (s *Service) Ping(ctx context.Context) error {
	return s.pstore.Ping(ctx)
}

// GetGauge retrieves the value of a gauge metric by key.
// Keys are case-insensitive.
func (s *Service) GetGauge(ctx context.Context, key string) (float64, error) {
	key = strings.ToLower(key)
	value, err := s.store.GetGauge(key)
	if err != nil {
		return 0, fmt.Errorf("get gauge %s: %w", key, err)
	}
	return value, nil
}

// GetCounter retrieves the value of a counter metric by key.
// Keys are case-insensitive.
func (s *Service) GetCounter(ctx context.Context, key string) (int, error) {
	key = strings.ToLower(key)
	value, err := s.store.GetCounter(key)
	if err != nil {
		return 0, fmt.Errorf("get counter %s: %w", key, err)
	}
	return value, nil
}

// GetAllMetrics retrieves all metrics as sorted slices of keys and a map of string values.
// Returns:
//   - gaugeKeys: sorted list of gauge metric names.
//   - counterKeys: sorted list of counter metric names.
//   - result: map of all metrics formatted as strings.
func (s *Service) GetAllMetrics(ctx context.Context) ([]string, []string, map[string]string) {
	result := make(map[string]string)

	gaugeKeys := make([]string, 0, len(result)) // len(result) is 0 initially, might want predefined cap
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

// GetAllGauges returns a map of all gauge metrics.
func (s *Service) GetAllGauges(ctx context.Context) map[string]float64 {
	gaugeMap := s.store.GetGaugeMap()
	return gaugeMap
}

// GetAllCounters returns a map of all counter metrics.
func (s *Service) GetAllCounters(ctx context.Context) map[string]int {
	counterMap := s.store.GetCounterMap()
	return counterMap
}

// GaugeInsert updates a gauge metric.
// It also triggers persistence if the storage is available and configured for synchronous writes.
func (s *Service) GaugeInsert(ctx context.Context, key string, value float64) error {
	if err := s.store.GaugeInsert(key, value); err != nil {
		return fmt.Errorf("store gauge %s: %w", key, err)
	}

	// If persistence layer is active/connected, try to save immediately (synchronous backup strategy)
	// NOTE: This might be heavy if persistence is slow (e.g. file IO on every write).
	if s.pstore.Ping(ctx) == nil {
		gauges := s.GetAllGauges(ctx)
		counters := s.GetAllCounters(ctx)
		if err := s.pstore.FormattingLogs(context.Background(), gauges, counters); err != nil {
			return fmt.Errorf("persist gauge %s: %w", key, err)
		}
	}
	return nil
}

// CounterInsert updates a counter metric.
// It also triggers persistence if the storage is available.
func (s *Service) CounterInsert(ctx context.Context, key string, value int) error {
	if err := s.store.CounterInsert(key, value); err != nil {
		return fmt.Errorf("store counter %s: %w", key, err)
	}
	if s.pstore.Ping(context.Background()) == nil {
		gauges := s.GetAllGauges(ctx)
		counters := s.GetAllCounters(ctx)
		if err := s.pstore.FormattingLogs(context.Background(), gauges, counters); err != nil {
			return fmt.Errorf("persist counter %s: %w", key, err)
		}
	}
	return nil
}

// PersistRestore loads metrics from the persistent storage into the in-memory storage.
// Typically called on application startup.
func (s *Service) PersistRestore(ctx context.Context) error {
	// Clearning storage is commented out in original code, implying additive restore or fresh start.
	// err := s.store.ClearStorage()
	metrics, err := s.pstore.ImportLogs(context.Background())
	if err != nil {
		return fmt.Errorf("import persisted metrics: %w", err)
	}
	for _, metric := range metrics {
		if err = s.FromStructToStore(ctx, metric); err != nil {
			return fmt.Errorf("restore metric %s: %w", metric.ID, err)
		}
	}
	return nil
}

// PersistFlush сбрасывает все текущие метрики из памяти в persistent storage.
// Используется для финального сохранения данных при graceful shutdown.
func (s *Service) PersistFlush(ctx context.Context) error {
	if s.pstore == nil {
		return nil
	}

	// Проверяем доступность хранилища
	if err := s.pstore.Ping(ctx); err != nil {
		// Если ping не прошёл, пробуем просто сделать flush
		// (для file storage ping может не работать)
		return s.pstore.Flush()
	}

	// Сохраняем все метрики
	gauges := s.GetAllGauges(ctx)
	counters := s.GetAllCounters(ctx)

	if err := s.pstore.FormattingLogs(ctx, gauges, counters); err != nil {
		return fmt.Errorf("persist flush formatting: %w", err)
	}

	// Вызываем flush для записи на диск
	if err := s.pstore.Flush(); err != nil {
		return fmt.Errorf("persist flush: %w", err)
	}

	return nil
}

// FromStructToStore updates the storage with a single metric DTO.
// Handles both Gauge and Counter types.
func (s *Service) FromStructToStore(ctx context.Context, metric metricsdto.Metrics) error {
	switch metric.MType {
	case metricsdto.MetricTypeGauge:
		if metric.Value == nil {
			value := float64(0)
			metric.Value = &value
		}
		if err := s.GaugeInsert(ctx, metric.ID, *metric.Value); err != nil {
			return fmt.Errorf("insert gauge %s: %w", metric.ID, err)
		}
	case metricsdto.MetricTypeCounter:
		if metric.Delta == nil {
			value := int64(0)
			metric.Delta = &value
		}
		if err := s.CounterInsert(ctx, metric.ID, int(*metric.Delta)); err != nil {
			return fmt.Errorf("insert counter %s: %w", metric.ID, err)
		}
	default:
		return fmt.Errorf("invalid action type")
	}
	return nil
}

// FromStructToStoreBatch updates the storage with a batch of metric DTOs.
func (s *Service) FromStructToStoreBatch(ctx context.Context, metrics []metricsdto.Metrics) error {
	for _, metric := range metrics {
		err := s.FromStructToStore(ctx, metric)
		if err != nil {
			return fmt.Errorf("cannot write batch %s: %w", metric.ID, err)
		}
	}
	return nil
}

// StorageCloser closes the persistent storage connection.
func (s *Service) StorageCloser() error {
	if s.pstore == nil {
		return nil
	}
	if err := s.pstore.Close(); err != nil {
		return fmt.Errorf("close persist storage: %w", err)
	}
	return nil
}

// LoopFlush starts an infinite loop to periodically flush metrics to persistent storage.
// The interval is determined by pstore.GetLoopTime().
// This is a blocking call and should typically be run in a goroutine.
// Для graceful shutdown используйте LoopFlushWithContext.
func (s *Service) LoopFlush() error {
	return s.LoopFlushWithContext(context.Background())
}

// LoopFlushWithContext периодически сбрасывает данные на диск с поддержкой отмены через контекст.
// Корректно завершается при отмене контекста, что позволяет реализовать graceful shutdown.
func (s *Service) LoopFlushWithContext(ctx context.Context) error {
	if s.pstore == nil {
		return nil
	}

	sendTimeDuration := time.Duration(s.pstore.GetLoopTime()) * time.Second
	ticker := time.NewTicker(sendTimeDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Контекст отменён - выходим из цикла
			return ctx.Err()
		case <-ticker.C:
			if err := s.pstore.Flush(); err != nil {
				return fmt.Errorf("flush persist storage: %w", err)
			}
		}
	}
}
