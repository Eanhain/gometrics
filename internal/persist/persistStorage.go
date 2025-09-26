package persist

import (
	"bufio"
	"encoding/json"
	metricsdto "gometrics/internal/api/metricsdto"
	"io"
	"log"
	"os"
	"path/filepath"
)

type PersistStorage struct {
	file       *os.File
	writer     *bufio.Writer
	reader     *bufio.Reader
	storeInter int
}

func NewPersistStorage(dirPath string, storeInter int) (*PersistStorage, error) {

	if dirPath == "agent" {
		return &PersistStorage{nil, nil, nil, -100}, nil
	}

	flags := os.O_RDWR | os.O_CREATE
	mode := os.FileMode(uint32(0755))
	err := os.MkdirAll(dirPath, mode)
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(dirPath, "Metrics.json")

	file, err := os.OpenFile(filePath, flags, mode)
	if err != nil {
		return nil, err
	}
	pstorage := &PersistStorage{
		file:       file,
		writer:     bufio.NewWriter(file),
		reader:     bufio.NewReader(file),
		storeInter: storeInter,
	}
	return pstorage, nil
}

func (pstorage *PersistStorage) FormattingLogs(gauge map[string]float64, counter map[string]int) error {
	var metrics []metricsdto.Metrics
	for gkey, gvalue := range gauge {
		metric := metricsdto.Metrics{
			ID:    gkey,
			MType: "gauge",
			Value: &gvalue}
		metrics = append(metrics, metric)
	}
	for ckey, cvalue := range counter {
		value64 := int64(cvalue)
		metric := metricsdto.Metrics{
			ID:    ckey,
			MType: "counter",
			Delta: &value64}
		metrics = append(metrics, metric)
	}
	metricsByte, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}
	if err := pstorage.file.Truncate(0); err != nil {
		return err
	}
	if _, err := pstorage.file.Seek(0, 0); err != nil {
		return err
	}
	pstorage.writer.Reset(pstorage.file)
	_, err = pstorage.writer.Write(metricsByte)
	if err != nil {
		return err
	}
	if pstorage.storeInter == 0 {
		err := pstorage.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

func (pstorage *PersistStorage) Close() error {
	return pstorage.file.Close()
}

func (pstorage *PersistStorage) WriteLogs(logs []metricsdto.Metrics) error {
	bytes, err := json.Marshal(logs)

	if err != nil {
		return err
	}

	if _, err := pstorage.writer.Write(bytes); err != nil {
		return err
	}

	if err := pstorage.writer.WriteByte('\n'); err != nil {
		return err
	}

	return err
}

func (pstorage *PersistStorage) ImportLogs() ([]metricsdto.Metrics, error) {

	var token []metricsdto.Metrics
	if pstorage.file == nil || pstorage.file.Name() == "agent" {
		log.Printf("WARN: persist storage disabled; file not configured (agent mode)")
		return []metricsdto.Metrics{}, nil
	}
	if _, err := pstorage.file.Seek(0, 0); err != nil {
		return []metricsdto.Metrics{}, err
	}
	pstorage.writer.Reset(pstorage.file)

	jBytes, err := io.ReadAll(pstorage.reader)

	if err != nil {
		return []metricsdto.Metrics{}, err
	}

	err = json.Unmarshal(jBytes, &token)
	if err != nil {
		return []metricsdto.Metrics{}, err
	}

	return token, nil
}

func (pstorage *PersistStorage) GetFile() *os.File {
	return pstorage.file
}

func (pstorage *PersistStorage) GetLoopTime() int {
	return pstorage.storeInter
}

func (pstorage *PersistStorage) Flush() error {

	return pstorage.writer.Flush()
}
