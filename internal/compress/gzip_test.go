package compress

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func BenchmarkGzipWriter(b *testing.B) {
	log, _ := zap.NewDevelopment()
	logS := log.Sugar()
	jsonExample := strings.Repeat(`
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
		]`, 100)
	b.Run("Reader bench", func(b *testing.B) {

		jsonCompress, err := Compress([]byte(jsonExample))
		if err != nil {
			panic(err)
		}

		nextHandlerReader := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := io.ReadAll(r.Body)
			defer r.Body.Close()
			if err != nil {
				logS.Errorln(err)
			}

		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			recorder := httptest.NewRecorder()
			jsonBuffer := bytes.NewBuffer(jsonCompress)
			req := httptest.NewRequest("GET", "http://testing", jsonBuffer)
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Content-Encoding", "gzip")
			gzHandlerReader := GzipHandleReader(nextHandlerReader)
			gzHandlerReader.ServeHTTP(recorder, req)
		}

	})

	b.Run("Writer bench", func(b *testing.B) {
		nextHandlerWriter := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bodyStr := jsonExample
			_, err := w.Write([]byte(bodyStr))
			if err != nil {
				logS.Errorln(err)
			}

		})
		gzHandlerWriter := GzipHandleWriter(nextHandlerWriter)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "http://testing", nil)
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept-Encoding", "gzip")

			gzHandlerWriter.ServeHTTP(recorder, req)
		}
	})
}
