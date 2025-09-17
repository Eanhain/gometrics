package metricsdto

//go:generate easyjson -all .

//easyjson:json
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`            // "gauge" | "counter"
	Delta *int64   `json:"delta,omitempty"` // для counter
	Value *float64 `json:"value,omitempty"` // для gauge и ответов
}
