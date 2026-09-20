package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"nowdone/pkg/retry"
)

// maxDownloadBytes caps an in-memory Telegram file download. Telegram's own bot
// download limit is 20 MiB; the extra megabyte lets us detect an over-size file
// rather than silently truncating it.
const maxDownloadBytes = 21 << 20

// downloadPolicy retries a stalled or reset file download. GET is idempotent, so
// a full retry is safe. AttemptTimeout matches the HTTP client's 60s ceiling so
// a large file coming slowly through the Worker is not aborted just short of
// completing; backoff is 1s, 2s, 4s.
var downloadPolicy = retry.Policy{
	Attempts:       3,
	BaseDelay:      1 * time.Second,
	MaxDelay:       4 * time.Second,
	AttemptTimeout: 60 * time.Second,
}

// fileDirectURL resolves a Telegram file id to a direct download URL, routed
// through the configured proxy. The underlying getFile call is idempotent and
// retried on transient errors, bounded by ctx.
func (b *Bot) fileDirectURL(ctx context.Context, fileID string) (string, error) {
	raw, err := retry.DoValue(ctx, requestPolicy, func(ctx context.Context) (string, error) {
		return callWithDeadline(ctx, func() (string, error) { return b.api.GetFileDirectURL(fileID) })
	})
	if err != nil {
		return "", fmt.Errorf("resolve telegram file url: %w", err)
	}
	return proxyFileURL(b.apiBaseURL, raw), nil
}

// downloadFile GETs url into memory using the shared pooled client. Each attempt
// is bounded by ctx and by downloadPolicy.AttemptTimeout; the body is capped at
// maxDownloadBytes; 429 / 5xx responses are retried, other non-200s are not.
// This replaces the previous bare http.Get, whose lack of any timeout was a
// primary cause of stuck voice/attachment handlers.
func (b *Bot) downloadFile(ctx context.Context, url string) (data []byte, contentType string, err error) {
	err = retry.Do(ctx, downloadPolicy, func(ctx context.Context) error {
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if e != nil {
			return retry.Stop(e)
		}

		resp, e := b.http.Do(req)
		if e != nil {
			return e
		}
		defer resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusOK:
			// fall through to read
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			return fmt.Errorf("download %s: upstream status %d", url, resp.StatusCode)
		default:
			return retry.Stop(fmt.Errorf("download %s: status %d", url, resp.StatusCode))
		}

		buf, e := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
		if e != nil {
			return e
		}
		if int64(len(buf)) > maxDownloadBytes {
			return retry.Stop(fmt.Errorf("download %s: file larger than %d bytes", url, maxDownloadBytes))
		}

		data = buf
		contentType = resp.Header.Get("Content-Type")
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return data, contentType, nil
}
