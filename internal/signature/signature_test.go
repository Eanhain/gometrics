package signature

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to generate valid HMAC signature
func generateSignature(data []byte, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestSignatureCheck(t *testing.T) {
	secret := []byte("super-secret-key")
	body := []byte("some payload data")
	validSign := generateSignature(body, secret)

	tests := []struct {
		name      string
		body      []byte
		header    string
		secret    []byte
		wantValid bool
	}{
		{
			name:      "Valid signature",
			body:      body,
			header:    validSign,
			secret:    secret,
			wantValid: true,
		},
		{
			name:      "Invalid signature (data mismatch)",
			body:      []byte("other data"),
			header:    validSign,
			secret:    secret,
			wantValid: false,
		},
		{
			name:      "Invalid signature (key mismatch)",
			body:      body,
			header:    validSign,
			secret:    []byte("wrong-key"),
			wantValid: false,
		},
		{
			name:      "Malformed hex string",
			body:      body,
			header:    "not-a-hex-string",
			secret:    secret,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем запрос с телом
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(tt.body))

			// Выполняем проверку
			got := SignatureCheck(req, tt.secret, tt.header)
			assert.Equal(t, tt.wantValid, got)

			// Проверяем, что тело запроса было восстановлено (io.NopCloser)
			// и его можно прочитать снова
			restoredBody, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.body, restoredBody, "Body should be readable after check")
		})
	}
}

func TestResponseHashWriter(t *testing.T) {
	key := []byte("secret")
	rec := httptest.NewRecorder()
	rw := NewResponseHashWriter(rec, key)

	// Тестируем Header()
	rw.Header().Set("X-Test", "True")
	assert.Equal(t, "True", rec.Header().Get("X-Test"))

	// Тестируем Write() - пишем в буфер
	data := []byte("response data")
	n, err := rw.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Проверяем, что пока ничего не записалось в реальный ResponseWriter
	assert.Equal(t, 0, rec.Body.Len())

	// Тестируем WriteHeader() - сохраняем код
	rw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, rw.rCode)

	// Тестируем Finalyze()
	n, err = rw.Finalyze()
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Теперь проверяем результат в рекордере
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, string(data), rec.Body.String())

	// Проверяем подпись ответа
	expectedSign := generateSignature(data, key)
	assert.Equal(t, expectedSign, rec.Header().Get("HashSHA256"))
}

func TestSignatureHandler(t *testing.T) {
	secretStr := "my-secret"
	handler := SignatureHandler(secretStr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	t.Run("No key configured (passthrough)", func(t *testing.T) {
		// Middleware с пустым ключом
		h := SignatureHandler("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("pass"))
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "pass", rec.Body.String())
	})

	t.Run("Request without header (allowed)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("data")))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// Ответ должен быть подписан, так как ключ задан
		assert.NotEmpty(t, rec.Header().Get("HashSHA256"))
	})

	t.Run("Request with 'none' header (allowed)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("data")))
		req.Header.Set("HashSHA256", "none") // Вариант "none"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Request with VALID signature", func(t *testing.T) {
		body := []byte("trusted data")
		sign := generateSignature(body, []byte(secretStr))

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("HashSHA256", sign)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())

		// Проверяем, что ответ тоже подписан корректно
		respSign := rec.Header().Get("HashSHA256")
		assert.NotEmpty(t, respSign)
		assert.Equal(t, generateSignature([]byte("OK"), []byte(secretStr)), respSign)
	})

	t.Run("Request with INVALID signature", func(t *testing.T) {
		body := []byte("malicious data")
		sign := generateSignature(body, []byte("wrong-secret")) // Подпись другим ключом

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("HashSHA256", sign)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "wrong key")
	})

	t.Run("Alternative header name 'Hash'", func(t *testing.T) {
		body := []byte("legacy header")
		sign := generateSignature(body, []byte(secretStr))

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("Hash", sign) // Используем старый заголовок
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
