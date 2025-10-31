package runtimemetrics

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
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
	"gometrics/internal/clientconfig"
	myCompress "gometrics/internal/compress"
	"gometrics/internal/retry"
)

type RuntimeUpdate struct {
	mu         sync.RWMutex
	service    serviceInt
	memMetrics runtime.MemStats
	client     *resty.Client
	ChIn       chan []metricsdto.Metrics
	ChOut      chan error
	// ChDone <- chan any
	RateLimit int
}

type serviceInt interface {
	GaugeInsert(ctx context.Context, key string, value float64) error
	CounterInsert(ctx context.Context, key string, value int) error
	GetAllMetrics(ctx context.Context) ([]string, []string, map[string]string)
	GetGauge(ctx context.Context, key string) (float64, error)
	GetCounter(ctx context.Context, key string) (int, error)
}

func NewRuntimeUpdater(service serviceInt, RateLimit int) *RuntimeUpdate {
	return &RuntimeUpdate{
		service:    service,
		memMetrics: runtime.MemStats{},
		client:     resty.New(),
		ChIn:       make(chan []metricsdto.Metrics),
		ChOut:      make(chan error, RateLimit),
		RateLimit:  RateLimit,
	}
}

func (ru *RuntimeUpdate) ParseGauge(rawValue reflect.Value) (float64, error) {
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

func (ru *RuntimeUpdate) FillRepo(ctx context.Context, metrics []string) error {
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
		err = ru.service.GaugeInsert(ctx, strings.ToLower(metricName), value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ru *RuntimeUpdate) ComputeHash(ctx context.Context, body []byte, key string) ([]byte, error) {
	hmac := hmac.New(sha256.New, []byte(key))
	if _, err := hmac.Write(body); err != nil {
		return nil, err
	}
	hash := hmac.Sum(nil)
	return hash, nil
}

func (ru *RuntimeUpdate) GetMetrics(ctx context.Context, metrics []string) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		ru.mu.Lock()
		defer ru.mu.Unlock()
		if err := ru.FillRepo(ctx, metrics); err != nil {
			return fmt.Errorf("collect runtime metrics: %w", err)
		}
		if err := ru.service.CounterInsert(ctx, "PollCount", 1); err != nil {
			return fmt.Errorf("update counter PollCount: %w", err)
		}
		if err := ru.service.GaugeInsert(ctx, "RandomValue", rand.Float64()); err != nil {
			return fmt.Errorf("update gauge RandomValue: %w", err)
		}
	}
	return nil
}

func (ru *RuntimeUpdate) Sender(ctx context.Context, wg *sync.WaitGroup, worker int, ticker *time.Ticker, retryCfg retry.RetryConfig, curl string, f clientconfig.ClientConfig) {
	defer wg.Done()
	select {
	case <-ctx.Done():
		return
	default:
		if _, err := retryCfg.Retry(ctx, func(_ ...any) (any, error) {
			log.Println("run goroutine", worker)
			err := ru.SendMetricGobCh(ctx, curl, f.Compress, f.Key)
			return nil, err
		}); err != nil {
			panic(fmt.Errorf("send metrics to %s:%s: %w", f.GetHost(), f.GetPort(), err))
		}
	}

}

func (ru *RuntimeUpdate) SendMetricGobCh(ctx context.Context, curl string, compress string, key string) error {
	var (
		bufOut    []byte
		newBuffer bytes.Buffer
	)
	for metrics := range ru.ChIn {
		req := ru.client.R().SetHeader("Content-Type", "application/x-gob")
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
		if key != "" {
			hash, err := ru.ComputeHash(ctx, bufOut, key)
			if err != nil {
				return err
			}
			req.SetHeader("HashSHA256", hex.EncodeToString(hash))
		}
		_, err = req.
			SetBody(bufOut).
			Post(curl)
		if err != nil {
			log.Println("WARN: Can't connect to metrics server")
		}
		ru.ChOut <- nil
	}
	return nil
}

func (ru *RuntimeUpdate) ParseMetrics(ctx context.Context, f clientconfig.ClientConfig, metrics []string) {
	// ticker := time.NewTicker(time.Duration(f.PollInterval) * time.Second)
	// defer ticker.Stop()
	for {
		if err := ru.GetMetrics(ctx, metrics); err != nil {
			panic(fmt.Errorf("runtime metrics loop: %w", err))
		}
		ru.GeneratorBatch(ctx)
	}
}

func (ru *RuntimeUpdate) AddGauge(keys []string, metrics map[string]string) (output []metricsdto.Metrics, err error) {
	for _, key := range keys {
		value := metrics[key]
		valueFloat, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return []metricsdto.Metrics{}, err
		}
		metric := metricsdto.Metrics{ID: key, MType: metricsdto.MetricTypeGauge, Value: &valueFloat}
		output = append(output, metric)
	}
	return output, nil
}

func (ru *RuntimeUpdate) AddCounter(keys []string, metrics map[string]string) (output []metricsdto.Metrics, err error) {
	for _, key := range keys {
		value := metrics[key]

		valueInt, err := strconv.Atoi(value)
		int64Value := int64(valueInt)
		if err != nil {
			return []metricsdto.Metrics{}, err
		}
		metric := metricsdto.Metrics{ID: key, MType: metricsdto.MetricTypeCounter, Delta: &int64Value}
		output = append(output, metric)
	}
	return output, nil
}

func (ru *RuntimeUpdate) GeneratorBatch(ctx context.Context) error {

	var (
		keysCounterIter []string
		keysGaugeIter   []string
		metrics         []metricsdto.Metrics
	)

	keysGauge, keysCounter, metricMaps := ru.service.GetAllMetrics(ctx)

	i := 10

	for {

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
		counters, err := ru.AddCounter(keysCounterIter, metricMaps)
		if err != nil {
			panic(fmt.Errorf("error with SendMetricsGob %v", err))
		}
		metrics = append(metrics, counters...)

		gauges, err := ru.AddGauge(keysGaugeIter, metricMaps)

		if err != nil {
			panic(fmt.Errorf("error with SendMetricsGob %v", err))
		}

		metrics = append(metrics, gauges...)

		if err != nil {
			panic(fmt.Errorf("error with SendMetricsGob %v", err))
		}

		ru.ChIn <- metrics

		if i >= len(metricMaps) {
			break
		}
		i += 10
	}
	return nil
}

func (ru *RuntimeUpdate) GetRateLimit() int {
	return ru.RateLimit
}
