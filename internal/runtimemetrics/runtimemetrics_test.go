package runtimemetrics

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	metricsdto "gometrics/internal/api/metricsdto"
	"gometrics/internal/service"
	"gometrics/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPersistStorage is a mock storage implementation for testing.
type stubPersistStorage struct{}

func (s *stubPersistStorage) FormattingLogs(context.Context, map[string]float64, map[string]int) error {
	return nil
}
func (s *stubPersistStorage) ImportLogs(context.Context) ([]metricsdto.Metrics, error) {
	return nil, nil
}
func (s *stubPersistStorage) GetLoopTime() int           { return 0 }
func (s *stubPersistStorage) Close() error               { return nil }
func (s *stubPersistStorage) Flush() error               { return nil }
func (s *stubPersistStorage) Ping(context.Context) error { return nil }

func Test_runtimeUpdate_FillRepo(t *testing.T) {
	type args struct {
		metrics []string
	}
	tests := []struct {
		name    string
		ru      *RuntimeUpdate
		args    args
		wantErr error
	}{
		{
			name:    "All OK",
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage(), &stubPersistStorage{}), 1),
			args:    args{metrics: []string{"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "TotalAlloc"}},
			wantErr: nil,
		},
		{
			name:    "Wrong key",
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage(), &stubPersistStorage{}), 1),
			args:    args{metrics: []string{"NewMetric"}},
			wantErr: fmt.Errorf("can't find value by this key"),
		},
		{
			name:    "Wrong type",
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage(), &stubPersistStorage{}), 1),
			args:    args{metrics: []string{"BySize"}}, // BySize is an array/slice in MemStats, not scalar
			wantErr: fmt.Errorf("wrong data type"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ru.FillRepo(context.Background(), tt.args.metrics)
			if tt.wantErr == nil {
				assert.Nil(t, err)
			} else {
				// Use ErrorContains if available or Contains string check
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.wantErr)
				} else {
					assert.Contains(t, err.Error(), tt.wantErr.Error())
				}
			}
		})
	}
}

func TestRuntimeUpdate_ParseGauge(t *testing.T) {
	ru := NewRuntimeUpdater(nil, 1)

	tests := []struct {
		name    string
		input   interface{}
		want    float64
		wantErr bool
	}{
		{"uint64", uint64(42), 42.0, false},
		{"uint32", uint32(10), 10.0, false},
		{"float64", float64(12.34), 12.34, false},
		{"string (invalid)", "invalid", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := reflect.ValueOf(tt.input)
			got, err := ru.ParseGauge(val)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRuntimeUpdate_ComputeHash(t *testing.T) {
	ru := NewRuntimeUpdater(nil, 1)
	key := "secret"
	data := []byte("test-data")

	hash1, err := ru.ComputeHash(context.Background(), data, key)
	require.NoError(t, err)
	require.NotEmpty(t, hash1)

	hash2, err := ru.ComputeHash(context.Background(), data, key)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "Hashes for same data and key should be identical")

	hash3, err := ru.ComputeHash(context.Background(), data, "other-secret")
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash3, "Hashes for different keys should differ")
}

func TestRuntimeUpdate_ConvertToDTO(t *testing.T) {
	ru := NewRuntimeUpdater(nil, 1)
	ctx := context.Background()

	metricMaps := map[string]string{
		"c1": "100",
		"g1": "12.5",
	}

	counters := []string{"c1"}
	gauges := []string{"g1"}

	metrics, err := ru.ConvertToDTO(ctx, counters, gauges, metricMaps)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	for _, m := range metrics {
		if m.ID == "c1" {
			assert.Equal(t, metricsdto.MetricTypeCounter, m.MType)
			assert.Equal(t, int64(100), *m.Delta)
		} else if m.ID == "g1" {
			assert.Equal(t, metricsdto.MetricTypeGauge, m.MType)
			assert.Equal(t, 12.5, *m.Value)
		}
	}
}

// ExampleRuntimeUpdate_GetMetrics demonstrates how to trigger metric collection.
func ExampleRuntimeUpdate_GetMetrics() {
	// Setup service
	memStore := storage.NewMemStorage()
	svc := service.NewService(memStore, &stubPersistStorage{})
	ru := NewRuntimeUpdater(svc, 10)

	ctx := context.Background()

	// Collect basic metrics
	err := ru.GetMetrics(ctx, []string{"Alloc", "Sys"}, false)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Verify they are in storage
	_, _, storedMetrics := svc.GetAllMetrics(ctx)
	if _, ok := storedMetrics["Alloc"]; ok {
		fmt.Println("Alloc metric collected")
	}
	if _, ok := storedMetrics["PollCount"]; ok {
		fmt.Println("PollCount collected")
	}

	// Output:
	// Alloc metric collected
	// PollCount collected
}

// ExampleRuntimeUpdate_SendBatch demonstrates adding metrics to the send channel.
func ExampleRuntimeUpdate_SendBatch() {
	ru := NewRuntimeUpdater(nil, 5)

	metrics := []metricsdto.Metrics{
		{ID: "test", MType: "gauge"},
	}

	// This sends to channel non-blocking if buffer exists
	go func() {
		ru.SendBatch(context.Background(), metrics)
		ru.CloseChannel(context.Background())
	}()

	// Simulate reader
	received := <-ru.ChIn
	fmt.Printf("Received batch size: %d\n", len(received))

	// Output:
	// Received batch size: 1
}

func TestRuntimeUpdate_SendBatch_Blocking(t *testing.T) {
	// Test that channel behavior works as expected
	ru := NewRuntimeUpdater(nil, 1) // Buffer size 1

	metrics := []metricsdto.Metrics{{ID: "m1"}}

	// First send should be fine
	ru.SendBatch(context.Background(), metrics)

	// Verify channel len
	assert.Equal(t, 1, len(ru.ChIn))

	// Read it out
	out := <-ru.ChIn
	assert.Equal(t, metrics, out)
}

func TestRuntimeUpdate_AddGauge_Error(t *testing.T) {
	ru := NewRuntimeUpdater(nil, 1)

	metricsMap := map[string]string{"g1": "invalid-float"}
	keys := []string{"g1"}

	_, err := ru.AddGauge(keys, metricsMap)
	assert.Error(t, err)
}

func TestRuntimeUpdate_AddCounter_Error(t *testing.T) {
	ru := NewRuntimeUpdater(nil, 1)

	metricsMap := map[string]string{"c1": "invalid-int"}
	keys := []string{"c1"}

	_, err := ru.AddCounter(keys, metricsMap)
	assert.Error(t, err)
}
