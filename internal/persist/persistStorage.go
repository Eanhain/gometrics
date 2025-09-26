package persist

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
		value := gvalue
		metric := metricsdto.Metrics{
			ID:    gkey,
			MType: "gauge",
			Value: &value}
		metrics = append(metrics, metric)
	}
	for ckey, cvalue := range counter {
		delta := int64(cvalue)
		metric := metricsdto.Metrics{
			ID:    ckey,
			MType: "counter",
			Delta: &delta}
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
	if pstorage == nil {
		return nil
	}

	var errFlush error
	if pstorage.writer != nil {
		errFlush = pstorage.writer.Flush()
	}

	if pstorage.file == nil {
		return errFlush
	}

	errClose := pstorage.file.Close()
	if errFlush != nil || errClose != nil {
		return errors.Join(errFlush, errClose)
	}

	return nil
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
	if pstorage.file == nil {
		log.Printf("WARN: persist storage disabled; file not configured (agent mode)")
		return []metricsdto.Metrics{}, nil
	}
	if _, err := pstorage.file.Seek(0, io.SeekStart); err != nil {
		return []metricsdto.Metrics{}, err
	}
	var reader io.Reader = pstorage.file
	if pstorage.reader != nil {
		pstorage.reader.Reset(pstorage.file)
		reader = pstorage.reader
	}

	jBytes, err := io.ReadAll(reader)
	if err != nil {
		return []metricsdto.Metrics{}, fmt.Errorf("read metrics file: %w", err)
	}

	if len(bytes.TrimSpace(jBytes)) == 0 {
		log.Printf("INFO: persist storage is empty")
		return []metricsdto.Metrics{}, nil
	}

	if err := json.Unmarshal(jBytes, &token); err != nil {
		out := string(jBytes)
		if len(out) > 256 {
			out = out[:256]
		}
		return []metricsdto.Metrics{}, fmt.Errorf("ERROR: decode metrics file: %w\npayload: %q", err, out)
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
