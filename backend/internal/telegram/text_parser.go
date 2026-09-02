package telegram

import (
	"regexp"
	"strings"
)

// markedFields is the result of scanning a text (or transcribed voice) message
// for explicit «Заголовок»/«Описание»-style markers that split a task/note into
// a title and a description.
type markedFields struct {
	title       string
	description string
	hasTitle    bool // a title marker («Заголовок», «Тема», «с заголовком», …) was present
	hasDesc     bool // a description marker («Описание», «Детали», «с описанием», …) was present
}

// found reports whether a usable split was extracted. When false the caller
// keeps its existing behaviour (first line = title / OpenAI extraction).
func (m markedFields) found() bool {
	return (m.hasTitle || m.hasDesc) &&
		(strings.TrimSpace(m.title) != "" || strings.TrimSpace(m.description) != "")
}

// Marker words (lower-cased), nominative + instrumental case so both "заголовок
// …" and "с заголовком …" are recognised. None contain «ё», so RE2's (?i) case
// folding is enough (it does not fold ё↔е).
var (
	titleMarkerWords = map[string]bool{
		"заголовок": true, "заголовком": true,
		"название": true, "названием": true,
		"тема": true, "темой": true,
	}
	descMarkerWords = map[string]bool{
		"описание": true, "описанием": true,
		"подробности": true, "подробностями": true,
		"детали": true, "деталями": true,
	}
)

// fieldMarkerRe matches one field marker plus its separator, anywhere in the
// message (case-insensitive). Whisper transcripts rarely contain a colon, so a
// bare "заголовок купить … описание молоко" must work too — hence the separator
// accepts a plain space, not only ":".
//
// A leading boundary (start of string or a non-letter/digit rune) plus a
// mandatory trailing separator keep "тема" from matching inside "тематика" and
// "система". The trade-off: a normal phrase that literally contains "детали " or
// "тема " will be treated as a marker — acceptable, these are explicit keywords.
var fieldMarkerRe = regexp.MustCompile(
	`(?i)(?:^|[^\p{L}\p{N}])` +
		`(?:(?:с|со)[ \t]+)?` + // optional preposition, e.g. "с заголовком"
		`(заголовком|заголовок|названием|название|темой|тема|описанием|описание|подробностями|подробности|деталями|детали)` +
		`(?:[ \t]*[:,\-–—][ \t]*|[ \t]+|$)`,
)

// parseMarkedFields extracts the title/description a user split by hand with
// «Заголовок»/«Описание» markers (typed or dictated).
//
// Rules:
//   - A marker's value runs from just after its separator to the next marker of
//     any kind, or to the end of the message.
//   - Only the first marker of each kind is honoured.
//   - Description marker but no title marker: the text before the first marker
//     becomes the title; if there is none, the description text itself is used
//     as the title and the description is left empty.
func parseMarkedFields(msg string) markedFields {
	msg = strings.TrimSpace(msg)

	locs := fieldMarkerRe.FindAllStringSubmatchIndex(msg, -1)
	if len(locs) == 0 {
		return markedFields{}
	}

	var out markedFields
	firstMarkerStart := -1

	for i, loc := range locs {
		// loc layout: [0:1] full match (leading boundary rune + optional "с "
		// preposition + marker word + separator), [2:3] the marker word itself.
		word := strings.ToLower(msg[loc[2]:loc[3]])
		if firstMarkerStart == -1 {
			// Everything before the whole match (boundary rune / preposition
			// excluded) is the implicit title when only a description is marked.
			firstMarkerStart = loc[0]
		}

		// Value: from the end of "<boundary><word><sep>" to the start of the next
		// marker match (its leading boundary rune, thus excluded here), or to the
		// end of the message.
		valueStart := loc[1]
		valueEnd := len(msg)
		if i+1 < len(locs) {
			valueEnd = locs[i+1][0]
		}
		value := strings.TrimSpace(msg[valueStart:valueEnd])

		switch {
		case titleMarkerWords[word] && !out.hasTitle:
			out.title, out.hasTitle = value, true
		case descMarkerWords[word] && !out.hasDesc:
			out.description, out.hasDesc = value, true
		}
	}

	// Description given without an explicit title.
	if out.hasDesc && !out.hasTitle {
		if prefix := strings.TrimSpace(msg[:firstMarkerStart]); prefix != "" {
			out.title = prefix
		} else {
			out.title, out.description = out.description, ""
		}
	}

	return out
}

// reminderPhraseRe matches a clause that asks for a reminder / states an alarm
// time so it can be cut from a task description once the reminder time itself
// has been extracted (by OpenAI or parseModelReminder). It covers, anywhere in
// the text and case-insensitively:
//
//   - "напомни / напомните / напомнить [мне] [в|на] 15:00", "напомни завтра в 10 утра"
//   - "напоминание / с напоминанием / поставь напоминание на пятницу 18:00"
//
// The match starts at an optional leading connector (" и", ",", ";", " с",
// " а также") so "молоко, хлеб и напоминание на 12 часов" collapses cleanly to
// "молоко, хлеб", and runs to the end of the clause (next "." / ";" / newline or
// end of string). A phrase that merely mentions reminders in passing
// ("напоминает мне о лете") is not matched — only the imperative "напомни*" and
// the noun "напоминани*" forms are.
var reminderPhraseRe = regexp.MustCompile(
	`(?i)(?:\s+и|\s*[,;]|\s+с|\s+а\s+также)?\s*` +
		`(?:напомни(?:те|ть)?|напоминани\p{L}*)[^.;\n]*`,
)

// stripReminderPhrase removes a "напомни в 15:00" / "с напоминанием …" clause
// from text so it is not duplicated in a task description after the reminder
// time has been pulled out separately. Surrounding punctuation/whitespace is
// trimmed; the result may be "" when the description was only about the reminder.
func stripReminderPhrase(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	cleaned := reminderPhraseRe.ReplaceAllString(s, "")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Trim(cleaned, " \t,.;:-–—")
	return strings.TrimSpace(cleaned)
}
