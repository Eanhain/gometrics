package handlers

import (
	"gometrics/internal/storage"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_handlerService_UpdateMetrics(t *testing.T) {
	// type args struct {
	// 	res http.ResponseWriter
	// 	req *http.Request
	// }
	tests := []struct {
		name   string
		h      *handlerService
		status int
		key    string
		value  float64
		url    string
		// args args
	}{
		{
			name:   "ok insert",
			h:      NewHandlerService(storage.NewMemStorage()),
			status: 200,
			key:    "cpu",
			value:  15,
			url:    "/update/gauge/cpu/15",
		},
		{
			name:   "bad select insert",
			h:      NewHandlerService(storage.NewMemStorage()),
			status: 400,
			key:    "cpu",
			value:  0,
			url:    "/select/gauge/cpu/15",
		},
		{
			name:   "guge insert",
			h:      NewHandlerService(storage.NewMemStorage()),
			status: 400,
			key:    "cpu",
			value:  0,
			url:    "/update/guge/cpu/15",
		},
		{
			name:   "value only insert",
			h:      NewHandlerService(storage.NewMemStorage()),
			status: 404,
			key:    "cpu",
			value:  0,
			url:    "/update/gauge/cpu/",
		},
		{
			name:   "overhead ins",
			h:      NewHandlerService(storage.NewMemStorage()),
			status: 400,
			key:    "cpu",
			value:  0,
			url:    "/update/gauge/cpu/15/16/17",
		},
		{
			name:   "ok insert 0",
			h:      NewHandlerService(storage.NewMemStorage()),
			status: 200,
			key:    "cpu",
			value:  0,
			url:    "/update/gauge/cpu/0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tt.url, nil)
			tt.h.UpdateMetrics(w, req)
			res := w.Result()
			assert.Equal(t, tt.status, res.StatusCode)
			assert.Equal(t, tt.value, tt.h.storage.GetGauge(tt.key))
			defer res.Body.Close()
		})
	}
}
