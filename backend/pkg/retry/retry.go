// Package retry runs an operation again, with exponential backoff and jitter,
// when it fails with a transient network error (connection reset, timeout,
// broken pipe, …). It is context-aware: the parent context aborts the whole
// loop immediately, and an optional per-attempt timeout bounds each try so a
// single stalled call can never wedge the caller.
//
// It has no third-party dependencies on purpose — the bot must build and run in
// environments where fetching extra modules is not possible.
package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"strings"
	"syscall"
	"time"
)

// Policy configures a retry loop. The zero value is usable: it behaves as
// Attempts=3, BaseDelay=200ms, MaxDelay=5s, no per-attempt timeout.
type Policy struct {
	// Attempts is the maximum number of tries, including the first. Values < 1
	// are treated as 3.
	Attempts int
	// BaseDelay is the backoff before the second attempt; it doubles each time,
	// capped at MaxDelay, with equal jitter applied.
	BaseDelay time.Duration
	// MaxDelay caps the per-attempt backoff.
	MaxDelay time.Duration
	// AttemptTimeout, when > 0, wraps every attempt in its own
	// context.WithTimeout. A timeout that fires here counts as a transient
	// failure and is retried; the parent context expiring does not.
	AttemptTimeout time.Duration
}

func (p Policy) withDefaults() Policy {
	if p.Attempts < 1 {
		p.Attempts = 3
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 200 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 5 * time.Second
	}
	return p
}

// Classifier lets an operation override the transient/permanent decision for its
// own error type — e.g. an HTTP client that wants 429 and 5xx retried but 4xx
// not. An error that implements it (anywhere in its Unwrap chain) wins over the
// built-in network-error heuristics.
type Classifier interface {
	Retryable() bool
}

type stopError struct{ err error }

func (e stopError) Error() string   { return e.err.Error() }
func (e stopError) Unwrap() error   { return e.err }
func (e stopError) Retryable() bool { return false }

// Stop wraps err so the retry loop treats it as permanent and returns it
// immediately. errors.Is / errors.As still see through to the original error.
func Stop(err error) error {
	if err == nil {
		return nil
	}
	return stopError{err: err}
}

// Do calls op until it returns nil, its error is permanent, or Attempts is
// reached. op receives the (possibly per-attempt-timeout-bounded) context and
// must use it for its I/O. The last error is returned, wrapped with the attempt
// count once the budget is exhausted.
func Do(ctx context.Context, p Policy, op func(ctx context.Context) error) error {
	p = p.withDefaults()

	var last error
	for attempt := 1; attempt <= p.Attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return joinErr(last, err)
		}

		attemptCtx := ctx
		var cancel context.CancelFunc
		if p.AttemptTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, p.AttemptTimeout)
		}

		last = op(attemptCtx)
		// Distinguish "this attempt used up its own budget" (retry) from "the
		// caller's context is done" (give up) before cancel() muddies the state.
		perAttemptTimeout := p.AttemptTimeout > 0 &&
			attemptCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil
		if cancel != nil {
			cancel()
		}

		if last == nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return joinErr(last, err)
		}
		if !retryable(last, perAttemptTimeout) {
			return last
		}
		if attempt == p.Attempts {
			break
		}
		if err := wait(ctx, backoff(p, attempt)); err != nil {
			return joinErr(last, err)
		}
	}
	return fmt.Errorf("retry: gave up after %d attempts: %w", p.Attempts, last)
}

// DoValue is Do for an operation that produces a value. On failure the zero
// value of T is returned alongside the error.
func DoValue[T any](ctx context.Context, p Policy, op func(ctx context.Context) (T, error)) (T, error) {
	var out T
	err := Do(ctx, p, func(ctx context.Context) error {
		v, e := op(ctx)
		if e != nil {
			return e
		}
		out = v
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}

func retryable(err error, perAttemptTimeout bool) bool {
	var c Classifier
	if errors.As(err, &c) {
		return c.Retryable()
	}
	if perAttemptTimeout {
		return true
	}
	return IsTransient(err)
}

func backoff(p Policy, attempt int) time.Duration {
	d := p.BaseDelay
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= p.MaxDelay {
			d = p.MaxDelay
			break
		}
	}
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	// Equal jitter: wait between 50% and 100% of the computed delay so a fleet
	// of callers that failed together do not retry in lockstep.
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func joinErr(last, ctxErr error) error {
	if last == nil {
		return ctxErr
	}
	return errors.Join(last, ctxErr)
}

// IsTransient reports whether err looks like a temporary network fault worth
// retrying: an i/o timeout, a reset/refused/aborted connection, a broken pipe,
// an unexpected EOF or a TLS handshake timeout. A cancelled or deadline-exceeded
// context is NOT transient — that is the caller telling us to stop.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	return containsAny(strings.ToLower(err.Error()),
		"connection reset by peer",
		"connection refused",
		"broken pipe",
		"unexpected eof",
		"tls handshake timeout",
		"i/o timeout",
		"network is unreachable",
		"no route to host",
		"server misbehaving",
		"http2: server sent goaway",
	)
}

// IsConnError reports the narrower case where the connection was never
// established (dial / DNS / TLS failure) or was reset. Such a request provably
// did not reach the server, so even a non-idempotent call is safe to retry.
func IsConnError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	return containsAny(strings.ToLower(err.Error()),
		"connection refused",
		"connection reset by peer",
		"no such host",
		"tls handshake timeout",
		"no route to host",
		"network is unreachable",
		"server misbehaving",
	)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
