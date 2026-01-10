package logger

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type (
	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}

	responseData struct {
		status int
		size   int
	}

	LoggerRequest struct {
		*zap.SugaredLogger
	}
)

func CreateLoggerRequest() (*LoggerRequest, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, fmt.Errorf("create zap logger: %w", err)
	}
	return &LoggerRequest{logger.Sugar()}, nil
}

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	if statusCode != 200 {
		r.ResponseWriter.WriteHeader(statusCode)
	}
	r.responseData.status = statusCode
}

func (l *LoggerRequest) WithLogging(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}
		var bufTMP bytes.Buffer
		origBody := r.Body
		r.Body = io.NopCloser(io.TeeReader(r.Body, &bufTMP))
		defer origBody.Close()
		h.ServeHTTP(&lw, r)

		var buf []byte
		// var err error

		buf = bufTMP.Bytes()

		// if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		// 	buf, err = compress.Decompress(bufTMP.Bytes())
		// 	if err != nil {
		// 		l.Errorw("gzip decompress failed", "err", err)
		// 	}

		// } else {

		// }
		duration := time.Since(start)
		l.Infoln(
			"uri", r.RequestURI,
			"method", r.Method,
			"status", lw.responseData.status,
			"duration", duration,
			"size", lw.responseData.size,
			"body", string(buf),
		)
	}
	return http.HandlerFunc(logFn)
}
