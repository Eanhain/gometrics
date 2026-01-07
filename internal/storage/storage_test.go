package storage

import (
	"fmt"
	"strconv"
	"strings"
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
		{
			name:    "floatValueInsert",
			storage: NewMemStorage(),
			args:    args{key: "usage", rawValue: "12.34"},
			want:    12.34,
		},
		{
			name:    "caseInsensitiveKey",
			storage: NewMemStorage(),
			args:    args{key: "CPU", rawValue: "55.5"},
			want:    55.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := strconv.ParseFloat(tt.args.rawValue, 64)
			require.NoError(t, err)
			err = tt.storage.GaugeInsert(tt.args.key, value)
			require.NoError(t, err)

			// Verify value using the same key
			stored, err := tt.storage.GetGauge(tt.args.key)
			require.NoError(t, err)
			assert.Equal(t, tt.want, stored)

			// Verify case insensitivity if applicable
			lowerKey := strings.ToLower(tt.args.key)
			storedLower, err := tt.storage.GetGauge(lowerKey)
			require.NoError(t, err)
			assert.Equal(t, tt.want, storedLower)
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
			want: 7, // 6 + 1
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

func TestMemStorage_GetMap_CasePreservation(t *testing.T) {
	ms := NewMemStorage()

	_ = ms.GaugeInsert("MixedCaseGauge", 1.1)
	_ = ms.CounterInsert("MixedCaseCounter", 10)

	// Check Gauges
	gauges := ms.GetGaugeMap()
	require.Len(t, gauges, 1)
	val, ok := gauges["MixedCaseGauge"] // Should be present with original case
	assert.True(t, ok, "Original key case should be preserved in GetGaugeMap")
	assert.Equal(t, 1.1, val)

	// Check Counters
	counters := ms.GetCounterMap()
	require.Len(t, counters, 1)
	valC, okC := counters["MixedCaseCounter"]
	assert.True(t, okC, "Original key case should be preserved in GetCounterMap")
	assert.Equal(t, 10, valC)
}

func TestMemStorage_ClearStorage(t *testing.T) {
	ms := NewMemStorage()
	_ = ms.GaugeInsert("g1", 1.0)
	_ = ms.CounterInsert("c1", 1)

	err := ms.ClearStorage()
	require.NoError(t, err)

	assert.Empty(t, ms.GetGaugeMap())
	assert.Empty(t, ms.GetCounterMap())
}

func TestMemStorage_ErrNotFound(t *testing.T) {
	ms := NewMemStorage()

	_, err := ms.GetGauge("non_existent")
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = ms.GetCounter("non_existent")
	assert.ErrorIs(t, err, ErrNotFound)
}

// ExampleMemStorage_CounterInsert demonstrates using counters.
func ExampleMemStorage_CounterInsert() {
	storage := NewMemStorage()

	// Increment counter
	_ = storage.CounterInsert("requests", 1)
	_ = storage.CounterInsert("requests", 1)

	val, _ := storage.GetCounter("requests")
	fmt.Printf("Requests: %d\n", val)

	// Output:
	// Requests: 2
}

// ExampleMemStorage_GaugeInsert demonstrates using gauges.
func ExampleMemStorage_GaugeInsert() {
	storage := NewMemStorage()

	// Set gauge value
	_ = storage.GaugeInsert("memory", 1024.5)

	// Update value (overwrite)
	_ = storage.GaugeInsert("memory", 512.0)

	val, _ := storage.GetGauge("memory")
	fmt.Printf("Memory: %.1f\n", val)

	// Output:
	// Memory: 512.0
}
