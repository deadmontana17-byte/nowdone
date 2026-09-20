package telegram

import (
	"net"
	"net/http"
	"time"
)

// clientTimeout is the whole-request ceiling for every Telegram call: dial +
// TLS + send + wait for response headers + read body. 60s is generous on
// purpose — the Bot API is reached through a Cloudflare Worker whose cold start
// and CF->api.telegram.org hop occasionally push a normally-0.2s call into the
// tens of seconds. It is still safe for tgbotapi's getUpdates long-poll: that
// poll is capped at 30s server-side (u.Timeout = 30), which fits inside 60s.
const clientTimeout = 60 * time.Second

// NewHTTPClient builds the one HTTP client the bot shares between every Telegram
// Bot API call (via tgbotapi) and every Telegram file download. Sharing it means
// connections are pooled and reused instead of dialled per request: the
// Transport below keeps up to 16 idle keep-alive connections per host for 90s,
// so back-to-back calls skip the dial + TLS handshake entirely.
//
// Blocking is bounded at every layer:
//
//   - DialContext / TLSHandshakeTimeout (10s each): a dead or unreachable host
//     (or a Worker that will not accept the connection) fails fast instead of
//     hanging.
//   - ResponseHeaderTimeout (60s): a connected-but-silent Worker fails after a
//     full minute rather than the previous 50s — the long-poll still fits.
//   - Client.Timeout (60s): the absolute cap on the whole exchange, matching the
//     retry layer's per-attempt timeout in send.go so neither obscures the other.
//
// On top of this, callers in send.go / download.go add bounded retries with
// exponential backoff (see pkg/retry) for transient dial/TLS/timeout failures.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: clientTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			// KeepAlive keeps the underlying TCP connection warm; combined with
			// MaxIdleConnsPerHost this is what lets the pool actually reuse
			// connections to the Worker instead of re-dialling every call.
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   16,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		},
	}
}
