package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Тестовые данные вынесем, чтобы не дублировать
var jsonExample = `
		[
			{
				"id": "LastGC",
				"type": "gauge",
				"value": 1.767445179915476e+18
			},
			{
				"id": "HeapObjects",
				"type": "gauge",
				"value": 3100
			},
			{
				"id": "MSpanSys",
				"type": "gauge",
				"value": 179520
			},
			{
				"id": "Mallocs",
				"type": "gauge",
				"value": 404259
			},
			{
				"id": "GCCPUFraction",
				"type": "gauge",
				"value": 2.6727660656183833e-05
			},
			{
				"id": "freememory",
				"type": "gauge",
				"value": 1.4974976e+08
			},
			{
				"id": "MCacheInuse",
				"type": "gauge",
				"value": 16912
			},
			{
				"id": "StackInuse",
				"type": "gauge",
				"value": 1.048576e+06
			},
			{
				"id": "Lookups",
				"type": "gauge",
				"value": 0
			},
			{
				"id": "totalmemory",
				"type": "gauge",
				"value": 2.5769803776e+10
			},
			{
				"id": "BuckHashSys",
				"type": "gauge",
				"value": 4547
			},
			{
				"id": "NumForcedGC",
				"type": "gauge",
				"value": 0
			},
			{
				"id": "HeapReleased",
				"type": "gauge",
				"value": 6.479872e+06
			},
			{
				"id": "PauseTotalNs",
				"type": "gauge",
				"value": 4.0814339e+07
			},
			{
				"id": "HeapSys",
				"type": "gauge",
				"value": 1.1534336e+07
			},
			{
				"id": "HeapAlloc",
				"type": "gauge",
				"value": 2.703632e+06
			},
			{
				"id": "MCacheSys",
				"type": "gauge",
				"value": 31408
			},
			{
				"id": "GCSys",
				"type": "gauge",
				"value": 2.729744e+06
			},
			{
				"id": "MSpanInuse",
				"type": "gauge",
				"value": 147200
			},
			{
				"id": "NumGC",
				"type": "gauge",
				"value": 243
			},
			{
				"id": "Sys",
				"type": "gauge",
				"value": 1.7846288e+07
			},
			{
				"id": "HeapInuse",
				"type": "gauge",
				"value": 3.776512e+06
			},
			{
				"id": "OtherSys",
				"type": "gauge",
				"value": 2.318157e+06
			},
			{
				"id": "RandomValue",
				"type": "gauge",
				"value": 0.8340260237431216
			},
			{
				"id": "StackSys",
				"type": "gauge",
				"value": 1.048576e+06
			},
			{
				"id": "NextGC",
				"type": "gauge",
				"value": 4.194304e+06
			},
			{
				"id": "Alloc",
				"type": "gauge",
				"value": 2.703632e+06
			},
			{
				"id": "HeapIdle",
				"type": "gauge",
				"value": 7.757824e+06
			},
			{
				"id": "cpuutilization1",
				"type": "gauge",
				"value": 14.762931035519141
			},
			{
				"id": "Frees",
				"type": "gauge",
				"value": 401159
			},
			{
				"id": "TotalAlloc",
				"type": "gauge",
				"value": 6.60303824e+08
			},
			{
				"id": "PollCount",
				"type": "counter",
				"delta": 23204
			}
		]`

// --- UNIT TESTS ---

func TestGzipMiddleware(t *testing.T) {
	// 1. GzipHandleReader (Decompress incoming request body)
	t.Run("GzipReader decompress request body", func(t *testing.T) {
		compressedData, err := Compress([]byte(jsonExample))
		require.NoError(t, err)

		mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			defer r.Body.Close()

			assert.JSONEq(t, jsonExample, string(body), "Body should match original JSON")
		})

		req := httptest.NewRequest("POST", "/", bytes.NewReader(compressedData))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		GzipHandleReader(mockHandler).ServeHTTP(w, req)
	})

	// 2. GzipHandleWriter (Compress outgoing response body)
	t.Run("GzipWriter compress response body", func(t *testing.T) {
		mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(jsonExample))
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()

		GzipHandleWriter(mockHandler).ServeHTTP(w, req)

		resp := w.Result()
		assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

		gzReader, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)
		defer resp.Body.Close()
		defer gzReader.Close()

		decompressedBody, err := io.ReadAll(gzReader)
		require.NoError(t, err)

		assert.JSONEq(t, jsonExample, string(decompressedBody))
	})

	// 3. No gzip if not requested
	t.Run("No gzip if not requested", func(t *testing.T) {
		mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello"))
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		GzipHandleWriter(mockHandler).ServeHTTP(w, req)

		resp := w.Result()
		assert.Empty(t, resp.Header.Get("Content-Encoding"))
		assert.Equal(t, "hello", w.Body.String())
		resp.Body.Close()
	})
}

func TestCompressDecompress(t *testing.T) {
	data := []byte("Hello, World! Repeated data compresses well. Repeated data compresses well.")

	compressed, err := Compress(data)
	require.NoError(t, err)
	assert.NotEqual(t, data, compressed)

	decompressed, err := Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

// --- BENCHMARKS ---

func BenchmarkGzip(b *testing.B) {
	bigJSON := strings.Repeat(jsonExample, 100)
	compressedData, _ := Compress([]byte(bigJSON))

	b.Run("GzipReader_Middleware", func(b *testing.B) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		})
		handler := GzipHandleReader(nextHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("POST", "/", bytes.NewReader(compressedData))
			req.Header.Set("Content-Encoding", "gzip")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})

	b.Run("GzipWriter_Middleware", func(b *testing.B) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(bigJSON))
		})
		handler := GzipHandleWriter(nextHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})
}

// --- EXAMPLES ---

// ExampleCompress demonstrates how to compress a byte slice.
func ExampleCompress() {
	data := []byte("Hello, Gzip!")
	compressed, err := Compress(data)
	if err != nil {
		fmt.Println("Compression error:", err)
		return
	}
	fmt.Printf("Original size: %d, Compressed size: %d\n", len(data), len(compressed))
	// Output isn't stable for size due to gzip headers/platform, so we don't assert it here
}

// ExampleDecompress demonstrates how to decompress a byte slice.
func ExampleDecompress() {
	// Let's assume we have this compressed data
	data := []byte("Hello, Gzip!")
	compressed, _ := Compress(data)

	decompressed, err := Decompress(compressed)
	if err != nil {
		fmt.Println("Decompression error:", err)
		return
	}
	fmt.Println(string(decompressed))

	// Output:
	// Hello, Gzip!
}
