package telegram

import (
	"context"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"nowdone/pkg/retry"
)

// tgbotapi v5 has no context support anywhere in its request path, so the only
// levers left are (a) the timeout-bounded http.Client injected in client.go and
// (b) these wrappers, which add a hard per-call ceiling, bounded retries and
// timing logs on top.

var (
	// requestPolicy covers idempotent Bot API calls (answerCallbackQuery,
	// deleteMessage, setMyCommands, getFile): re-issuing them has no visible
	// side effect, so they retry freely on any transient error.
	//
	// AttemptTimeout is 60s to match the HTTP client's whole-request ceiling
	// (NewHTTPClient): a single attempt is allowed to run as long as the Worker
	// can possibly take, and only a genuine transport failure (which surfaces
	// before 60s) triggers a retry. Backoff is 1s, 2s, 4s (capped at MaxDelay),
	// with equal jitter applied by pkg/retry.
	requestPolicy = retry.Policy{
		Attempts:       3,
		BaseDelay:      1 * time.Second,
		MaxDelay:       4 * time.Second,
		AttemptTimeout: 60 * time.Second,
	}

	// sendPolicy is deliberately narrower: a chat message must not be delivered
	// twice. Each attempt still gets the full 60s (so a slow-but-successful
	// Worker call is NOT killed mid-flight — that was the main source of
	// "context deadline exceeded"), but a retry only happens for errors that
	// prove the request never left this process (dial / DNS / TLS failures,
	// connection resets); see the retry.IsConnError guard in send().
	sendPolicy = retry.Policy{
		Attempts:       3,
		BaseDelay:      1 * time.Second,
		MaxDelay:       4 * time.Second,
		AttemptTimeout: 60 * time.Second,
	}
)

// send delivers a message, photo, invoice, edit, … It does not retry on
// ambiguous failures (a timeout after the request was already on the wire) to
// avoid duplicate messages; the injected http.Client's timeouts already stop the
// old "hangs forever" behaviour.
//
// ctx is the caller's own deadline (the per-update timeout from handleUpdate,
// or a handler-specific one): it bounds the whole retry loop, so a stuck
// Telegram/proxy response can no longer keep this call — and the goroutine
// holding it — alive past the caller's own budget.
func (b *Bot) send(ctx context.Context, c tgbotapi.Chattable) (tgbotapi.Message, error) {
	start := time.Now()
	msg, err := retry.DoValue(ctx, sendPolicy, func(ctx context.Context) (tgbotapi.Message, error) {
		m, e := callWithDeadline(ctx, func() (tgbotapi.Message, error) { return b.api.Send(c) })
		if e != nil && !retry.IsConnError(e) {
			return m, retry.Stop(e) // permanent for this call
		}
		return m, e
	})
	b.logAPICall("send", start, err)
	return msg, err
}

// request runs an idempotent Bot API call with full retries, bounded by ctx
// (see send's doc comment on why this must not be context.Background()).
func (b *Bot) request(ctx context.Context, c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	start := time.Now()
	resp, err := retry.DoValue(ctx, requestPolicy, func(ctx context.Context) (*tgbotapi.APIResponse, error) {
		return callWithDeadline(ctx, func() (*tgbotapi.APIResponse, error) { return b.api.Request(c) })
	})
	b.logAPICall("request", start, err)
	return resp, err
}

// slowAPICallThreshold is the wall-clock duration above which a Telegram
// round-trip is logged at Warn instead of Info, so a slow Worker stands out.
const slowAPICallThreshold = 3 * time.Second

// logAPICall emits one structured line per Bot API call (including any retries)
// with its wall-clock duration. It logs at Info on success so the timing of
// every Telegram round-trip is visible in production too — the default prod log
// level hides Debug — which is what makes it possible to see whether the delay
// is on Telegram's / the Worker's side rather than in our own handlers.
func (b *Bot) logAPICall(op string, start time.Time, err error) {
	elapsed := time.Since(start)
	switch {
	case err != nil:
		b.log.Error("telegram api call failed", "op", op, "elapsed_ms", elapsed.Milliseconds(), "error", err)
	case elapsed > slowAPICallThreshold:
		b.log.Warn("telegram api call slow", "op", op, "elapsed_ms", elapsed.Milliseconds())
	default:
		b.log.Info("telegram api call", "op", op, "elapsed_ms", elapsed.Milliseconds())
	}
}

// callWithDeadline runs fn (a blocking, non-cancellable tgbotapi call) in a
// goroutine and returns as soon as ctx is done, giving the caller a real
// deadline the library cannot otherwise honour. On timeout the goroutine is left
// to finish on its own — it is bounded by the http.Client's ResponseHeaderTimeout
// (≤50s) and its result is discarded.
func callWithDeadline[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	type result struct {
		val T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := fn()
		ch <- result{val: v, err: err}
	}()

	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case r := <-ch:
		return r.val, r.err
	}
}
