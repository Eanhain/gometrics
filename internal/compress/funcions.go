// Package compress provides utilities for gzip compression and decompression.
// It includes helper functions for byte slices and HTTP middleware for
// transparent request/response compression.
package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
)

// Compress compresses a byte slice using gzip.
// It returns the compressed bytes or an error if the compression fails.
func Compress(data []byte) ([]byte, error) {
	var b bytes.Buffer
	// Create a gzip.Writer writing to the buffer.
	w := gzip.NewWriter(&b)

	// Write data to the gzip writer.
	_, err := w.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed write data to compress temporary buffer: %v", err)
	}

	// Close the writer to flush any remaining data to the buffer.
	// This is crucial; otherwise, the compressed data might be incomplete.
	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("failed compress data: %v", err)
	}

	// Return the compressed bytes.
	return b.Bytes(), nil
}

// Decompress decompresses a gzip-compressed byte slice.
// It returns the original uncompressed bytes or an error if decompression fails.
func Decompress(data []byte) ([]byte, error) {
	// Create a gzip.Reader reading from the byte slice.
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed create reader: %v", err)
	}
	defer r.Close()

	var b bytes.Buffer
	// Read decompressed data into the buffer.
	_, err = b.ReadFrom(r)
	if err != nil {
		return nil, fmt.Errorf("failed decompress data: %v", err)
	}

	return b.Bytes(), nil
}
