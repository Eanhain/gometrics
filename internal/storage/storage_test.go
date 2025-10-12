package storage

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_memStorage_GaugeInsert(t *testing.T) {
	type args struct {
		key      string
		rawValue string
	}
	tests := []struct {
		name    string
		storage *MemStorage
		args    args
		want    float64
	}{
		{
			name:    "intValueInsert",
			storage: NewMemStorage(),
			args:    args{key: "cpu", rawValue: "6"},
			want:    6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := strconv.ParseFloat(tt.args.rawValue, 64)
			require.NoError(t, err)
			err = tt.storage.GaugeInsert(tt.args.key, value)
			require.NoError(t, err)
			stored, err := tt.storage.GetGauge(tt.args.key)
			require.NoError(t, err)
			assert.Equal(t, tt.want, stored)
		})
	}
}

func Test_memStorage_CounterInsert(t *testing.T) {
	type args struct {
		key      string
		rawValue string
	}
	tests := []struct {
		name    string
		storage *MemStorage
		args    args
		want    int
	}{
		{
			name:    "intValueInsert",
			storage: NewMemStorage(),
			args:    args{key: "cpu", rawValue: "6"},
			want:    6,
		},
		{
			name: "appendToMemStorage",
			storage: func() *MemStorage {
				ms := NewMemStorage()
				require.NoError(t, ms.GaugeInsert("mem", 7.81))
				require.NoError(t, ms.CounterInsert("cpu", 6))
				return ms
			}(),
			args: args{key: "cpu", rawValue: "1"},
			want: 7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := strconv.Atoi(tt.args.rawValue)
			require.NoError(t, err)
			err = tt.storage.CounterInsert(tt.args.key, value)
			require.NoError(t, err)
			result, err := tt.storage.GetCounter(tt.args.key)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}
