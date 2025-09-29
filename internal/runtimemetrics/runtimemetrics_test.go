package runtimemetrics

import (
	"fmt"
	"os"
	"testing"

	metricsdto "gometrics/internal/api/metricsdto"
	"gometrics/internal/service"
	"gometrics/internal/storage"

	"github.com/stretchr/testify/assert"
)

type stubPersistStorage struct{}

func (s *stubPersistStorage) GaugeInsert(string, float64) error  { return nil }
func (s *stubPersistStorage) CounterInsert(string, int) error    { return nil }
func (s *stubPersistStorage) FormattingLogs(map[string]float64, map[string]int) error {
	return nil
}
func (s *stubPersistStorage) ImportLogs() ([]metricsdto.Metrics, error) {
	return nil, nil
}
func (s *stubPersistStorage) GetFile() *os.File { return nil }
func (s *stubPersistStorage) GetLoopTime() int  { return 0 }
func (s *stubPersistStorage) Close() error      { return nil }
func (s *stubPersistStorage) Flush() error      { return nil }

func Test_runtimeUpdate_FillRepo(t *testing.T) {
	type args struct {
		metrics []string
	}
	tests := []struct {
		name    string
		ru      *runtimeUpdate
		args    args
		wantErr error
	}{
		{
			name:    "All OK",
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage(), &stubPersistStorage{})),
			args:    args{metrics: []string{"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "TotalAlloc"}},
			wantErr: nil,
		},
		{
			name:    "Wrong key",
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage(), &stubPersistStorage{})),
			args:    args{metrics: []string{"NewMetric"}},
			wantErr: fmt.Errorf("can't find value by this key"),
		},
		{
			name:    "Wrong type",
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage(), &stubPersistStorage{})),
			args:    args{metrics: []string{"BySize"}},
			wantErr: fmt.Errorf("wrong data type"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ru.FillRepo(tt.args.metrics)
			if tt.wantErr == nil {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), tt.wantErr.Error(), "expected error containing %q, got %s", tt.wantErr, err)
			}
		})
	}
}
