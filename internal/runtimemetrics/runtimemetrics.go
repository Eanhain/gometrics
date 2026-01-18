// Package runtimemetrics provides functionality for collecting runtime statistics
// (such as memory usage and CPU utilization) and sending them to a remote metrics server.
package runtimemetrics

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rsa"
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

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/go-resty/resty/v2"

	metricsdto "gometrics/internal/api/metricsdto"
	"gometrics/internal/clientconfig"
	myCompress "gometrics/internal/compress"
	"gometrics/internal/netutil" // Новый импорт для получения локального IP
	"gometrics/internal/retry"
	"gometrics/internal/signature"
)

// RuntimeUpdate manages the collection and transmission of runtime metrics.
// It holds the state required for buffering metrics, handling rate limits,
// and communicating with the storage service and external client.
type RuntimeUpdate struct {
	mu         sync.RWMutex
	service    Service
	memMetrics runtime.MemStats
	client     *resty.Client
	ChIn       chan []metricsdto.Metrics
	RateLimit  int
	PubKey     *rsa.PublicKey
	localIP    string // Кэшированный IP-адрес хоста агента
}

// Service defines the interface for interacting with the local metrics storage/service.
type Service interface {
	GaugeInsert(ctx context.Context, key string, value float64) error
	CounterInsert(ctx context.Context, key string, value int) error
	GetAllMetrics(ctx context.Context) ([]string, []string, map[string]string)
	GetGauge(ctx context.Context, key string) (float64, error)
	GetCounter(ctx context.Context, key string) (int, error)
}

// NewRuntimeUpdater creates a new instance of RuntimeUpdate.
//
// Arguments:
//   - service: The local service interface for storing collected metrics before sending.
//   - RateLimit: The capacity of the internal channel for outgoing metric batches.
func NewRuntimeUpdater(service Service, RateLimit int, pubKey *rsa.PublicKey) *RuntimeUpdate {
	// Получаем IP-адрес хоста агента при инициализации
	localIP, err := netutil.GetOutboundIPString()
	if err != nil {
		log.Printf("WARNING: failed to get local IP address: %v", err)
		// Пробуем альтернативный метод
		localIP, err = netutil.GetLocalIP()
		if err != nil {
			log.Printf("WARNING: failed to get local IP via interfaces: %v", err)
			localIP = "" // Будет пустым, сервер может отклонить запрос
		}
	}

	return &RuntimeUpdate{
		service:    service,
		memMetrics: runtime.MemStats{},
		client:     resty.New(),
		ChIn:       make(chan []metricsdto.Metrics, RateLimit),
		RateLimit:  RateLimit,
		PubKey:     pubKey,
		localIP:    localIP, // Сохраняем IP для использования в заголовках
	}
}

// FillRepoExt collects extended metrics using gopsutil (VirtualMemory, CPU).
// It saves the collected metrics (TotalMemory, FreeMemory, CPUUtilization) into the local service.
//
// Argument 'metrics' expects a slice of 3 strings naming the keys for Total, Free, and CPU respectively.
func (ru *RuntimeUpdate) FillRepoExt(ctx context.Context, metrics []string) error {
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return err
	}
	ru.mu.Lock()
	defer ru.mu.Unlock()
	if err = ru.service.GaugeInsert(ctx, strings.ToLower(metrics[0]), float64(vmem.Total)); err != nil {
		return err
	}
	if err = ru.service.GaugeInsert(ctx, strings.ToLower(metrics[1]), float64(vmem.Free)); err != nil {
		return err
	}
	// Assuming cpuPercent returns at least one value
	if len(cpuPercent) > 0 {
		if err = ru.service.GaugeInsert(ctx, strings.ToLower(metrics[2]), cpuPercent[0]); err != nil {
			return err
		}
	}

	return nil
}

// ParseGauge converts a reflect.Value (from runtime.MemStats fields) to a float64.
// It supports uint64, uint32, float64, and float32 types.
func (ru *RuntimeUpdate) ParseGauge(rawValue reflect.Value) (float64, error) {
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
		return -1, fmt.Errorf("wrong data type %s", rawValue.Kind())
	}
}

