package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"nowdone/pkg/retry"
)

const (
	chatCompletionsURL = "https://api.openai.com/v1/chat/completions"
	transcriptionsURL  = "https://api.openai.com/v1/audio/transcriptions"
)

// Per-call retry policies. Both endpoints are effectively idempotent for our
// use (we send the same prompt / audio and read the answer), so a transient
// network failure or a 429 / 5xx is safe to retry with backoff.
var (
	intentRetry = retry.Policy{
		Attempts:       3,
		BaseDelay:      500 * time.Millisecond,
		MaxDelay:       4 * time.Second,
		AttemptTimeout: 20 * time.Second,
	}
	transcribeRetry = retry.Policy{
		Attempts:       2,
		BaseDelay:      time.Second,
		MaxDelay:       5 * time.Second,
		AttemptTimeout: 45 * time.Second,
	}
)

// Intent is the structured result of parsing a user's free-form Telegram
// message (text or voice transcript) into an actionable task command.
type Intent struct {
	Action      string `json:"action"` // "create" | "update_status" | "delete" | "list" | "reschedule"
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Date        string `json:"date,omitempty"`     // YYYY-MM-DD
	Status      string `json:"status,omitempty"`   // "done" | "not_done"
	NewDate     string `json:"new_date,omitempty"` // YYYY-MM-DD, for reschedule
	// ReminderTime is a "YYYY-MM-DDTHH:MM" local wall-clock value the model
	// fills in when the user asks to be reminded / gives an alarm time
	// ("напомни в 15:00", "с напоминанием завтра в 10 утра"). Empty otherwise.
	ReminderTime string `json:"reminder_time,omitempty"`
}

const intentSystemPrompt = `You are an intent parser for a daily planner app called NowDone.
Given a user's message (in Russian or English), determine their intent and extract parameters.
Respond with ONLY a single JSON object, no prose, no markdown fences, matching exactly this shape:
{"action":"create|update_status|delete|list|reschedule","title":"","description":"","date":"YYYY-MM-DD or empty","status":"done|not_done or empty","new_date":"YYYY-MM-DD or empty","reminder_time":"YYYY-MM-DDTHH:MM or empty"}
Use the user's current local date and time (given below) as the reference for relative values like "завтра" (tomorrow), "сегодня" (today), "в пятницу" (on Friday).
Set "reminder_time" to a "YYYY-MM-DDTHH:MM" local datetime ONLY when the user explicitly asks for a reminder/alarm or states a clock time to be notified at (e.g. "напомни в 15:00", "с напоминанием завтра в 10 утра", "установи напоминание на пятницу 18:00"). Otherwise leave it "".
If a field is not applicable, use an empty string.`

// OpenAIService calls gpt-4o-mini to turn a transcribed message into an Intent.
type OpenAIService struct {
	apiKey string
	http   *http.Client
	log    *slog.Logger
}

// NewOpenAIService builds the service. A nil logger falls back to
// slog.Default(). The http.Client keeps a generous backstop timeout; the real
// per-attempt bound comes from the retry policy's AttemptTimeout.
func NewOpenAIService(apiKey string, log *slog.Logger) *OpenAIService {
	if log == nil {
		log = slog.Default()
	}
	return &OpenAIService{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 60 * time.Second},
		log:    log,
	}
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Temperature    float64         `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// ParseIntent sends the transcribed/typed message to gpt-4o-mini and parses
// the JSON-only response into an Intent. nowRef is the user's current local
// date-time as "YYYY-MM-DDTHH:MM", used to resolve relative dates and reminder
// times.
func (s *OpenAIService) ParseIntent(ctx context.Context, userMessage string, nowRef string) (*Intent, error) {
	start := time.Now()

	reqBody := chatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []chatMessage{
			{Role: "system", Content: intentSystemPrompt + "\nUser's current local date and time: " + nowRef},
			{Role: "user", Content: userMessage},
		},
		ResponseFormat: &responseFormat{Type: "json_object"},
		Temperature:    0,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, err := retry.DoValue(ctx, intentRetry, func(ctx context.Context) ([]byte, error) {
		return s.postJSON(ctx, chatCompletionsURL, body)
	})
	s.logCall("parse_intent", start, err)
	if err != nil {
		return nil, err
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return nil, fmt.Errorf("unmarshal completion: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("empty completion")
	}

	var intent Intent
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &intent); err != nil {
		return nil, fmt.Errorf("unmarshal intent json: %w", err)
	}
	return &intent, nil
}

// TranscribeVoice sends an OGG/Opus voice file to Whisper (via the OpenAI
// audio transcription endpoint) and returns the recognized text. Telegram
// already offers built-in STT for some clients, but we fall back to this for
// reliability across all clients.
func (s *OpenAIService) TranscribeVoice(ctx context.Context, audioBytes []byte, filename string) (string, error) {
	start := time.Now()

	const boundary = "nowdoneboundary"
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--%s\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nwhisper-1\r\n", boundary)
	fmt.Fprintf(&buf, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\nContent-Type: audio/ogg\r\n\r\n", boundary, filename)
	buf.Write(audioBytes)
	fmt.Fprintf(&buf, "\r\n--%s--\r\n", boundary)
	// Snapshot the body so each retry attempt sends a fresh reader.
	multipartBody := buf.Bytes()
	contentType := "multipart/form-data; boundary=" + boundary

	respBody, err := retry.DoValue(ctx, transcribeRetry, func(ctx context.Context) ([]byte, error) {
		return s.post(ctx, transcriptionsURL, contentType, multipartBody)
	})
	s.logCall("transcribe_voice", start, err)
	if err != nil {
		return "", err
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal transcription: %w", err)
	}
	return result.Text, nil
}

// postJSON is post with a JSON content type.
func (s *OpenAIService) postJSON(ctx context.Context, url string, body []byte) ([]byte, error) {
	return s.post(ctx, url, "application/json", body)
}

// post issues one authenticated POST and returns the response body. A 429 or
// 5xx is returned as a plain (retryable) error; every other non-200 is wrapped
// with retry.Stop so the caller fails fast instead of hammering a bad request.
func (s *OpenAIService) post(ctx context.Context, url, contentType string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, retry.Stop(fmt.Errorf("build request: %w", err))
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		return respBody, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(respBody))
	default:
		return nil, retry.Stop(fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(respBody)))
	}
}

// logCall records one structured line per OpenAI request with its duration.
func (s *OpenAIService) logCall(op string, start time.Time, err error) {
	elapsed := time.Since(start)
	if err != nil {
		s.log.Error("openai call failed", "op", op, "elapsed_ms", elapsed.Milliseconds(), "error", err)
		return
	}
	s.log.Info("openai call", "op", op, "elapsed_ms", elapsed.Milliseconds())
}
