package retry

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

// Action — интерфейс для автоматической генерации мока через mockery.
// Сигнатура совпадает с rFunc func(...any) (any, error).
//
//go:generate go run github.com/vektra/mockery/v2@v2.53.5 --name Action --inpackage --case underscore
type Action interface {
	Execute(args ...any) (any, error)
}

type RetryConfig struct {
	Attempts    int
	Delays      []time.Duration
	ShouldRetry func(error) bool
	OnRetry     func(err error, attempt int, delay time.Duration)
}

func DefaultConfig() RetryConfig {
	return RetryConfig{
		Attempts:    4,
		Delays:      []time.Duration{time.Second, 3 * time.Second, 5 * time.Second},
		ShouldRetry: defaultShouldRetry,
	}
}

func (cfg RetryConfig) Retry(ctx context.Context, rFunc func(...any) (any, error), args ...any) (any, error) {
	attempts := cfg.Attempts
	if attempts <= 0 {
		attempts = 1
	}

	shouldRetry := cfg.ShouldRetry
	if shouldRetry == nil {
		shouldRetry = defaultShouldRetry
	}

	var result any
	var err error

	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result, err = rFunc(args...)
		if err == nil {
			return result, nil
		}

		if !shouldRetry(err) || attempt == attempts-1 {
			return nil, err
		}

		delay := cfg.delayForAttempt(attempt)
		if cfg.OnRetry != nil {
			cfg.OnRetry(err, attempt+1, delay)
		}

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, err
}

func (cfg RetryConfig) delayForAttempt(attempt int) time.Duration {
	if len(cfg.Delays) == 0 {
		return time.Second
	}
	if attempt >= len(cfg.Delays) {
		return cfg.Delays[len(cfg.Delays)-1]
	}
	return cfg.Delays[attempt]
}

func defaultShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
		err = urlErr.Err
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		// fall through to text-based checks below
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		code := string(pqErr.Code)
		if pgerrcode.IsConnectionException(code) {
			return true
		}

		// многие уникальные нарушения не требуют повторов
		if code == pgerrcode.UniqueViolation {
			return false
		}
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errors.Is(pathErr.Err, syscall.EACCES) ||
			errors.Is(pathErr.Err, syscall.EAGAIN) ||
			errors.Is(pathErr.Err, syscall.EBUSY) {
			return true
		}
		return false
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "no such host") {
		return true
	}

	return false
}
