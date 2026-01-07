package service

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	metricsdto "gometrics/internal/api/metricsdto"
	storageOrig "gometrics/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPersistStorage mocks the persistence layer.
type stubPersistStorage struct{}

func (s *stubPersistStorage) FormattingLogs(_ context.Context, _ map[string]float64, _ map[string]int) error {
	return nil
}
func (s *stubPersistStorage) ImportLogs(context.Context) ([]metricsdto.Metrics, error) {
	// Return some mock data for restore test
	return []metricsdto.Metrics{
		{ID: "restored_gauge", MType: metricsdto.MetricTypeGauge, Value: new(float64)},
	}, nil
}
func (s *stubPersistStorage) GetLoopTime() int           { return 0 }
func (s *stubPersistStorage) Close() error               { return nil }
func (s *stubPersistStorage) Flush() error               { return nil }
func (s *stubPersistStorage) Ping(context.Context) error { return nil }

func Test_service_GetAllMetrics(t *testing.T) {
	type args struct {
		key       string
		rawValue  string
		valueType string
	}
	type want struct {
		gaugeKeys   []string
		counterKeys []string
		result      map[string]string
	}
	tests := []struct {
		name    string
		service *Service
		args    []args
		want    want
	}{
		{
			name:    "Test insert & get metrics",
			service: NewService(storageOrig.NewMemStorage(), &stubPersistStorage{}),
			args: []args{
				{key: "g1", rawValue: "1", valueType: metricsdto.MetricTypeGauge},
				{key: "g2", rawValue: "2", valueType: metricsdto.MetricTypeGauge},
				{key: "g3", rawValue: "3", valueType: metricsdto.MetricTypeGauge},
				{key: "c1", rawValue: "1", valueType: metricsdto.MetricTypeCounter},
				{key: "c2", rawValue: "2", valueType: metricsdto.MetricTypeCounter},
				{key: "c3", rawValue: "3", valueType: metricsdto.MetricTypeCounter},
			},
			want: want{
				gaugeKeys:   []string{"g1", "g2", "g3"},
				counterKeys: []string{"c1", "c2", "c3"},
				result: map[string]string{
					"g1": "1", "g2": "2", "g3": "3",
					"c1": "1", "c2": "2", "c3": "3",
				},
			},
		},
		{
			name:    "Empty test",
			service: NewService(storageOrig.NewMemStorage(), &stubPersistStorage{}),
			args:    []args{{}},
			want: want{
				counterKeys: []string{},
				gaugeKeys:   []string{},
				result:      map[string]string{},
			},
		},
		{
			name:    "Only gauge",
			service: NewService(storageOrig.NewMemStorage(), &stubPersistStorage{}),
			args: []args{
				{key: "g1", rawValue: "1", valueType: metricsdto.MetricTypeGauge},
				{key: "g2", rawValue: "2", valueType: metricsdto.MetricTypeGauge},
				{key: "g3", rawValue: "3", valueType: metricsdto.MetricTypeGauge},
			},
			want: want{
				counterKeys: []string{},
				gaugeKeys:   []string{"g1", "g2", "g3"},
				result: map[string]string{
					"g1": "1", "g2": "2", "g3": "3",
				},
			},
		},
		{
			name:    "Only counter",
			service: NewService(storageOrig.NewMemStorage(), &stubPersistStorage{}),
			args: []args{
				{key: "c1", rawValue: "1", valueType: metricsdto.MetricTypeCounter},
				{key: "c2", rawValue: "2", valueType: metricsdto.MetricTypeCounter},
				{key: "c3", rawValue: "3", valueType: metricsdto.MetricTypeCounter},
			},
			want: want{
				gaugeKeys:   []string{},
				counterKeys: []string{"c1", "c2", "c3"},
				result: map[string]string{
					"c1": "1", "c2": "2", "c3": "3",
				},
			},
		},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ins := range tt.args {
				if ins.key == "" {
					continue
				}
				switch ins.valueType {
				case metricsdto.MetricTypeGauge:
					valueFloat, err := strconv.ParseFloat(ins.rawValue, 64)
					require.NoError(t, err)
					tt.service.GaugeInsert(ctx, ins.key, valueFloat)
				case metricsdto.MetricTypeCounter:
					valueInt64, err := strconv.Atoi(ins.rawValue)
					require.NoError(t, err)
					tt.service.CounterInsert(ctx, ins.key, valueInt64)
				}
			}
			gauge, counter, res := tt.service.GetAllMetrics(ctx)
			got := want{gaugeKeys: gauge, counterKeys: counter, result: res}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_GetGauge_Error(t *testing.T) {
	s := NewService(storageOrig.NewMemStorage(), &stubPersistStorage{})
	_, err := s.GetGauge(context.Background(), "non-existent")
	assert.Error(t, err)
}

func TestService_GetCounter_Error(t *testing.T) {
	s := NewService(storageOrig.NewMemStorage(), &stubPersistStorage{})
	_, err := s.GetCounter(context.Background(), "non-existent")
	assert.Error(t, err)
}

func TestService_FromStructToStoreBatch(t *testing.T) {
	s := NewService(storageOrig.NewMemStorage(), &stubPersistStorage{})
	val := 10.5
	delta := int64(5)

	metrics := []metricsdto.Metrics{
		{ID: "g_batch", MType: metricsdto.MetricTypeGauge, Value: &val},
		{ID: "c_batch", MType: metricsdto.MetricTypeCounter, Delta: &delta},
	}

	err := s.FromStructToStoreBatch(context.Background(), metrics)
	require.NoError(t, err)

	g, err := s.GetGauge(context.Background(), "g_batch")
	assert.NoError(t, err)
	assert.Equal(t, val, g)

	c, err := s.GetCounter(context.Background(), "c_batch")
	assert.NoError(t, err)
	assert.Equal(t, int(delta), c)
}

func TestService_PersistRestore(t *testing.T) {
	// Uses stubPersistStorage which returns "restored_gauge"
	s := NewService(storageOrig.NewMemStorage(), &stubPersistStorage{})

	err := s.PersistRestore(context.Background())
	require.NoError(t, err)

	// Check if "restored_gauge" is in memory
	_, err = s.GetGauge(context.Background(), "restored_gauge")
	assert.NoError(t, err)
}

// ExampleService_GaugeInsert demonstrates inserting a gauge metric.
func ExampleService_GaugeInsert() {
	// Initialize service with memory storage and mock persistence
	svc := NewService(storageOrig.NewMemStorage(), &stubPersistStorage{})

	ctx := context.Background()
	_ = svc.GaugeInsert(ctx, "Temperature", 22.5)

	val, _ := svc.GetGauge(ctx, "Temperature")
	fmt.Printf("Temperature: %.1f\n", val)

	// Output:
	// Temperature: 22.5
}
