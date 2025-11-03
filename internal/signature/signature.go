package signature

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
)

type ResponseHashWriter struct {
	inherit http.ResponseWriter
	mac     hash.Hash
	buffer  bytes.Buffer
	rCode   int
}

func NewResponseHashWriter(w http.ResponseWriter, key []byte) *ResponseHashWriter {
	return &ResponseHashWriter{
		inherit: w,
		mac:     hmac.New(sha256.New, key),
		buffer:  bytes.Buffer{},
		rCode:   http.StatusOK,
	}
}

func (rw *ResponseHashWriter) Header() http.Header  { return rw.inherit.Header() }
func (rw *ResponseHashWriter) WriteHeader(code int) { rw.rCode = code }
func (rw *ResponseHashWriter) Write(b []byte) (int, error) {
	return rw.buffer.Write(b)
}

func SignatureCheck(r *http.Request, secret []byte, header string) bool {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(payload))

	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := mac.Sum(nil)

	got, err := hex.DecodeString(header)
	return err == nil && hmac.Equal(got, expected)
}

func (rw *ResponseHashWriter) Finalyze() (int, error) {
	if _, err := rw.mac.Write(rw.buffer.Bytes()); err != nil {
		return 0, fmt.Errorf("cannot parse buffer for hmac %v", err)
	}
	rw.Header().Set("HashSHA256", hex.EncodeToString(rw.mac.Sum(nil)))
	rw.inherit.WriteHeader(rw.rCode)
	return rw.inherit.Write(rw.buffer.Bytes())
}

func SignatureHandler(secret string) func(http.Handler) http.Handler {
	key := []byte(secret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(key) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			reqHeader := strings.TrimSpace(r.Header.Get("HashSHA256"))
			if reqHeader == "" {
				reqHeader = strings.TrimSpace(r.Header.Get("Hash"))
			}

			if reqHeader != "" && !strings.EqualFold(reqHeader, "none") {
				if !SignatureCheck(r, key, reqHeader) {
					http.Error(w, "wrong key", http.StatusBadRequest)
					return
				}
			}

			rw := NewResponseHashWriter(w, key)
			next.ServeHTTP(rw, r)
			if _, err := rw.Finalyze(); err != nil {
				http.Error(w, "cannot write buffer to response", http.StatusBadRequest)
			}

		})
	}
}
