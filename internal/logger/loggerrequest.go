package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type LoggerRequest struct {
	*zap.SugaredLogger
}

func (l *LoggerRequest) WithLogging(h http.HandlerFunc) http.HandlerFunc {
	logFn := func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		uri := r.RequestURI
		method := r.Method

		h.ServeHTTP(w, r)

		duration := time.Since(start)

		l.Infoln(
			"uri", uri,
			"method", method,
			"duration", duration,
		)

	}
	return http.HandlerFunc(logFn)
}

func CreateLoggerRequest() *LoggerRequest {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return &LoggerRequest{logger.Sugar()}
}
