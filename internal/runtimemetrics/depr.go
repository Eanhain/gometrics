package runtimemetrics

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	metricsdto "gometrics/internal/api/metricsdto"
	myCompress "gometrics/internal/compress"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/mailru/easyjson"
)

func (ru *RuntimeUpdate) SendMetricsGob(ctx context.Context, ticker *time.Ticker, host string, port string, compress string, key string) error {
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
		keysGauge, keysCounter, metricMaps := ru.service.GetAllMetrics(ctx)
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

func (ru *RuntimeUpdate) SendMetrics(ctx context.Context, ticker *time.Ticker, host string, port string, compress string) error {
	var bufOut []byte
	curl := fmt.Sprintf("http://%v%v/update/", host, port)
	select {
	case <-ticker.C:
		ru.mu.RLock()
		keysGauge, keysCounter, metricMaps := ru.service.GetAllMetrics(ctx)
		ru.mu.RUnlock()

		req := ru.client.R().
			SetHeader("Content-Type", "application/json")

		for _, key := range keysGauge {

			valueString := metricMaps[key]
			valueFloat, err := strconv.ParseFloat(valueString, 64)

			if err != nil {
				return err
			}
			metrics := metricsdto.Metrics{ID: key, MType: metricsdto.MetricTypeGauge, Value: &valueFloat}
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
			metrics := metricsdto.Metrics{ID: key, MType: metricsdto.MetricTypeCounter, Delta: &int64Value}
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
