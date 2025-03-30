package runtimemetrics

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

type runtimeUpdate struct {
	storage    repositories
	memMetrics runtime.MemStats
	client     *resty.Client
}

type repositories interface {
	GaugeInsert(key string, value string) int
	CounterInsert(key string, rawValue string) int
	GetUpdateUrls(host string, port string) []string
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetAllMetrics() map[string]string
}

func NewRuntimeUpdater(storage repositories) *runtimeUpdate {
	return &runtimeUpdate{
		storage:    storage,
		memMetrics: runtime.MemStats{},
		client:     resty.New(),
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
		ValueNotFound := fmt.Errorf("по переданному ключу %v не найдено значения", metricName)
		TypeError := fmt.Errorf("неверный тип данных %s: %s", metricName, metricValue.Kind())
		if !metricValue.IsValid() {
			return ValueNotFound
		} else {
			metricStringValue := ru.ConvertToString(metricValue)
			if metricStringValue != "error" {
				ru.storage.GaugeInsert(metricName, metricStringValue)
			} else {
				return TypeError
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
			_, err := ru.client.R().
				SetHeader("Content-Type", "text/plain").
				Post(url)
			if err != nil {
				panic(err)
			}
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
