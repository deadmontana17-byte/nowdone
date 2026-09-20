package retry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func fastPolicy(attempts int) Policy {
	return Policy{Attempts: attempts, BaseDelay: time.Millisecond, MaxDelay: 4 * time.Millisecond}
}

func TestDo_SucceedsFirstTry(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastPolicy(3), func(context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("want 1 call, got %d", calls)
	}
}

func TestDo_RetriesTransientThenSucceeds(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastPolicy(5), func(context.Context) error {
		calls++
		if calls < 3 {
			return &net.OpError{Op: "read", Err: errors.New("connection reset by peer")}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("want nil after recovery, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("want 3 calls, got %d", calls)
	}
}

func TestDo_ExhaustsAttempts(t *testing.T) {
	calls := 0
	sentinel := errors.New("connection refused")
	err := Do(context.Background(), fastPolicy(3), func(context.Context) error {
		calls++
		return sentinel
	})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("want wrapped sentinel, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("want 3 calls, got %d", calls)
	}
}

func TestDo_PermanentErrorNotRetried(t *testing.T) {
	calls := 0
	permanent := errors.New("bad request: 400")
	err := Do(context.Background(), fastPolicy(4), func(context.Context) error {
		calls++
		return permanent
	})
	if !errors.Is(err, permanent) {
		t.Fatalf("want permanent error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("permanent error must not retry, got %d calls", calls)
	}
}

func TestDo_StopWrapsAsPermanent(t *testing.T) {
	calls := 0
	inner := errors.New("connection reset by peer") // transient-looking...
	err := Do(context.Background(), fastPolicy(4), func(context.Context) error {
		calls++
		return Stop(inner) // ...but explicitly marked permanent
	})
	if !errors.Is(err, inner) {
		t.Fatalf("errors.Is must see through Stop, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("Stop must prevent retries, got %d calls", calls)
	}
}

func TestDo_ClassifierOverride(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastPolicy(3), func(context.Context) error {
		calls++
		return retryableErr{fmt.Errorf("openai returned 429")}
	})
	if err == nil {
		t.Fatal("want error after exhausting retries")
	}
	if calls != 3 {
		t.Fatalf("Classifier said retryable, want 3 calls, got %d", calls)
	}
}

func TestDo_ParentContextCancelStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := Do(ctx, Policy{Attempts: 10, BaseDelay: 20 * time.Millisecond}, func(context.Context) error {
		calls++
		cancel() // cancel mid-flight
		return errors.New("i/o timeout")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("want loop to stop after cancel, got %d calls", calls)
	}
}

func TestDo_PerAttemptTimeoutIsRetried(t *testing.T) {
	calls := 0
	p := Policy{Attempts: 3, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond, AttemptTimeout: 15 * time.Millisecond}
	err := Do(context.Background(), p, func(ctx context.Context) error {
		calls++
		if calls < 2 {
			<-ctx.Done() // blow the per-attempt budget once
			return ctx.Err()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("per-attempt timeout should be retried, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("want 2 calls, got %d", calls)
	}
}

func TestDoValue(t *testing.T) {
	calls := 0
	got, err := DoValue(context.Background(), fastPolicy(4), func(context.Context) (int, error) {
		calls++
		if calls < 2 {
			return 0, errors.New("broken pipe")
		}
		return 42, nil
	})
	if err != nil || got != 42 {
		t.Fatalf("want 42/nil, got %d/%v", got, err)
	}
}

type retryableErr struct{ error }

func (retryableErr) Retryable() bool { return true }
