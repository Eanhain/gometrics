package runtimemetrics

import (
	"fmt"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage())),
			args:    args{metrics: []string{"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "TotalAlloc"}},
			wantErr: nil,
		},
		{
			name:    "Wrong key",
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage())),
			args:    args{metrics: []string{"NewMetric"}},
			wantErr: fmt.Errorf("can't find value by this key"),
		},
		{
			name:    "Wrong type",
			ru:      NewRuntimeUpdater(service.NewService(storage.NewMemStorage())),
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
