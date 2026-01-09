package retry

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock функция для тестирования вызовов
type MockAction struct {
	mock.Mock
}

func (m *MockAction) Execute(args ...any) (any, error) {
	callArgs := m.Called(args...)
	return callArgs.Get(0), callArgs.Error(1)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 4, cfg.Attempts)
	assert.Len(t, cfg.Delays, 3)
	assert.NotNil(t, cfg.ShouldRetry)
}

func TestRetry_SuccessOnFirstTry(t *testing.T) {
	cfg := DefaultConfig()
	mockAction := new(MockAction)

	// Ожидаем один вызов, который вернет "success" и nil ошибку
	mockAction.On("Execute", mock.Anything).Return("success", nil).Once()

	res, err := cfg.Retry(context.Background(), mockAction.Execute, "arg1")

	assert.NoError(t, err)
	assert.Equal(t, "success", res)
	mockAction.AssertExpectations(t)
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	// Настраиваем быстрые ретраи для теста
	cfg := RetryConfig{
		Attempts:    3,
		Delays:      []time.Duration{1 * time.Millisecond, 1 * time.Millisecond},
		ShouldRetry: func(err error) bool { return true },
	}
	mockAction := new(MockAction)

	// 1-й вызов: ошибка
	mockAction.On("Execute", mock.Anything).Return(nil, errors.New("fail 1")).Once()
	// 2-й вызов: ошибка
	mockAction.On("Execute", mock.Anything).Return(nil, errors.New("fail 2")).Once()
	// 3-й вызов: успех
	mockAction.On("Execute", mock.Anything).Return("finally success", nil).Once()

	res, err := cfg.Retry(context.Background(), mockAction.Execute)

	assert.NoError(t, err)
	assert.Equal(t, "finally success", res)
	mockAction.AssertExpectations(t)
	mockAction.AssertNumberOfCalls(t, "Execute", 3)
}

func TestRetry_FailAfterAllAttempts(t *testing.T) {
	cfg := RetryConfig{
		Attempts:    2,
		Delays:      []time.Duration{1 * time.Millisecond},
		ShouldRetry: func(err error) bool { return true },
	}
	mockAction := new(MockAction)

	expectedErr := errors.New("persistent error")
	mockAction.On("Execute", mock.Anything).Return(nil, expectedErr)

	res, err := cfg.Retry(context.Background(), mockAction.Execute)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, res)
	mockAction.AssertNumberOfCalls(t, "Execute", 2)
}

func TestRetry_ContextCancellation(t *testing.T) {
	cfg := RetryConfig{
		Attempts:    5,
		Delays:      []time.Duration{100 * time.Millisecond}, // Долгая задержка
		ShouldRetry: func(err error) bool { return true },
	}
	mockAction := new(MockAction)
	mockAction.On("Execute", mock.Anything).Return(nil, errors.New("err"))

	ctx, cancel := context.WithCancel(context.Background())

	// Запускаем ретрай в горутине
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel() // Отменяем контекст во время ожидания (sleep)
	}()

	start := time.Now()
	_, err := cfg.Retry(ctx, mockAction.Execute)
	duration := time.Since(start)

	assert.ErrorIs(t, err, context.Canceled)
	// Должно завершиться быстро, не дожидаясь всех 5 попыток по 100мс
	assert.Less(t, duration, 200*time.Millisecond)
}

func TestRetry_NonRetriableError(t *testing.T) {
	cfg := DefaultConfig()
	// Переопределяем ShouldRetry, чтобы он возвращал false для определенной ошибки
	cfg.ShouldRetry = func(err error) bool {
		return err.Error() != "fatal error"
	}

	mockAction := new(MockAction)
	mockAction.On("Execute", mock.Anything).Return(nil, errors.New("fatal error")).Once()

	_, err := cfg.Retry(context.Background(), mockAction.Execute)

	// Должен упасть сразу после первой попытки, не делая ретраев
	assert.Error(t, err)
	assert.Equal(t, "fatal error", err.Error())
	mockAction.AssertNumberOfCalls(t, "Execute", 1)
}

func TestDelayForAttempt(t *testing.T) {
	cfg := RetryConfig{
		Delays: []time.Duration{10 * time.Second, 20 * time.Second},
	}

	assert.Equal(t, 10*time.Second, cfg.delayForAttempt(0))
	assert.Equal(t, 20*time.Second, cfg.delayForAttempt(1))
	assert.Equal(t, 20*time.Second, cfg.delayForAttempt(2), "Should use last delay for out of bounds")
	assert.Equal(t, 20*time.Second, cfg.delayForAttempt(100))

	emptyCfg := RetryConfig{}
	assert.Equal(t, time.Second, emptyCfg.delayForAttempt(0), "Default fallback")
}

// --- Тесты логики defaultShouldRetry ---

func TestDefaultShouldRetry(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		{
			name:      "Nil error",
			err:       nil,
			wantRetry: false,
		},
		{
			name:      "Context canceled",
			err:       context.Canceled,
			wantRetry: false,
		},
		{
			name:      "Generic error",
			err:       errors.New("something wrong"),
			wantRetry: false,
		},
		{
			name:      "Connection refused string",
			err:       errors.New("dial tcp: connection refused"),
			wantRetry: true,
		},
		{
			name:      "Postgres Connection Exception (Code 08000)",
			err:       &pq.Error{Code: pgerrcode.ConnectionException},
			wantRetry: true,
		},
		{
			name:      "Postgres Unique Violation (Code 23505) - Should NOT retry",
			err:       &pq.Error{Code: pgerrcode.UniqueViolation},
			wantRetry: false,
		},
		{
			name:      "OS Path Error (EACCES)",
			err:       &os.PathError{Err: syscall.EACCES},
			wantRetry: true,
		},
		{
			name:      "OS Path Error (ENOENT) - File not found - Should NOT retry",
			err:       &os.PathError{Err: syscall.ENOENT},
			wantRetry: false,
		},
		{
			name:      "Net OpError Timeout",
			err:       &net.OpError{Err: os.ErrDeadlineExceeded}, // Имитация таймаута
			wantRetry: true,
		},
		{
			name:      "URL Error Timeout",
			err:       &url.Error{Err: context.DeadlineExceeded}, // Имитация таймаута URL
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantRetry, defaultShouldRetry(tt.err))
		})
	}
}

// Дополнительный тест для покрытия net.Error Timeout()
type timeoutError struct{}

func (e timeoutError) Error() string   { return "timeout" }
func (e timeoutError) Timeout() bool   { return true }
func (e timeoutError) Temporary() bool { return true }

func TestDefaultShouldRetry_NetTimeout(t *testing.T) {
	// Создаем ошибку, реализующую net.Error с Timeout() == true
	err := timeoutError{}
	assert.True(t, defaultShouldRetry(err))
}
