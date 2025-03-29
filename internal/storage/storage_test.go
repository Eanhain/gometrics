package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
		want    int
	}{
		{
			name:    "intValueInsert",
			storage: NewMemStorage(),
			args:    args{key: "cpu", rawValue: "6"},
			want:    200,
		},
		{
			name:    "stringValueInsert",
			storage: NewMemStorage(),
			args:    args{key: "cpu", rawValue: "six"},
			want:    400,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.storage.GaugeInsert(tt.args.key, tt.args.rawValue); got != tt.want {
				t.Errorf("memStorage.GaugeInsert() = %v, want %v", got, tt.want)
			}
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
			want:    200,
		},
		{
			name:    "floatValueInsert",
			storage: NewMemStorage(),
			args:    args{key: "cpu", rawValue: "6.1"},
			want:    400,
		},
		{
			name:    "stringtValueInsert",
			storage: NewMemStorage(),
			args:    args{key: "cpu", rawValue: "string"},
			want:    400,
		},
		{
			name: "appendToMemStorage",
			storage: &MemStorage{gauge: map[string]float64{"mem": 7.81},
				counter: map[string]int{"cpu": 6}},
			args: args{key: "cpu", rawValue: "1"},
			want: 7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "appendToMemStorage" {
				tt.storage.CounterInsert(tt.args.key, tt.args.rawValue)
				assert.Equal(t, tt.storage.GetCounter("cpu"), tt.want)
			} else if got := tt.storage.CounterInsert(tt.args.key, tt.args.rawValue); got != tt.want {
				t.Errorf("memStorage.CounterInsert() = %v, want %v", got, tt.want)
			}
		})
	}
}
