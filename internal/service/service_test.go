package service

import (
	"context"
	"strconv"
	"testing"

	metricsdto "gometrics/internal/api/metricsdto"
	storageOrig "gometrics/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPersistStorage struct{}

func (s *stubPersistStorage) FormattingLogs(_ context.Context, _ map[string]float64, _ map[string]int) error {
	return nil
}
func (s *stubPersistStorage) ImportLogs(context.Context) ([]metricsdto.Metrics, error) {
	return nil, nil
}
func (s *stubPersistStorage) GetLoopTime() int { return 0 }
func (s *stubPersistStorage) Close() error     { return nil }
func (s *stubPersistStorage) Flush() error     { return nil }
func (s *stubPersistStorage) Ping(context.Context) error {
	return nil
}

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
				{key: "g1", rawValue: "1", valueType: "gauge"},
				{key: "g2", rawValue: "2", valueType: "gauge"},
				{key: "g3", rawValue: "3", valueType: "gauge"},
				{key: "c1", rawValue: "1", valueType: "counter"},
				{key: "c2", rawValue: "2", valueType: "counter"},
				{key: "c3", rawValue: "3", valueType: "counter"},
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
				{key: "g1", rawValue: "1", valueType: "gauge"},
				{key: "g2", rawValue: "2", valueType: "gauge"},
				{key: "g3", rawValue: "3", valueType: "gauge"},
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
				{key: "c1", rawValue: "1", valueType: "counter"},
				{key: "c2", rawValue: "2", valueType: "counter"},
				{key: "c3", rawValue: "3", valueType: "counter"},
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ins := range tt.args {
				switch ins.valueType {
				case "gauge":
					valueFloat, err := strconv.ParseFloat(ins.rawValue, 64)
					require.NoError(t, err)
					tt.service.GaugeInsert(ins.key, valueFloat)
				case "counter":
					valueInt64, err := strconv.Atoi(ins.rawValue)
					require.NoError(t, err)
					tt.service.CounterInsert(ins.key, valueInt64)
				}
			}
			gauge, counter, res := tt.service.GetAllMetrics()
			got := want{gaugeKeys: gauge, counterKeys: counter, result: res}
			boolRes := assert.Equal(t, got, tt.want)
			if !boolRes {
				t.Errorf("result is different = %v, want %v", got, tt.want)
			}
		})
	}
}
