package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		cfg := DefaultRetryConfig()
		err := Retry(context.Background(), cfg, func() error { return nil }, DefaultIsRetryable)
		assert.NoError(t, err)
	})

	t.Run("succeeds after retries", func(t *testing.T) {
		cfg := RetryConfig{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, Multiplier: 2.0, Jitter: false}
		callCount := 0
		err := Retry(context.Background(), cfg, func() error {
			callCount++
			if callCount < 3 {
				return context.DeadlineExceeded
			}
			return nil
		}, DefaultIsRetryable)
		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)
	})

	t.Run("fails after max attempts", func(t *testing.T) {
		cfg := RetryConfig{MaxAttempts: 2, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond, Multiplier: 2.0, Jitter: false}
		err := Retry(context.Background(), cfg, func() error { return context.DeadlineExceeded }, DefaultIsRetryable)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("non-retryable error stops immediately", func(t *testing.T) {
		cfg := DefaultRetryConfig()
		callCount := 0
		nonRetryable := errors.New("business error")
		err := Retry(context.Background(), cfg, func() error {
			callCount++
			return nonRetryable
		}, DefaultIsRetryable)
		assert.Equal(t, nonRetryable, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("context cancellation stops retry", func(t *testing.T) {
		cfg := RetryConfig{MaxAttempts: 10, InitialDelay: 50 * time.Millisecond, MaxDelay: 200 * time.Millisecond, Multiplier: 2.0, Jitter: false}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := Retry(ctx, cfg, func() error { return context.DeadlineExceeded }, DefaultIsRetryable)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestRetryWithContext(t *testing.T) {
	t.Run("passes context to function", func(t *testing.T) {
		cfg := RetryConfig{MaxAttempts: 2, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond, Multiplier: 2.0, Jitter: false}
		callCount := 0
		err := RetryWithContext(context.Background(), cfg, func(ctx context.Context) error {
			callCount++
			if callCount < 2 {
				return context.DeadlineExceeded
			}
			return nil
		}, DefaultIsRetryable)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})
}

func TestCalculateDelay(t *testing.T) {
	t.Run("exponential backoff", func(t *testing.T) {
		cfg := RetryConfig{InitialDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second, Multiplier: 2.0, Jitter: false}
		assert.Equal(t, 100*time.Millisecond, calculateDelay(cfg, 0))
		assert.Equal(t, 200*time.Millisecond, calculateDelay(cfg, 1))
		assert.Equal(t, 400*time.Millisecond, calculateDelay(cfg, 2))
	})

	t.Run("capped at max delay", func(t *testing.T) {
		cfg := RetryConfig{InitialDelay: 1 * time.Second, MaxDelay: 5 * time.Second, Multiplier: 10.0, Jitter: false}
		assert.Equal(t, 5*time.Second, calculateDelay(cfg, 1))
	})
}

func TestDefaultIsRetryable(t *testing.T) {
	t.Run("nil error is not retryable", func(t *testing.T) {
		assert.False(t, DefaultIsRetryable(nil))
	})
	t.Run("deadline exceeded is retryable", func(t *testing.T) {
		assert.True(t, DefaultIsRetryable(context.DeadlineExceeded))
	})
	t.Run("canceled is not retryable", func(t *testing.T) {
		assert.False(t, DefaultIsRetryable(context.Canceled))
	})
	t.Run("generic error is not retryable", func(t *testing.T) {
		assert.False(t, DefaultIsRetryable(errors.New("some error")))
	})
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("starts in closed state", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 10*time.Second)
		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("transitions to open after max failures", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 10*time.Second)
		for i := 0; i < 3; i++ {
			err := cb.Execute(func() error { return errors.New("fail") })
			require.Error(t, err)
		}
		assert.Equal(t, StateOpen, cb.State())
	})

	t.Run("open state rejects calls", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 10*time.Second)
		err := cb.Execute(func() error { return errors.New("fail") })
		require.Error(t, err)
		err = cb.Execute(func() error { return nil })
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "open")
	})

	t.Run("success resets failures", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 10*time.Second)
		cb.Execute(func() error { return errors.New("fail") })
		cb.Execute(func() error { return errors.New("fail") })
		cb.Execute(func() error { return nil })
		assert.Equal(t, StateClosed, cb.State())
	})

	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 50*time.Millisecond)
		cb.Execute(func() error { return errors.New("fail") })
		assert.Equal(t, StateOpen, cb.State())
		time.Sleep(60 * time.Millisecond)
		assert.Equal(t, StateHalfOpen, cb.State())
	})

	t.Run("half-open allows trial request", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 50*time.Millisecond)
		cb.Execute(func() error { return errors.New("fail") })
		time.Sleep(60 * time.Millisecond)
		err := cb.Execute(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, StateClosed, cb.State())
	})
}

func TestBulkhead(t *testing.T) {
	t.Run("allows up to max concurrent", func(t *testing.T) {
		bh := NewBulkhead(2, 5*time.Second)
		err := bh.Execute(func() error { return nil })
		assert.NoError(t, err)
		err = bh.Execute(func() error { return nil })
		assert.NoError(t, err)
	})

	t.Run("rejects when capacity exceeded with short timeout", func(t *testing.T) {
		bh := NewBulkhead(1, 50*time.Millisecond)
		done := make(chan struct{})
		go func() {
			bh.Execute(func() error {
				time.Sleep(200 * time.Millisecond)
				close(done)
				return nil
			})
		}()
		time.Sleep(10 * time.Millisecond) // let goroutine acquire semaphore
		err := bh.Execute(func() error { return nil })
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
		<-done // cleanup goroutine
	})
}