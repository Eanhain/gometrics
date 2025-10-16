package runtimemetrics

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"math/rand/v2"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	metricsdto "gometrics/internal/api/metricsdto"

	myCompress "gometrics/internal/compress"

	easyjson "github.com/mailru/easyjson"
)

type runtimeUpdate struct {
	mu         sync.RWMutex
	service    serviceInt
	memMetrics runtime.MemStats
	client     *resty.Client
}

type serviceInt interface {
	GaugeInsert(key string, value float64) error
	CounterInsert(key string, value int) error
	GetAllMetrics() ([]string, []string, map[string]string)
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
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
		err = ru.service.GaugeInsert(strings.ToLower(metricName), value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ru *runtimeUpdate) AddGauge(keys []string, metrics map[string]string) (output []metricsdto.Metrics, err error) {
	for _, key := range keys {
		value := metrics[key]
		valueFloat, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return []metricsdto.Metrics{}, err
		}
		metric := metricsdto.Metrics{ID: key, MType: "gauge", Value: &valueFloat}
		output = append(output, metric)
	}
	return output, nil
}

func (ru *runtimeUpdate) AddCounter(keys []string, metrics map[string]string) (output []metricsdto.Metrics, err error) {
	for _, key := range keys {
		value := metrics[key]

		valueInt, err := strconv.Atoi(value)
		int64Value := int64(valueInt)
		if err != nil {
			return []metricsdto.Metrics{}, err
		}
		metric := metricsdto.Metrics{ID: key, MType: "counter", Delta: &int64Value}
		output = append(output, metric)
	}
	return output, nil
}

func (ru *runtimeUpdate) SendMetricsGob(ctx context.Context, ticker *time.Ticker, host string, port string, compress string) error {
	var (
		bufOut          []byte
		metrics         []metricsdto.Metrics
		wg              sync.WaitGroup
		keysCounterIter []string
		keysGaugeIter   []string
	)
	curl := fmt.Sprintf("http://%v%v/updates/", host, port)
	select {
	case <-ticker.C:
		ru.mu.RLock()
		keysGauge, keysCounter, metricMaps := ru.service.GetAllMetrics()
		ru.mu.RUnlock()

		req := ru.client.R().
			SetHeader("Content-Type", "application/x-gob")
		i := 10

		for {
			metrics = []metricsdto.Metrics{}
			if len(keysCounter) <= i && len(keysCounter) > i-10 {
				keysCounterIter = keysCounter[i-10:]
			} else if i-10 >= len(keysCounter) {
				keysCounterIter = []string{}
			} else {
				keysCounterIter = keysCounter[i-10 : i]
			}

			if len(keysGauge) <= i && len(keysGauge) > i-10 {
				keysGaugeIter = keysGauge[i-10:]
			} else if i-10 >= len(keysGauge) {
				keysGaugeIter = []string{}
			} else {
				keysGaugeIter = keysGauge[i-10 : i]
			}

			wg.Add(2)
			go func() {
				defer wg.Done()
				counters, err := ru.AddCounter(keysCounterIter, metricMaps)
				if err != nil {
					panic(fmt.Errorf("error with SendMetricsGob %v", err))
				}
				ru.mu.Lock()
				metrics = append(metrics, counters...)
				ru.mu.Unlock()
			}()

			go func() {
				defer wg.Done()
				gauges, err := ru.AddGauge(keysGaugeIter, metricMaps)
				if err != nil {
					panic(fmt.Errorf("error with SendMetricsGob %v", err))
				}
				ru.mu.Lock()
				metrics = append(metrics, gauges...)
				ru.mu.Unlock()
			}()

			wg.Wait()
			if len(metrics) > 0 {
				var newBuffer bytes.Buffer
				encoder := gob.NewEncoder(&newBuffer)
				err := encoder.Encode(metrics)
				newBufferBytes := newBuffer.Bytes()
				if err != nil {
					return err
				}
				switch compress {
				case "gzip":
					bufOut, err = myCompress.Compress(newBufferBytes)
					if err != nil {
						return err
					}
					req.
						SetHeader("Accept-Encoding", "gzip").
						SetHeader("Content-Encoding", "gzip")
				case "false":
					bufOut = newBufferBytes
				default:
					bufOut = newBufferBytes
				}
				_, err = req.
					SetBody(bufOut).
					Post(curl)
				if err != nil {
					log.Println("WARN: Can't connect to metrics server")
				}
			}
			if i >= len(metricMaps) {
				break
			}
			i += 10

		}

	case <-ctx.Done():
		return nil

	}
	return nil
}

func (ru *runtimeUpdate) SendMetrics(ctx context.Context, ticker *time.Ticker, host string, port string, compress string) error {
	var bufOut []byte
	curl := fmt.Sprintf("http://%v%v/update/", host, port)
	select {
	case <-ticker.C:
		ru.mu.RLock()
		keysGauge, keysCounter, metricMaps := ru.service.GetAllMetrics()
		ru.mu.RUnlock()

		req := ru.client.R().
			SetHeader("Content-Type", "application/json")

		for _, key := range keysGauge {

			valueString := metricMaps[key]
			valueFloat, err := strconv.ParseFloat(valueString, 64)

			if err != nil {
				return err
			}
			metrics := metricsdto.Metrics{ID: key, MType: "gauge", Value: &valueFloat}
			bufTemp, err := easyjson.Marshal(metrics)
			if err != nil {
				return err
			}

			switch compress {
			case "gzip":
				bufOut, err = myCompress.Compress(bufTemp)
				if err != nil {
					return err
				}
				req.
					SetHeader("Accept-Encoding", "gzip").
					SetHeader("Content-Encoding", "gzip")
			case "false":
				bufOut = bufTemp
			default:
				bufOut = bufTemp
			}
			_, err = req.
				SetBody(bufOut).
				Post(curl)
			if err != nil {
				log.Println("WARN: Can't connect to metrics server")
			}
		}

		for _, key := range keysCounter {
			value := metricMaps[key]
			valueInt, err := strconv.Atoi(value)
			int64Value := int64(valueInt)
			if err != nil {
				return err
			}
			metrics := metricsdto.Metrics{ID: key, MType: "counter", Delta: &int64Value}
			bufTemp, err := easyjson.Marshal(metrics)
			if err != nil {
				return err
			}
			switch compress {
			case "gzip":
				bufOut, err = myCompress.Compress(bufTemp)
				if err != nil {
					return err
				}
				req.SetHeader("Accept-Encoding", "gzip").
					SetHeader("Content-Encoding", "gzip")
			case "false":
				bufOut = bufTemp
			default:
				bufOut = bufTemp
			}
			_, err = req.
				SetBody(bufOut).
				Post(curl)
			if err != nil {
				log.Println("WARN: Can't connect to metrics server")
			}

		}
	case <-ctx.Done():
		return nil

	}
	return nil
}

func (ru *runtimeUpdate) GetMetrics(ctx context.Context, ticker *time.Ticker, metrics []string) error {

	select {
	case <-ticker.C:
		ru.mu.Lock()
		defer ru.mu.Unlock()
		if err := ru.FillRepo(metrics); err != nil {
			return fmt.Errorf("collect runtime metrics: %w", err)
		}
		if err := ru.service.CounterInsert("PollCount", 1); err != nil {
			return fmt.Errorf("update counter PollCount: %w", err)
		}
		if err := ru.service.GaugeInsert("RandomValue", rand.Float64()); err != nil {
			return fmt.Errorf("update gauge RandomValue: %w", err)
		}
	case <-ctx.Done():
		return nil
	}
	return nil
}
