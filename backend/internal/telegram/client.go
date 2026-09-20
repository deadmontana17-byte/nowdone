package telegram

import (
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// DefaultAPIURL is the standard Telegram Bot API origin, used whenever
// TELEGRAM_API_PROXY_URL is not configured.
const DefaultAPIURL = "https://api.telegram.org"

// NewBotAPI builds a Telegram Bot API client. apiBaseURL is normally
// DefaultAPIURL; pass a proxy origin (for example a Cloudflare Worker URL) to
// route every Bot API call through it on networks where api.telegram.org is
// blocked. An empty or default value keeps the library's standard behaviour.
//
// httpClient is the pooled, timeout-bounded client from NewHTTPClient. It is
// injected (rather than letting tgbotapi create a bare &http.Client{}) so Bot
// API calls can no longer block indefinitely on a stalled connection. Passing
// nil falls back to a fresh NewHTTPClient.
func NewBotAPI(token, apiBaseURL string, httpClient *http.Client) (*tgbotapi.BotAPI, error) {
	if httpClient == nil {
		httpClient = NewHTTPClient()
	}

	// v5 formats requests as fmt.Sprintf(endpoint, token, method), so the
	// endpoint must keep the "/bot%s/%s" shape.
	endpoint := tgbotapi.APIEndpoint
	if base := normalizeAPIBase(apiBaseURL); base != DefaultAPIURL {
		endpoint = base + "/bot%s/%s"
	}
	return tgbotapi.NewBotAPIWithClient(token, endpoint, httpClient)
}

// proxyFileURL rewrites a file download URL returned by GetFileDirectURL so it
// points at the configured proxy instead of api.telegram.org. The tgbotapi v5
// File.Link helper hard-codes the official host, so file downloads bypass the
// proxy endpoint unless rewritten here. It is a no-op when no proxy is set.
func proxyFileURL(apiBaseURL, rawURL string) string {
	base := normalizeAPIBase(apiBaseURL)
	if base == DefaultAPIURL {
		return rawURL
	}
	return strings.Replace(rawURL, DefaultAPIURL, base, 1)
}

// normalizeAPIBase trims surrounding whitespace and any trailing slash, falling
// back to DefaultAPIURL when the value is empty.
func normalizeAPIBase(apiBaseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if base == "" {
		return DefaultAPIURL
	}
	return base
}
