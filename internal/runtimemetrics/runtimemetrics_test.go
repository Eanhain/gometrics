package runtimemetrics

import (
	"fmt"
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
			ru:      NewRuntimeUpdater(storage.NewMemStorage()),
			args:    args{metrics: []string{"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "TotalAlloc"}},
			wantErr: nil,
		},
		{
			name:    "Wrong key",
			ru:      NewRuntimeUpdater(storage.NewMemStorage()),
			args:    args{metrics: []string{"NewMetric"}},
			wantErr: fmt.Errorf("не найдено значения"),
		},
		{
			name:    "Wrong type",
			ru:      NewRuntimeUpdater(storage.NewMemStorage()),
			args:    args{metrics: []string{"BySize"}},
			wantErr: fmt.Errorf("неверный тип данных"),
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
