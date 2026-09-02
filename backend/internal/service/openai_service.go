package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
}

func NewOpenAIService(apiKey string) *OpenAIService {
	return &OpenAIService{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 20 * time.Second},
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call openai: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(respBody))
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
	var buf bytes.Buffer
	boundary := "nowdoneboundary"
	writeField := func(name, value string) {
		fmt.Fprintf(&buf, "--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s\r\n", boundary, name, value)
	}

	writeField("model", "whisper-1")
	fmt.Fprintf(&buf, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\nContent-Type: audio/ogg\r\n\r\n", boundary, filename)
	buf.Write(audioBytes)
	fmt.Fprintf(&buf, "\r\n--%s--\r\n", boundary)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("call whisper: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("whisper returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal transcription: %w", err)
	}
	return result.Text, nil
}
