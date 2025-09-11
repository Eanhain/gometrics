package service

import "fmt"

type Metrics struct {
	ID    string   `json:"id"`              // имя метрики
	MType string   `json:"type"`            // параметр, принимающий значение gauge или counter
	Delta *int64   `json:"delta,omitempty"` // значение метрики в случае передачи counter
	Value *float64 `json:"value,omitempty"` // значение метрики в случае передачи gauge
}

func (s *Service) FormatMetric(valueType, key string) (Metrics, error) {
	switch key {
	case "gauge":
		value, err := (*s.store).GetGauge(key)
		if err != nil {
			return Metrics{}, err
		}
		return Metrics{ID: key, MType: "gauge", Value: &value}, nil
	case "counter":
		value, err := (*s.store).GetCounter(key)
		value64 := int64(value)
		if err != nil {
			return Metrics{}, err
		}
		return Metrics{ID: key, MType: "gauge", Delta: &value64}, nil
	default:
		return Metrics{}, fmt.Errorf("this type doesn't found %s", key)
	}
}
