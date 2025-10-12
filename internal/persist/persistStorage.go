package persist

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	metricsdto "gometrics/internal/api/metricsdto"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type PersistStorage struct {
	file       *os.File
	writer     *bufio.Writer
	reader     *bufio.Reader
	storeInter int
	mu         sync.Mutex
	pending    []byte
}

func NewPersistStorage(dirPath string, storeInter int) (*PersistStorage, error) {

	if dirPath == "agent" {
		return &PersistStorage{storeInter: -100}, nil
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

func (pstorage *PersistStorage) FormattingLogs(ctx context.Context, gauge map[string]float64, counter map[string]int) error {
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

	pstorage.mu.Lock()
	defer pstorage.mu.Unlock()
	pstorage.pending = metricsByte

	if pstorage.storeInter != 0 {
		return nil
	}

	return pstorage.writeSnapshotLocked()
}

func (pstorage *PersistStorage) Close() error {
	if pstorage == nil {
		return nil
	}

	errFlush := pstorage.Flush()
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

func (pstorage *PersistStorage) ImportLogs(ctx context.Context) ([]metricsdto.Metrics, error) {

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
		return []metricsdto.Metrics{}, fmt.Errorf("can't read metrics file: %w", err)
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
		return []metricsdto.Metrics{}, fmt.Errorf("decode metrics file: %w\npayload: %q", err, out)
	}

	return token, nil
}

func (pstorage *PersistStorage) Ping(ctx context.Context) error {
	_, err := pstorage.file.Stat()

	return fmt.Errorf("file not found\n%v", err)
}

func (pstorage *PersistStorage) GetLoopTime() int {
	return pstorage.storeInter
}

func (pstorage *PersistStorage) Flush() error {
	pstorage.mu.Lock()
	defer pstorage.mu.Unlock()

	if pstorage.writer == nil || pstorage.file == nil {
		return nil
	}

	return pstorage.writeSnapshotLocked()
}

func (pstorage *PersistStorage) writeSnapshotLocked() error {
	if pstorage.file == nil {
		return nil
	}

	if err := pstorage.file.Truncate(0); err != nil {
		return err
	}
	if _, err := pstorage.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	pstorage.writer.Reset(pstorage.file)

	if len(pstorage.pending) > 0 {
		if _, err := pstorage.writer.Write(pstorage.pending); err != nil {
			return err
		}
	}

	if err := pstorage.writer.Flush(); err != nil {
		return err
	}

	return pstorage.file.Sync()
}
