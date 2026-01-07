// Package persist implements file-based storage for metrics persistence.
// It supports saving metrics to a JSON file and restoring them, with support
// for both synchronous (immediate) and buffered flushing strategies.
package persist

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	metricsdto "gometrics/internal/api/metricsdto"
)

// PersistStorage handles reading and writing metrics to a local file.
// It is concurrent-safe and manages file I/O operations.
type PersistStorage struct {
	file       *os.File
	writer     *bufio.Writer
	reader     *bufio.Reader
	storeInter int // storeInter defines the flush interval (0 for sync writes, >0 for manual flush).
	mu         sync.Mutex
	pending    []byte // pending holds serialized metrics waiting to be flushed.
}

// NewPersistStorage initializes a new storage engine backed by a file.
//
// Arguments:
//   - dirPath: The directory where "Metrics.json" will be created. If set to "agent", storage runs in no-op mode.
//   - storeInter: The interval settings. If 0, every write is immediately synced to disk.
//
// Returns an error if the directory cannot be created or the file cannot be opened.
func NewPersistStorage(dirPath string, storeInter int) (*PersistStorage, error) {
	// "agent" is a special value to disable persistent storage logic for agent-side usage?
	// Based on original logic:
	if dirPath == "agent" {
		return &PersistStorage{storeInter: -100}, nil
	}

	flags := os.O_RDWR | os.O_CREATE
	mode := os.FileMode(uint32(0755))

	// Ensure directory exists
	err := os.MkdirAll(dirPath, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	filePath := filepath.Join(dirPath, "Metrics.json")

	file, err := os.OpenFile(filePath, flags, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage file: %w", err)
	}

	pstorage := &PersistStorage{
		file:       file,
		writer:     bufio.NewWriter(file),
		reader:     bufio.NewReader(file),
		storeInter: storeInter,
	}
	return pstorage, nil
}

// FormattingLogs converts in-memory maps of gauges and counters to a slice of Metrics,
// serializes them to JSON, and persists them to the file.
//
// If storeInter is 0, the data is flushed to disk immediately.
// Otherwise, it is stored in the 'pending' buffer and must be explicitly flushed later.
func (pstorage *PersistStorage) FormattingLogs(ctx context.Context, gauge map[string]float64, counter map[string]int) error {
	var metrics []metricsdto.Metrics
	for gkey, gvalue := range gauge {
		value := gvalue
		metric := metricsdto.Metrics{
			ID:    gkey,
			MType: metricsdto.MetricTypeGauge,
			Value: &value}
		metrics = append(metrics, metric)
	}
	for ckey, cvalue := range counter {
		delta := int64(cvalue)
		metric := metricsdto.Metrics{
			ID:    ckey,
			MType: metricsdto.MetricTypeCounter,
			Delta: &delta}
		metrics = append(metrics, metric)
	}

	metricsByte, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	pstorage.mu.Lock()
	defer pstorage.mu.Unlock()

	pstorage.pending = metricsByte

	// If interval is 0, we treat it as "sync mode" -> write immediately
	if pstorage.storeInter != 0 {
		return nil
	}

	return pstorage.writeSnapshotLocked()
}

// Close ensures all pending data is flushed to disk and closes the underlying file handle.
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

// WriteLogs appends a slice of metrics directly to the writer (append-only log style).
// Note: This seems different from FormattingLogs which overwrites the file.
// Check usage consistency in your application.
func (pstorage *PersistStorage) WriteLogs(logs []metricsdto.Metrics) error {
	bytes, err := json.Marshal(logs)
	if err != nil {
		return err
	}

	// Lock potentially needed if writer is shared?
	// Assuming external locking or single-threaded access for this method based on original code.
	if _, err := pstorage.writer.Write(bytes); err != nil {
		return err
	}

	if err := pstorage.writer.WriteByte('\n'); err != nil {
		return err
	}

	return nil // err was nil here in original
}

// ImportLogs reads the entire file content and deserializes it into a slice of Metrics.
// Used for restoring state on startup.
func (pstorage *PersistStorage) ImportLogs(ctx context.Context) ([]metricsdto.Metrics, error) {
	var token []metricsdto.Metrics

	if pstorage.file == nil {
		log.Printf("WARN: persist storage disabled; file not configured (agent mode)")
		return []metricsdto.Metrics{}, nil
	}

	pstorage.mu.Lock()
	defer pstorage.mu.Unlock()

	// Seek to start to read full file
	if _, err := pstorage.file.Seek(0, io.SeekStart); err != nil {
		return []metricsdto.Metrics{}, fmt.Errorf("seek error: %w", err)
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
		// Truncate output for logging safety
		out := string(jBytes)
		if len(out) > 256 {
			out = out[:256] + "..."
		}
		return []metricsdto.Metrics{}, fmt.Errorf("decode metrics file: %w\npayload: %q", err, out)
	}

	return token, nil
}

// Ping checks if the storage file is accessible.
func (pstorage *PersistStorage) Ping(ctx context.Context) error {
	if pstorage.file == nil {
		return fmt.Errorf("storage not initialized")
	}
	_, err := pstorage.file.Stat()
	if err != nil {
		return fmt.Errorf("file not found or inaccessible: %w", err)
	}
	return nil
}

// GetLoopTime returns the configured storage interval.
func (pstorage *PersistStorage) GetLoopTime() int {
	return pstorage.storeInter
}

// Flush writes any pending in-memory metrics data to the underlying file
// and syncs the filesystem.
func (pstorage *PersistStorage) Flush() error {
	pstorage.mu.Lock()
	defer pstorage.mu.Unlock()

	if pstorage.writer == nil || pstorage.file == nil {
		return nil
	}

	return pstorage.writeSnapshotLocked()
}

// writeSnapshotLocked performs the actual file write operations (Truncate -> Seek -> Write -> Flush -> Sync).
// It must be called with pstorage.mu held.
func (pstorage *PersistStorage) writeSnapshotLocked() error {
	if pstorage.file == nil {
		return nil
	}

	// Overwrite strategy: Truncate file and write fresh content
	if err := pstorage.file.Truncate(0); err != nil {
		return fmt.Errorf("truncate failed: %w", err)
	}
	if _, err := pstorage.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	pstorage.writer.Reset(pstorage.file)

	if len(pstorage.pending) > 0 {
		if _, err := pstorage.writer.Write(pstorage.pending); err != nil {
			return fmt.Errorf("write pending failed: %w", err)
		}
	}

	if err := pstorage.writer.Flush(); err != nil {
		return fmt.Errorf("flush writer failed: %w", err)
	}

	// Ensure data hits the disk
	return pstorage.file.Sync()
}
