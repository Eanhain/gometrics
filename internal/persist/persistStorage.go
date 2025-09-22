package persist

import (
	"bufio"
	"encoding/json"
	metricsdto "gometrics/internal/api/metricsdto"
	"os"
)

type PersistStorage struct {
	filePath string
}

func NewPersistStorage(filePath string) *PersistStorage {
	pstorage := &PersistStorage{
		filePath: filePath,
	}
	return pstorage
}

func (pstorage *PersistStorage) GaugeInsert(key string, value float64) error {
	metric := metricsdto.Metrics{
		ID:    key,
		MType: "gauge",
		Value: &value}
	pstorage.writeLogs(metric)
	err := pstorage.writeLogs(metric)
	return err
}

func (pstorage *PersistStorage) CounterInsert(key string, value int) error {
	value64 := int64(value)
	metric := metricsdto.Metrics{
		ID:    key,
		MType: "counter",
		Delta: &value64}
	err := pstorage.writeLogs(metric)
	return err
}

func (pstorage *PersistStorage) writeLogs(logs metricsdto.Metrics) error {
	flags := os.O_RDWR | os.O_CREATE | os.O_APPEND

	file, err := os.OpenFile(pstorage.filePath, flags, 0644)

	if err != nil {
		return err
	}

	defer file.Close()

	data, err := json.Marshal(logs)
	if err != nil {
		return err
	}

	_, err = file.Write(data)

	return err
}

func (pstorage *PersistStorage) ImportLogs() ([]metricsdto.Metrics, error) {

	flags := os.O_RDONLY

	file, err := os.OpenFile(pstorage.filePath, flags, 0644)

	if err != nil {
		return nil, err
	}

	reader := bufio.NewScanner(file)

	var token []metricsdto.Metrics
	var tmp metricsdto.Metrics

	for reader.Scan() {
		jBytes := reader.Bytes()
		err := json.Unmarshal(jBytes, &tmp)
		if err != nil {
			return nil, err
		}
		token = append(token, tmp)
	}

	return token, err
}