// FillRepo collects standard Go runtime statistics (MemStats) and saves them to the local service.
// It iterates over the provided 'metrics' names and extracts corresponding fields from runtime.MemStats using reflection.
func (ru *RuntimeUpdate) FillRepo(ctx context.Context, metrics []string) error {
	runtime.ReadMemStats(&ru.memMetrics)
	metricsGauge := make(map[string]float64, len(metrics))
	v := reflect.ValueOf(ru.memMetrics)

	for _, metricName := range metrics {
		metricValue := v.FieldByName(metricName)
		if !metricValue.IsValid() {
			return fmt.Errorf("can't find value by this key %v", metricName)
		}
		value, err := ru.ParseGauge(metricValue)
		if err != nil {
			return err
		}
		metricsGauge[metricName] = value
	}

	ru.mu.Lock()
	defer ru.mu.Unlock()
	for key, value := range metricsGauge {
		err := ru.service.GaugeInsert(ctx, key, value)
		if err != nil {
			return err
		}
	}

	return nil
}

// ComputeHash calculates the HMAC-SHA256 hash of the provided body using the given key.
func (ru *RuntimeUpdate) ComputeHash(ctx context.Context, body []byte, key string) ([]byte, error) {
	hmac := hmac.New(sha256.New, []byte(key))
	if _, err := hmac.Write(body); err != nil {
		return nil, err
	}
	hash := hmac.Sum(nil)
	return hash, nil
}

// GetMetrics triggers the collection of metrics.
// If 'ext' is true, it collects extended system metrics (CPU/Mem).
// If 'ext' is false, it collects runtime MemStats and updates PollCount/RandomValue.
func (ru *RuntimeUpdate) GetMetrics(ctx context.Context, metrics []string, ext bool) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		if !ext {
			if err := ru.FillRepo(ctx, metrics); err != nil {
				return fmt.Errorf("collect runtime metrics: %w", err)
			}
			ru.mu.Lock()
			defer ru.mu.Unlock()
			if err := ru.service.CounterInsert(ctx, "PollCount", 1); err != nil {
				return fmt.Errorf("update counter PollCount: %w", err)
			}
			if err := ru.service.GaugeInsert(ctx, "RandomValue", rand.Float64()); err != nil {
				return fmt.Errorf("update gauge RandomValue: %w", err)
			}
		} else {
			if err := ru.FillRepoExt(ctx, metrics); err != nil {
				return fmt.Errorf("collect runtime metrics: %w", err)
			}
		}
	}
	return nil
}

// SendMetricGobCh continuously reads batches of metrics from the input channel (ChIn),
// encodes them using Gob, optionally compresses them with gzip, signs them with HMAC (if key is present),
// and sends them to the server URL (curl).

// SendMetricGobCh continuously reads batches of metrics from the input channel,
// encodes them using Gob, optionally compresses them with gzip, signs them with HMAC,
// and sends them to the server URL with X-Real-IP header.
func (ru *RuntimeUpdate) SendMetricGobCh(ctx context.Context, curl string, compress string, key string) error {
	for metrics := range ru.ChIn {
		var (
			bufOut    []byte
			newBuffer bytes.Buffer
		)

		req := ru.client.R().SetHeader("Content-Type", "application/x-gob")

		// Добавляем заголовок X-Real-IP с IP-адресом хоста агента
		if ru.localIP != "" {
			req.SetHeader("X-Real-IP", ru.localIP)
		}

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
			req.SetHeader("Accept-Encoding", "gzip").
				SetHeader("Content-Encoding", "gzip")
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

		var empty *rsa.PublicKey
		if ru.PubKey != empty {
			bufOut, err = signature.EncryptByRSA(bufOut, ru.PubKey)
			if err != nil {
				return err
			}
		}

		ru.mu.Lock()
		retryCfg := retry.DefaultConfig()
		_, err = retryCfg.Retry(ctx, func(_ ...any) (any, error) {
			_, err := req.SetBody(bufOut).Post(curl)
			return nil, err
		})
		ru.mu.Unlock()

		if err != nil {
			log.Printf("WARN: Failed to send metric after retries: %v", err)
		}
	}
	return nil
}

