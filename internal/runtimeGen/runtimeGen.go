package runtimegen

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"time"
)

type runtimeUpdate struct {
	storage    repositories
	memMetrics runtime.MemStats
}

type repositories interface {
	GaugeInsert(key string, value string) int
	CounterInsert(key string, rawValue string) int
	GetUpdateUrls(host string, port string) []string
}

func NewRuntimeUpdater(storage repositories) *runtimeUpdate {
	return &runtimeUpdate{
		storage:    storage,
		memMetrics: runtime.MemStats{},
	}
}

func (ru *runtimeUpdate) ConvertToString(rawValue reflect.Value) string {
	valueType := rawValue.Kind().String()
	switch valueType {
	case "uint64":
		uintValue := strconv.FormatUint(rawValue.Uint(), 10)
		return uintValue
	case "uint32":
		uintValue := strconv.FormatUint(rawValue.Uint(), 10)
		return uintValue
	case "float64":
		floatValue := fmt.Sprintf("%f", rawValue.Float())
		return floatValue
	case "float32":
		floatValue := fmt.Sprintf("%f", rawValue.Float())
		return floatValue
	case "string":
		return rawValue.String()
	default:
		return "error"
	}

}

func (ru *runtimeUpdate) FillRepo(metrics []string) error {
	runtime.ReadMemStats(&ru.memMetrics)
	v := reflect.ValueOf(ru.memMetrics)
	for _, metricName := range metrics {
		metricValue := v.FieldByName(metricName)
		if !metricValue.IsValid() {
			// fmt.Printf("Поле %s не найдено \n", metricName)
		} else {
			metricStringValue := ru.ConvertToString(metricValue)
			if metricStringValue != "error" {
				ru.storage.GaugeInsert(metricName, metricStringValue)
			} else {
				// fmt.Printf("Неверный тип данных %s: %v \n", metricName, metricValue.Type())
			}
		}
	}
	return nil
}

func (ru *runtimeUpdate) SendMetrics(host string, port string, sendTime int) {
	sendTimeDuration := time.Duration(sendTime)
	for {
		urls := ru.storage.GetUpdateUrls(host, port)
		for _, url := range urls {
			resp, _ := http.Post(url, "text/plain", nil)
			resp.Body.Close()
		}
		time.Sleep(sendTimeDuration * time.Second)
	}

}

func (ru *runtimeUpdate) GetLoopMetrics(refreshTime int, metrics []string) {
	refreshTimeDuration := time.Duration(refreshTime)
	for {
		ru.FillRepo(metrics)
		ru.storage.CounterInsert("PollCount", "1")
		ru.storage.GaugeInsert("RandomValue", fmt.Sprintf("%v", rand.Float64()))
		time.Sleep(refreshTimeDuration * time.Second)
	}
}
