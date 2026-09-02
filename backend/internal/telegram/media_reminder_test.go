package telegram

import (
	"testing"
	"time"

	"nowdone/internal/models"
)

func TestPrimaryMedia(t *testing.T) {
	tests := []struct {
		name     string
		atts     []models.Attachment
		wantURL  string
		wantKind string
	}{
		{
			name:     "normalised category",
			atts:     []models.Attachment{{Type: "image", URL: "https://s3/a.jpg"}},
			wantURL:  "https://s3/a.jpg",
			wantKind: "image",
		},
		{
			name:     "raw MIME type",
			atts:     []models.Attachment{{Type: "image/png", URL: "https://s3/b"}},
			wantURL:  "https://s3/b",
			wantKind: "image",
		},
		{
			name:     "type missing, sniff extension from URL",
			atts:     []models.Attachment{{Type: "", URL: "https://s3/c.jpeg?token=x"}},
			wantURL:  "https://s3/c.jpeg?token=x",
			wantKind: "image",
		},
		{
			name:     "image is preferred over video, scheme-less URL is upgraded",
			atts:     []models.Attachment{{Type: "video", URL: "v.mp4"}, {Type: "image", URL: "i.png"}},
			wantURL:  "https://i.png",
			wantKind: "image",
		},
		{
			name:     "video only, scheme-less URL is upgraded",
			atts:     []models.Attachment{{Type: "video/mp4", URL: "v"}},
			wantURL:  "https://v",
			wantKind: "video",
		},
		{
			name: "documents are not inlined",
			atts: []models.Attachment{{Type: "file", URL: "doc.pdf", Name: "doc.pdf"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url, kind := primaryMedia(tc.atts)
			if url != tc.wantURL || kind != tc.wantKind {
				t.Fatalf("primaryMedia = (%q, %q), want (%q, %q)", url, kind, tc.wantURL, tc.wantKind)
			}
		})
	}
}

func TestEnsureAbsoluteURL(t *testing.T) {
	cases := map[string]string{
		"":                                  "",
		"   ":                               "",
		"https://firsts3.ru/now.done/a.jpg": "https://firsts3.ru/now.done/a.jpg",
		"http://host/a.jpg":                 "http://host/a.jpg",
		"//firsts3.ru/now.done/a.jpg":       "https://firsts3.ru/now.done/a.jpg",
		"firsts3.ru/now.done/a.jpg":         "https://firsts3.ru/now.done/a.jpg",
		"/now.done/a.jpg":                   "https://now.done/a.jpg",
	}
	for in, want := range cases {
		if got := ensureAbsoluteURL(in); got != want {
			t.Errorf("ensureAbsoluteURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripReminderPhrase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Молоко, хлеб и напоминание на 12 часов", "Молоко, хлеб"},
		{"Купи молоко, напомни в 12:00", "Купи молоко"},
		{"напомни в 12:00", ""},
		{"Купить продукты с напоминанием завтра в 10 утра", "Купить продукты"},
		{"напоминание на пятницу 18:00", ""},
		{"Позвонить маме", "Позвонить маме"},
		{"", ""},
		{"Это напоминает мне о поездке", "Это напоминает мне о поездке"},
	}
	for _, c := range cases {
		if got := stripReminderPhrase(c.in); got != c.want {
			t.Errorf("stripReminderPhrase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestComposeNote(t *testing.T) {
	// Notes share marker / first-line parsing with tasks but never resolve a
	// date or reminder (composeNote does not call OpenAI or resolveSchedule).
	title, desc := composeNote("Заголовок: Идеи Описание: купить краски, кисти")
	if title != "Идеи" || desc != "купить краски, кисти" {
		t.Fatalf("markers: got (%q, %q)", title, desc)
	}

	title, desc = composeNote("Список покупок\nмолоко\nхлеб")
	if title != "Список покупок" || desc != "молоко\nхлеб" {
		t.Fatalf("first-line split: got (%q, %q)", title, desc)
	}
}

func TestParseModelReminder(t *testing.T) {
	loc := time.UTC
	future := time.Now().In(loc).Add(48 * time.Hour).Format("2006-01-02T15:04")
	past := time.Now().In(loc).Add(-48 * time.Hour).Format("2006-01-02T15:04")

	if got := parseModelReminder("", loc); got != nil {
		t.Errorf("empty -> %v, want nil", got)
	}
	if got := parseModelReminder("none", loc); got != nil {
		t.Errorf("none -> %v, want nil", got)
	}
	if got := parseModelReminder("not a date", loc); got != nil {
		t.Errorf("garbage -> %v, want nil", got)
	}
	if got := parseModelReminder(past, loc); got != nil {
		t.Errorf("past -> %v, want nil", got)
	}
	if got := parseModelReminder(future, loc); got == nil {
		t.Errorf("future -> nil, want a time")
	}
}
