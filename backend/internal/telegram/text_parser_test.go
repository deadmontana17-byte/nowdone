package telegram

import "testing"

func TestParseMarkedFields(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantFound bool
		wantTitle string
		wantDesc  string
	}{
		{
			name:      "title and description markers, colon form",
			in:        "Заголовок: Купить продукты Описание: Молоко, хлеб, яйца",
			wantFound: true,
			wantTitle: "Купить продукты",
			wantDesc:  "Молоко, хлеб, яйца",
		},
		{
			name:      "title marker only",
			in:        "Заголовок: Позвонить маме",
			wantFound: true,
			wantTitle: "Позвонить маме",
			wantDesc:  "",
		},
		{
			name:      "description marker only, nothing before it -> text becomes the title",
			in:        "Описание: Подготовить отчёт к пятнице",
			wantFound: true,
			wantTitle: "Подготовить отчёт к пятнице",
			wantDesc:  "",
		},
		{
			name:      "description marker with leading text -> leading text is the title",
			in:        "Завтра встретиться с Петей в 10 утра Описание: Обсудить проект",
			wantFound: true,
			wantTitle: "Завтра встретиться с Петей в 10 утра",
			wantDesc:  "Обсудить проект",
		},
		{
			name:      "date after the description marker stays in the description (resolved separately)",
			in:        "Заголовок: Встреча Описание: Обсудить бюджет Завтра в 10:00",
			wantFound: true,
			wantTitle: "Встреча",
			wantDesc:  "Обсудить бюджет Завтра в 10:00",
		},
		{
			name:      "voice-style: no colons, markers mid-utterance",
			in:        "заголовок купить продукты описание молоко хлеб яйца",
			wantFound: true,
			wantTitle: "купить продукты",
			wantDesc:  "молоко хлеб яйца",
		},
		{
			name:      "voice-style: 'с заголовком' / 'с описанием' (instrumental case)",
			in:        "с заголовком позвонить в банк с описанием уточнить баланс",
			wantFound: true,
			wantTitle: "позвонить в банк",
			wantDesc:  "уточнить баланс",
		},
		{
			name:      "synonyms Тема / Детали, colon form",
			in:        "Тема: Ремонт Детали: Позвонить мастеру",
			wantFound: true,
			wantTitle: "Ремонт",
			wantDesc:  "Позвонить мастеру",
		},
		{
			name:      "case-insensitive",
			in:        "заголовок отчёт ОПИСАНИЕ за март",
			wantFound: true,
			wantTitle: "отчёт",
			wantDesc:  "за март",
		},
		{
			name:      "bare marker mid-sentence is now honoured (explicit keyword)",
			in:        "Уточнить детали проекта у заказчика",
			wantFound: true,
			wantTitle: "Уточнить",
			wantDesc:  "проекта у заказчика",
		},
		{
			name:      "no markers at all -> not found",
			in:        "Купить хлеб и молоко",
			wantFound: false,
		},
		{
			name:      "marker word inside another word is ignored",
			in:        "Продумать тематику презентации",
			wantFound: false,
		},
		{
			name:      "lone marker with no value -> not a usable split",
			in:        "детали",
			wantFound: false,
		},
		{
			name:      "order independent: description stated before title",
			in:        "Описание: молоко и хлеб Заголовок: Покупки",
			wantFound: true,
			wantTitle: "Покупки",
			wantDesc:  "молоко и хлеб",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMarkedFields(tc.in)
			if got.found() != tc.wantFound {
				t.Fatalf("found() = %v, want %v (got %+v)", got.found(), tc.wantFound, got)
			}
			if !tc.wantFound {
				return
			}
			if got.title != tc.wantTitle {
				t.Errorf("title = %q, want %q", got.title, tc.wantTitle)
			}
			if got.description != tc.wantDesc {
				t.Errorf("description = %q, want %q", got.description, tc.wantDesc)
			}
		})
	}
}
