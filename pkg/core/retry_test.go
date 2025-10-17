package core

import (
	"errors"
	"testing"
	"time"
)

func TestRetryStopsAfterSuccess(t *testing.T) {
	attempts := 0
	sleeper := &captureSleeper{}
	strategy := DefaultBackoff()
	strategy.Sleeper = sleeper
	strategy.Rand = func() float64 { return 0 }
	gotAttempts, err := strategy.Retry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("fail")
		}
		return nil
	}, func(error) bool { return true })
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if gotAttempts != 3 {
		t.Fatalf("expected 3 attempts got %d", gotAttempts)
	}
	if len(sleeper.calls) != 2 {
		t.Fatalf("expected 2 sleeps, got %d", len(sleeper.calls))
	}
	// With zero jitter, delays should double until cap.
	if sleeper.calls[0] != 100*time.Millisecond || sleeper.calls[1] != 200*time.Millisecond {
		t.Fatalf("unexpected delays: %+v", sleeper.calls)
	}
}

func TestRetryStopsOnNonRetryable(t *testing.T) {
	strategy := DefaultBackoff()
	attempts, err := strategy.Retry(func() error {
		return errors.New("fatal")
	}, func(error) bool { return false })
	if err == nil {
		t.Fatalf("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

type captureSleeper struct{ calls []time.Duration }

func (c *captureSleeper) Sleep(d time.Duration) { c.calls = append(c.calls, d) }
