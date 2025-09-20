package logger

import (
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

func CreateLoggerRequest() *LoggerRequest {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return &LoggerRequest{logger.Sugar()}
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
		h.ServeHTTP(&lw, r)

		duration := time.Since(start)
		l.Infoln(
			"uri", r.RequestURI,
			"method", r.Method,
			"status", lw.responseData.status,
			"duration", duration,
			"size", lw.responseData.size,
		)
	}
	return http.HandlerFunc(logFn)
}
