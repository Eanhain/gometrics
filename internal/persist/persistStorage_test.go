package persist

import (
	"context"
	"path/filepath"
	"testing"

	metricsdto "gometrics/internal/api/metricsdto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistStorageFormattingAndImport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		storeInter int
		gauges     map[string]float64
		counters   map[string]int
	}{
		{
			name:       "empty",
			storeInter: 0,
			gauges:     map[string]float64{},
			counters:   map[string]int{},
		},
		{
			name:       "only gauges",
			storeInter: 0,
			gauges: map[string]float64{
				"cpu":  1.25,
				"heap": 64,
			},
			counters: map[string]int{},
		},
		{
			name:       "mixed with flush",
			storeInter: 1,
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
			storage, err := NewPersistStorage(filepath.Join(dir, "metrics"), tc.storeInter)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, storage.Close())
			})

			require.NoError(t, storage.FormattingLogs(context.Background(), tc.gauges, tc.counters))
			if tc.storeInter != 0 {
				require.NoError(t, storage.Flush())
			}

			metrics, err := storage.ImportLogs(context.Background())
			require.NoError(t, err)
			assertPersistedMetrics(t, metrics, tc.gauges, tc.counters)
		})
	}
}

func assertPersistedMetrics(t *testing.T, metrics []metricsdto.Metrics, gauges map[string]float64, counters map[string]int) {
	t.Helper()

	gotGauges := make(map[string]float64, len(gauges))
	gotCounters := make(map[string]int, len(counters))

	for _, metric := range metrics {
		switch metric.MType {
		case "gauge":
			if metric.Value == nil {
				t.Fatalf("gauge %q has nil value", metric.ID)
			}
			gotGauges[metric.ID] = *metric.Value
		case "counter":
			if metric.Delta == nil {
				t.Fatalf("counter %q has nil delta", metric.ID)
			}
			gotCounters[metric.ID] = int(*metric.Delta)
		default:
			t.Fatalf("unexpected metric type %q", metric.MType)
		}
	}

	assert.Equal(t, gauges, gotGauges)
	assert.Equal(t, counters, gotCounters)
}
