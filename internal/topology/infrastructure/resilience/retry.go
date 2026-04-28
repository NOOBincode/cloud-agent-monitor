package resilience

import (
	"context"
	"errors"
	"net"
	"time"
)

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       bool
	OnRetry      OnRetryCallback
}

type OnRetryCallback func(ctx context.Context, attempt int, err error)

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

type RetryableFunc func() error

type RetryableFuncWithContext func(ctx context.Context) error

type IsRetryable func(err error) bool

func Retry(ctx context.Context, cfg RetryConfig, fn RetryableFunc, isRetryable IsRetryable) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		lastErr = err
		if cfg.OnRetry != nil {
			cfg.OnRetry(ctx, attempt+1, err)
		}
		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(cfg, attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return lastErr
}

func RetryWithContext(ctx context.Context, cfg RetryConfig, fn RetryableFuncWithContext, isRetryable IsRetryable) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		lastErr = err
		if cfg.OnRetry != nil {
			cfg.OnRetry(ctx, attempt+1, err)
		}
		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(cfg, attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return lastErr
}

func calculateDelay(cfg RetryConfig, attempt int) time.Duration {
	delay := float64(cfg.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= cfg.Multiplier
	}
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	if cfg.Jitter {
		delay = delay * randomJitter()
	}
	return time.Duration(delay)
}

func DefaultIsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

type OnStateChangeCallback func(fromState, toState CircuitState)

type CircuitBreaker struct {
	maxFailures    int
	timeout        time.Duration
	failures       int
	lastFailTime   time.Time
	state          CircuitState
	OnStateChange  OnStateChangeCallback
}

type CircuitState int

const (
	StateClosed    CircuitState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       StateClosed,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	prevState := cb.state
	if cb.state == StateOpen {
		if time.Since(cb.lastFailTime) > cb.timeout {
			cb.state = StateHalfOpen
			cb.notifyStateChange(prevState, cb.state)
		} else {
			return errors.New("circuit breaker is open")
		}
	}

	err := fn()
	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()
		prevState := cb.state
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}
		cb.notifyStateChange(prevState, cb.state)
		return err
	}

	prevState = cb.state
	cb.failures = 0
	cb.state = StateClosed
	cb.notifyStateChange(prevState, cb.state)
	return nil
}

func (cb *CircuitBreaker) notifyStateChange(from, to CircuitState) {
	if cb.OnStateChange != nil && from != to {
		cb.OnStateChange(from, to)
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	if cb.state == StateOpen && time.Since(cb.lastFailTime) > cb.timeout {
		return StateHalfOpen
	}
	return cb.state
}

func RetryWithCircuitBreaker(ctx context.Context, retryCfg RetryConfig, cb *CircuitBreaker, fn RetryableFunc) error {
	if cb.State() == StateOpen {
		return errors.New("circuit breaker is open")
	}
	return Retry(ctx, retryCfg, fn, func(err error) bool {
		return DefaultIsRetryable(err)
	})
}

type OnBulkheadChangeCallback func(delta int64, concurrency int)

type Bulkhead struct {
	semaphore      chan struct{}
	maxConcurrent  int
	timeout        time.Duration
	OnActiveChange OnBulkheadChangeCallback
}

func NewBulkhead(maxConcurrent int, timeout time.Duration) *Bulkhead {
	return &Bulkhead{
		semaphore:     make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
		timeout:       timeout,
	}
}

func (b *Bulkhead) Execute(fn func() error) error {
	select {
	case b.semaphore <- struct{}{}:
		if b.OnActiveChange != nil {
			b.OnActiveChange(1, b.maxConcurrent)
		}
		defer func() {
			<-b.semaphore
			if b.OnActiveChange != nil {
				b.OnActiveChange(-1, b.maxConcurrent)
			}
		}()
		return fn()
	case <-time.After(b.timeout):
		return errors.New("bulkhead: timeout waiting for semaphore")
	}
}

func randomJitter() float64 {
	return 0.5 + float64(time.Now().UnixNano()%1000)/1000.0*0.5
}