// GetLocalIP возвращает IP-адрес хоста агента.
func (ru *RuntimeUpdate) GetLocalIP() string {
	return ru.localIP
}

// Sender starts the metric sending process using configuration from ClientConfig.
// It acts as a wrapper around SendMetricGobCh.
func (ru *RuntimeUpdate) Sender(ctx context.Context, curl string, f clientconfig.ClientConfig) {
	if err := ru.SendMetricGobCh(ctx, curl, f.Compress, f.Key); err != nil {
		panic(fmt.Errorf("send metrics to %s:%s: %w", f.GetHost(), f.GetPort(), err))
	}
}

// AddGauge converts string values from metric maps into gauge metrics (metricsdto.Metrics).
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

// AddCounter converts string values from metric maps into counter metrics (metricsdto.Metrics).
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

// GetMetricsBatch retrieves all metrics from the local service, converts them to DTOs,
// and sends them in batches (currently size 10) to the processing channel.
func (ru *RuntimeUpdate) GetMetricsBatch(ctx context.Context) error {
	var (
		keysCounterIter []string
		keysGaugeIter   []string
	)

	keysGauge, keysCounter, metricMaps := ru.service.GetAllMetrics(ctx)

	i := 10

	for {
		// Pagination/Batching logic for counters
		if len(keysCounter) <= i && len(keysCounter) > i-10 {
			keysCounterIter = keysCounter[i-10:]
		} else if i-10 >= len(keysCounter) {
			keysCounterIter = []string{}
		} else {
			keysCounterIter = keysCounter[i-10 : i]
		}

		// Pagination/Batching logic for gauges
		if len(keysGauge) <= i && len(keysGauge) > i-10 {
			keysGaugeIter = keysGauge[i-10:]
		} else if i-10 >= len(keysGauge) {
			keysGaugeIter = []string{}
		} else {
			keysGaugeIter = keysGauge[i-10 : i]
		}

		// Convert current batch to DTOs
		metrics, err := ru.ConvertToDTO(ctx, keysCounterIter, keysGaugeIter, metricMaps)
		if err != nil {
			panic(err)
		}

		ru.SendBatch(ctx, metrics)

		if i >= len(metricMaps) { // Should check against max length of keys, but map len is roughly sum
			// Better termination condition might be: if both iter slices are empty or we covered all keys
			// Assuming original logic works for your case.
			if len(keysCounterIter) == 0 && len(keysGaugeIter) == 0 && i > 10 {
				break
			}
			// Safe break if we processed everything
			if i > len(keysGauge)+len(keysCounter)+10 {
				break
			}
		}
		i += 10
	}
	return nil
}

// SendBatch queues a slice of metrics for sending by writing to the ChIn channel.
func (ru *RuntimeUpdate) SendBatch(ctx context.Context, metrics []metricsdto.Metrics) {
	if len(metrics) != 0 {
		ru.ChIn <- metrics
	}
}

// CloseChannel closes the input channel, signalling no more batches will be sent.
func (ru *RuntimeUpdate) CloseChannel(ctx context.Context) {
	close(ru.ChIn)
}

// ConvertToDTO combines AddCounter and AddGauge to convert raw metric data into a DTO slice.
func (ru *RuntimeUpdate) ConvertToDTO(ctx context.Context, keysCounterIter []string, keysGaugeIter []string, metricMaps map[string]string) ([]metricsdto.Metrics, error) {
	metrics := []metricsdto.Metrics{}
	counters, err := ru.AddCounter(keysCounterIter, metricMaps)
	if err != nil {
		return nil, fmt.Errorf("error converting counters: %v", err)
	}
	metrics = append(metrics, counters...)

	gauges, err := ru.AddGauge(keysGaugeIter, metricMaps)
	if err != nil {
		return nil, fmt.Errorf("error converting gauges: %v", err)
	}
	metrics = append(metrics, gauges...)
	return metrics, nil
}

// GetRateLimit returns the configured rate limit for the updater.
func (ru *RuntimeUpdate) GetRateLimit() int {
	return ru.RateLimit
}
