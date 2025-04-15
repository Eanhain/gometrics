package handlers

import (
	"gometrics/internal/logger"
	"gometrics/internal/service"
	"gometrics/internal/storage"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRequest(t *testing.T, ts *httptest.Server, method,
	path string) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, nil)
	require.NoError(t, err)

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(respBody)
}

func Test_handlerService_CreateHandlers(t *testing.T) {
	tests := []struct {
		name   string
		method string
		status int
		key    string
		value  float64
		url    string
	}{
		{
			name:   "ok insert",
			method: "POST",
			status: 200,
			key:    "cpu",
			value:  15,
			url:    "/update/gauge/cpu/15",
		},
		{
			name:   "bad select insert",
			method: "POST",
			status: 404,
			key:    "cpu",
			value:  0,
			url:    "/select/gauge/cpu/15",
		},
		{
			name:   "guge insert",
			method: "POST",
			status: 400,
			key:    "cpu",
			value:  0,
			url:    "/update/guge/cpu/15",
		},
		{
			name:   "value only insert",
			method: "POST",
			status: 404,
			key:    "cpu",
			value:  0,
			url:    "/update/gauge/cpu/",
		},
		{
			name:   "overhead ins",
			method: "POST",
			status: 404,
			key:    "cpu",
			value:  0,
			url:    "/update/gauge/cpu/15/16/17",
		},
		{
			name:   "ok insert 0",
			method: "POST",
			status: 200,
			key:    "cpu",
			value:  0,
			url:    "/update/gauge/cpu/0",
		},
		{
			name:   "all metrics",
			method: "GET",
			status: 200,
			key:    "cpu",
			value:  0,
			url:    "/",
		},
		{
			name:   "get not found metrics",
			method: "GET",
			status: 404,
			key:    "cpu",
			value:  0,
			url:    "/value/test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlerService(service.NewService(storage.NewMemStorage()), logger.CreateLoggerRequest())
			h.CreateHandlers()
			ts := httptest.NewServer(h.GetRouter())
			defer ts.Close()
			resp, _ := testRequest(t, ts, tt.method, tt.url)
			defer resp.Body.Close()
			assert.Equal(t, tt.status, resp.StatusCode)
		})
	}
}
