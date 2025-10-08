package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	dto "gometrics/internal/api/metricsdto"
	"gometrics/internal/service"
	"gometrics/internal/storage"

	"github.com/go-chi/chi/v5"
	easyjson "github.com/mailru/easyjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPersistStorage struct{}

func (s *stubPersistStorage) GaugeInsert(string, float64) error { return nil }
func (s *stubPersistStorage) CounterInsert(string, int) error   { return nil }
func (s *stubPersistStorage) FormattingLogs(map[string]float64, map[string]int) error {
	return nil
}
func (s *stubPersistStorage) ImportLogs() ([]dto.Metrics, error) {
	return nil, nil
}
func (s *stubPersistStorage) GetFile() *os.File { return nil }
func (s *stubPersistStorage) GetLoopTime() int  { return 0 }
func (s *stubPersistStorage) Close() error      { return nil }
func (s *stubPersistStorage) Flush() error      { return nil }

type stubDBStorage struct{}

func (s *stubDBStorage) PingDB(context.Context) error { return nil }
func (s *stubDBStorage) Close() error                 { return nil }

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
			h := NewHandlerService(service.NewService(storage.NewMemStorage(), &stubPersistStorage{}, &stubDBStorage{}), chi.NewMux())
			h.CreateHandlers()
			ts := httptest.NewServer(h.GetRouter())
			defer ts.Close()
			resp, _ := testRequest(t, ts, tt.method, tt.url)
			defer resp.Body.Close()
			assert.Equal(t, tt.status, resp.StatusCode)
		})
	}
}

func testRequestJSON(t *testing.T, ts *httptest.Server, method,
	path string, body []byte) (*http.Response, dto.Metrics) {
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
	require.NoError(t, err)

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out dto.Metrics

	require.NoError(t, easyjson.Unmarshal(respBody, &out))

	return resp, out
}

func Test_handlerService_JsonInsert(t *testing.T) {
	var f1, f2, f3 = 1.1, 2.2, 3.3
	var i1, i2, i3 int64 = 1, 2, 3
	tests := []struct {
		name   string
		method string
		status int
		value  []dto.Metrics
		url    string
	}{
		{
			name:   "ok json insert",
			method: "POST",
			status: 200,
			value: []dto.Metrics{
				{
					ID:    "g1",
					MType: "gauge",
					Value: &f1,
				},
				{
					ID:    "g2",
					MType: "gauge",
					Value: &f2,
				},
				{
					ID:    "g3",
					MType: "gauge",
					Value: &f3,
				},
				{
					ID:    "c1",
					MType: "counter",
					Delta: &i1,
				},
				{
					ID:    "c2",
					MType: "counter",
					Delta: &i2,
				},
				{
					ID:    "c3",
					MType: "counter",
					Delta: &i3,
				},
			},
			url: "/update/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlerService(service.NewService(storage.NewMemStorage(), &stubPersistStorage{}, &stubDBStorage{}), chi.NewMux())
			h.CreateHandlers()
			ts := httptest.NewServer(h.GetRouter())
			defer ts.Close()
			for _, obj := range tt.value {
				b, err := easyjson.Marshal(obj)
				require.NoError(t, err)
				resp, body := testRequestJSON(t, ts, tt.method, tt.url, b)
				defer resp.Body.Close()
				assert.Equal(t, body, obj)
				assert.Equal(t, tt.status, resp.StatusCode)
			}
		})
	}
}

func Test_handlerService_JsonGet(t *testing.T) {
	var f1, f2, f3 = 1.1, 2.2, 3.3
	var i1, i2, i3 int64 = 1, 2, 3
	tests := []struct {
		name   string
		method string
		status int
		req    []dto.Metrics
		expect []dto.Metrics
		url    string
	}{
		{
			name:   "ok json Get",
			method: "POST",
			status: 200,
			req: []dto.Metrics{
				{
					ID:    "g1",
					MType: "gauge",
				},
				{
					ID:    "g2",
					MType: "gauge",
				},
				{
					ID:    "g3",
					MType: "gauge",
				},
				{
					ID:    "c1",
					MType: "counter",
				},
				{
					ID:    "c2",
					MType: "counter",
				},
				{
					ID:    "c3",
					MType: "counter",
				},
			},
			expect: []dto.Metrics{
				{
					ID:    "g1",
					MType: "gauge",
					Value: &f1,
				},
				{
					ID:    "g2",
					MType: "gauge",
					Value: &f2,
				},
				{
					ID:    "g3",
					MType: "gauge",
					Value: &f3,
				},
				{
					ID:    "c1",
					MType: "counter",
					Delta: &i1,
				},
				{
					ID:    "c2",
					MType: "counter",
					Delta: &i2,
				},
				{
					ID:    "c3",
					MType: "counter",
					Delta: &i3,
				},
			},
			url: "/value/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlerService(service.NewService(storage.NewMemStorage(), &stubPersistStorage{}, &stubDBStorage{}), chi.NewMux())
			h.CreateHandlers()
			ts := httptest.NewServer(h.GetRouter())
			defer ts.Close()
			for _, obj := range tt.expect {
				b, err := easyjson.Marshal(obj)
				require.NoError(t, err)
				resp, _ := testRequestJSON(t, ts, tt.method, "/update/", b)
				defer resp.Body.Close()
			}
			for it, obj := range tt.req {
				b, err := easyjson.Marshal(obj)
				require.NoError(t, err)
				resp, body := testRequestJSON(t, ts, tt.method, tt.url, b)
				defer resp.Body.Close()
				assert.Equal(t, tt.expect[it], body)
				assert.Equal(t, tt.status, resp.StatusCode)
			}
		})
	}
}
