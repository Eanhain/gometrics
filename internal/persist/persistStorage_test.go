package persist

import (
	"context"
	"fmt"
	"os"
	"testing"

	metricsdto "gometrics/internal/api/metricsdto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPersistStorageFormattingAndImport verifies that metrics saved via FormattingLogs
// can be correctly restored via ImportLogs.
func TestPersistStorageFormattingAndImport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		storeInter int
		gauges     map[string]float64
		counters   map[string]int
	}{
		{
			name:       "empty metrics",
			storeInter: 0,
			gauges:     map[string]float64{},
			counters:   map[string]int{},
		},
		{
			name:       "only gauges",
			storeInter: 0,
			gauges: map[string]float64{
				"cpu":  1.25,
				"heap": 64.5,
			},
			counters: map[string]int{},
		},
		{
			name:       "only counters",
			storeInter: 0,
			gauges:     map[string]float64{},
			counters: map[string]int{
				"poll": 5,
			},
		},
		{
			name:       "mixed metrics with deferred flush",
			storeInter: 300, // Non-zero means we must call Flush() manually
			gauges: map[string]float64{
				"temp":  24.5,
				"usage": 0.85,
			},
			counters: map[string]int{
				"requests":  42,
				"retries":   7,
				"snapshots": 0,
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			// Create storage instance
			storage, err := NewPersistStorage(dir, tc.storeInter)
			require.NoError(t, err)

			// Ensure cleanup
			t.Cleanup(func() {
				require.NoError(t, storage.Close())
			})

			// 1. Write metrics
			err = storage.FormattingLogs(context.Background(), tc.gauges, tc.counters)
			require.NoError(t, err)

			// If interval is set, we expect data NOT to be on disk yet (technically implementation details),
			// but we MUST call Flush to ensure it's there for ImportLogs.
			if tc.storeInter != 0 {
				require.NoError(t, storage.Flush())
			}

			// 2. Read metrics back
			metrics, err := storage.ImportLogs(context.Background())
			require.NoError(t, err)

			// 3. Verify content
			assertPersistedMetrics(t, metrics, tc.gauges, tc.counters)
		})
	}
}

// TestPersistStorage_Ping verifies the Ping method functionality.
func TestPersistStorage_Ping(t *testing.T) {
	dir := t.TempDir()
	storage, err := NewPersistStorage(dir, 0)
	require.NoError(t, err)
	// Не используем defer storage.Close() здесь, так как будем закрывать вручную для теста

	// 1. Ping должен проходить, пока файл открыт
	assert.NoError(t, storage.Ping(context.Background()))

	// 2. Закрываем storage
	storage.Close()
	// В реализации Close() файл закрывается, но указатель pstorage.file остается (он не зануляется в структуре, если смотреть ваш код)
	// НО! Метод Stat() на закрытом файле вернет ошибку.

	// Проверим реализацию Close в persist.go:
	// func (pstorage *PersistStorage) Close() error { ... pstorage.file.Close() ... }
	// Поле file не зануляется. Вызов методов на закрытом файле обычно вызывает ошибку.

	err = storage.Ping(context.Background())
	assert.Error(t, err, "Ping should fail after Close()")
}

// TestPersistStorage_AgentMode verifies behavior when "agent" path is used.
func TestPersistStorage_AgentMode(t *testing.T) {
	storage, err := NewPersistStorage("agent", 0)
	require.NoError(t, err)

	// Agent mode should essentially be no-op or specific behavior
	// Check Ping or Import
	metrics, err := storage.ImportLogs(context.Background())
	require.NoError(t, err)
	assert.Empty(t, metrics)
}

// Helper to compare slice of Metrics with expected maps.
func assertPersistedMetrics(t *testing.T, metrics []metricsdto.Metrics, gauges map[string]float64, counters map[string]int) {
	t.Helper()

	gotGauges := make(map[string]float64)
	gotCounters := make(map[string]int)

	for _, metric := range metrics {
		switch metric.MType {
		case metricsdto.MetricTypeGauge:
			if metric.Value == nil {
				t.Errorf("gauge %q has nil value", metric.ID)
				continue
			}
			gotGauges[metric.ID] = *metric.Value
		case metricsdto.MetricTypeCounter:
			if metric.Delta == nil {
				t.Errorf("counter %q has nil delta", metric.ID)
				continue
			}
			gotCounters[metric.ID] = int(*metric.Delta)
		default:
			t.Errorf("unexpected metric type %q", metric.MType)
		}
	}

	assert.Equal(t, gauges, gotGauges)
	assert.Equal(t, counters, gotCounters)
}

// ExampleNewPersistStorage demonstrates usage of the storage.
func ExampleNewPersistStorage() {
	// Create a temporary directory for the example
	dir, _ := os.MkdirTemp("", "example_metrics")
	defer os.RemoveAll(dir)

	// Initialize storage with sync writing (interval 0)
	store, _ := NewPersistStorage(dir, 0)
	defer store.Close()

	// Data to save
	gauges := map[string]float64{"CpuUsage": 0.75}
	counters := map[string]int{"RequestCount": 10}

	// Save to file
	ctx := context.Background()
	_ = store.FormattingLogs(ctx, gauges, counters)

	// In a real app, you might restart here.
	// Reload data:
	loadedMetrics, _ := store.ImportLogs(ctx)

	fmt.Printf("Loaded %d metrics\n", len(loadedMetrics))

	// Output:
	// Loaded 2 metrics
}
