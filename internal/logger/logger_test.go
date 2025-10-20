package logger

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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newTestLogger(t *testing.T) (*LoggerRequest, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	encoderCfg := zap.NewProductionEncoderConfig()
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(&buf), zapcore.DebugLevel)
	baseLogger := zap.New(core)
	t.Cleanup(func() { _ = baseLogger.Sync() })

	return &LoggerRequest{SugaredLogger: baseLogger.Sugar()}, &buf
}

func gzipCompress(t *testing.T, data string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestWithLoggingRecordsRequest(t *testing.T) {
	l, buf := newTestLogger(t)

	handlerCalled := false
	handler := l.WithLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "test-body", string(body))

		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte("hello"))
		require.NoError(t, err)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test-body"))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.True(t, handlerCalled)
	require.Equal(t, http.StatusCreated, rr.Code)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "uri /test")
	assert.Contains(t, logOutput, "method POST")
	assert.Contains(t, logOutput, "status 201")
	assert.Contains(t, logOutput, "size 5")
	assert.Contains(t, logOutput, "body test-body")
}

func TestWithLoggingDecompressesBody(t *testing.T) {
	l, buf := newTestLogger(t)

	handler := l.WithLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))

	payload := gzipCompress(t, "zip-test")
	req := httptest.NewRequest(http.MethodPost, "/gzip", bytes.NewReader(payload))
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "body zip-test")
}
