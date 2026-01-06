package compress

import (
	"bytes"
	"compress/gzip"
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
	// 1. Тест GzipHandleReader (Распаковка входящего тела)
	t.Run("GzipReader decompress request body", func(t *testing.T) {
		// Сжимаем данные "клиентом"
		compressedData, err := Compress([]byte(jsonExample))
		require.NoError(t, err)

		// Хендлер, который должен получить уже РАСПАКОВАННЫЕ данные
		mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			defer r.Body.Close()

			// Проверяем, что middleware действительно распаковал данные
			assert.JSONEq(t, jsonExample, string(body), "Body should match original JSON")
		})

		// Создаем запрос с заголовком Content-Encoding: gzip
		req := httptest.NewRequest("POST", "/", bytes.NewReader(compressedData))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Запускаем через middleware
		GzipHandleReader(mockHandler).ServeHTTP(w, req)
	})

	// 2. Тест GzipHandleWriter (Сжатие исходящего ответа)
	t.Run("GzipWriter compress response body", func(t *testing.T) {
		// Хендлер, который пишет обычный JSON
		mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(jsonExample))
		})

		// Создаем запрос с Accept-Encoding: gzip
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		// Запускаем через middleware
		GzipHandleWriter(mockHandler).ServeHTTP(w, req)

		// Проверяем заголовки
		resp := w.Result()

		assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

		// Читаем и распаковываем ответ "клиентом"
		gzReader, err := gzip.NewReader(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)
		defer gzReader.Close()

		decompressedBody, err := io.ReadAll(gzReader)
		require.NoError(t, err)

		// Проверяем содержимое
		assert.JSONEq(t, jsonExample, string(decompressedBody))
	})

	// 3. Тест без сжатия (если клиент не просит)
	t.Run("No gzip if not requested", func(t *testing.T) {
		mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello"))
		})

		req := httptest.NewRequest("GET", "/", nil)
		// НЕ ставим Accept-Encoding
		w := httptest.NewRecorder()

		GzipHandleWriter(mockHandler).ServeHTTP(w, req)

		resp := w.Result()
		assert.Empty(t, resp.Header.Get("Content-Encoding"))
		assert.Equal(t, "hello", w.Body.String())
		resp.Body.Close()
	})
}

// --- BENCHMARKS ---

func BenchmarkGzip(b *testing.B) {
	// Подготовка данных (большой JSON)
	bigJSON := strings.Repeat(jsonExample, 10)
	compressedData, _ := Compress([]byte(bigJSON))

	b.Run("GzipReader_Middleware", func(b *testing.B) {
		// Хендлер-заглушка, просто читает Body
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		})

		handler := GzipHandleReader(nextHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// В бенчмарке создание запроса тоже занимает время,
			// но выносить его полностью сложно, т.к. Body вычитывается.
			// bytes.NewReader - дешевая операция.
			req := httptest.NewRequest("POST", "/", bytes.NewReader(compressedData))
			req.Header.Set("Content-Encoding", "gzip")

			// Используем легкий Recorder, чтобы не тратить память на запись ответа
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
		}
	})

	b.Run("GzipWriter_Middleware", func(b *testing.B) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
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
