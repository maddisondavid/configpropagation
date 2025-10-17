package core

import (
	"math"
	"math/rand"
	"time"
)

// Sleeper abstracts time.Sleep for deterministic tests.
type Sleeper interface {
	Sleep(time.Duration)
}

// FuncSleeper wraps a function to satisfy Sleeper.
type FuncSleeper func(time.Duration)

// Sleep implements the Sleeper interface.
func (f FuncSleeper) Sleep(d time.Duration) { f(d) }

// BackoffStrategy holds retry parameters.
type BackoffStrategy struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	MaxAttempts int
	Jitter      float64
	Sleeper     Sleeper
	Rand        func() float64
}

// DefaultBackoff returns a conservative exponential backoff configuration.
func DefaultBackoff() BackoffStrategy {
	return BackoffStrategy{
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    3 * time.Second,
		MaxAttempts: 5,
		Jitter:      0.2,
	}
}

// Retry executes fn with exponential backoff. The function stops retrying when fn returns nil,
// when shouldRetry returns false, or after MaxAttempts have been exhausted. It returns the
// number of attempts executed and the last error from fn, if any.
func (b BackoffStrategy) Retry(fn func() error, shouldRetry func(error) bool) (int, error) {
	if b.MaxAttempts <= 0 {
		b.MaxAttempts = 1
	}
	if b.BaseDelay <= 0 {
		b.BaseDelay = 100 * time.Millisecond
	}
	if b.MaxDelay <= 0 {
		b.MaxDelay = time.Second
	}
	sleeper := b.Sleeper
	if sleeper == nil {
		sleeper = FuncSleeper(time.Sleep)
	}
	rnd := b.Rand
	if rnd == nil {
		rnd = rand.Float64
	}
	for attempt := 1; attempt <= b.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return attempt, nil
		}
		if shouldRetry != nil && !shouldRetry(err) {
			return attempt, err
		}
		if attempt == b.MaxAttempts {
			return attempt, err
		}
		delay := b.nextDelay(attempt)
		if b.Jitter > 0 {
			jitter := float64(delay) * b.Jitter * rnd()
			delay += time.Duration(jitter)
		}
		sleeper.Sleep(delay)
	}
	return b.MaxAttempts, nil
}

func (b BackoffStrategy) nextDelay(attempt int) time.Duration {
	exp := float64(attempt - 1)
	delay := float64(b.BaseDelay) * math.Pow(2, exp)
	max := float64(b.MaxDelay)
	if delay > max {
		delay = max
	}
	return time.Duration(delay)
}
