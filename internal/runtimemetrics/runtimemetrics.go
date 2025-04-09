package runtimemetrics

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type runtimeUpdate struct {
	service    serviceInt
	memMetrics runtime.MemStats
	client     *resty.Client
}

type serviceInt interface {
	GaugeInsert(key string, value float64) int
	CounterInsert(key string, value int) int
	GetUpdateUrls(host string, port string) []string
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetAllMetrics() map[string]string
}

func NewRuntimeUpdater(service serviceInt) *runtimeUpdate {
	return &runtimeUpdate{
		service:    service,
		memMetrics: runtime.MemStats{},
		client:     resty.New(),
	}
}

func (ru *runtimeUpdate) ParseGauge(rawValue reflect.Value) (float64, error) {
	TypeError := fmt.Errorf("wrong data type %s", rawValue.Kind())
	valueType := rawValue.Kind().String()
	switch valueType {
	case "uint64":
		return float64(rawValue.Uint()), nil
	case "uint32":
		return float64(rawValue.Uint()), nil
	case "float64":
		return rawValue.Float(), nil
	case "float32":
		return rawValue.Float(), nil
	default:
		return -1, TypeError
	}

}

func (ru *runtimeUpdate) FillRepo(metrics []string) error {
	runtime.ReadMemStats(&ru.memMetrics)
	v := reflect.ValueOf(ru.memMetrics)
	for _, metricName := range metrics {
		metricValue := v.FieldByName(metricName)
		ValueNotFound := fmt.Errorf("can't find value by this key %v", metricName)
		if !metricValue.IsValid() {
			return ValueNotFound
		}
		value, err := ru.ParseGauge(metricValue)
		if err != nil {
			return err
		}
		ru.service.GaugeInsert(strings.ToLower(metricName), value)
	}
	return nil
}

func (ru *runtimeUpdate) SendMetrics(host string, port string, sendTime int) {
	sendTimeDuration := time.Duration(sendTime)
	for {
		urls := ru.service.GetUpdateUrls(host, port)
		for _, url := range urls {
			_, err := ru.client.R().
				SetHeader("Content-Type", "text/plain").
				Post(url)
			if err != nil {
				fmt.Printf("Can't send metric %v\n", url)
				break
			}
		}
		time.Sleep(sendTimeDuration * time.Second)
	}

}

func (ru *runtimeUpdate) GetLoopMetrics(refreshTime int, metrics []string) {
	refreshTimeDuration := time.Duration(refreshTime)
	for {
		err := ru.FillRepo(metrics)
		if err != nil {
			panic(err)
		}
		ru.service.CounterInsert("PollCount", 1)
		ru.service.GaugeInsert("RandomValue", rand.Float64())
		time.Sleep(refreshTimeDuration * time.Second)
	}
}
