package analyzer

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

const baseSummary = "Покупательница получила сыпь и жалуется на сломанный дозатор."

// baseFields возвращает поля валидного ответа; тесты меняют их по одному
func baseFields() map[string]any {
	return map[string]any{
		"sentiment": "negative",
		"topics":    []string{"аллергия", "упаковка"},
		"priority":  "high",
		"summary":   baseSummary,
	}
}

func response(t *testing.T, fields map[string]any) string {
	t.Helper()
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("сборка JSON: %v", err)
	}
	return string(data)
}

func TestTopics_ExactList(t *testing.T) {
	want := []string{
		"эффект", "запах", "текстура", "цвет", "цена", "упаковка",
		"аллергия", "соответствие описанию", "качество",
		"подходит для типа кожи", "состав", "другое",
	}
	if got := Topics(); !slices.Equal(got, want) {
		t.Errorf("Topics() = %v, ожидалось %v", got, want)
	}
}

func TestTopics_ReturnsCopy(t *testing.T) {
	first := Topics()
	first[0] = "изменено"

	if got := Topics()[0]; got != "эффект" {
		t.Errorf("изменение результата Topics() повлияло на внутренний список: %q", got)
	}
}

func TestParse_Valid(t *testing.T) {
	got, err := Parse(response(t, baseFields()))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Sentiment != SentimentNegative || got.Priority != PriorityHigh {
		t.Errorf("Sentiment = %q, Priority = %q", got.Sentiment, got.Priority)
	}
	if len(got.Topics) != 2 || got.Topics[0] != "аллергия" || got.Topics[1] != "упаковка" {
		t.Errorf("Topics = %v", got.Topics)
	}
	if got.Summary != baseSummary {
		t.Errorf("Summary = %q", got.Summary)
	}
}

func TestParse_ValidVariants(t *testing.T) {
	tests := []struct {
		name   string
		modify func(f map[string]any)
	}{
		{"нейтральный", func(f map[string]any) { f["sentiment"], f["priority"] = "neutral", "normal" }},
		{"положительный", func(f map[string]any) { f["sentiment"], f["priority"] = "positive", "normal" }},
		{"негативный без высокого приоритета", func(f map[string]any) { f["priority"] = "normal" }},
		{"тема другое", func(f map[string]any) { f["topics"] = []string{"другое"} }},
		{"все темы сразу", func(f map[string]any) { f["topics"] = Topics() }},
		{"темы соответствие описанию и тип кожи вместе", func(f map[string]any) {
			f["topics"] = []string{"соответствие описанию", "подходит для типа кожи"}
		}},
		{"summary с латиницей и кириллицей", func(f map[string]any) { f["summary"] = "Крем Calm Water не подошел." }},
		{"summary с пробелами по краям", func(f map[string]any) { f["summary"] = "  Крем не подошел.  " }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := baseFields()
			tt.modify(fields)
			if _, err := Parse(response(t, fields)); err != nil {
				t.Errorf("Parse: %v", err)
			}
		})
	}
}

func TestParse_StripsCodeFence(t *testing.T) {
	body := response(t, baseFields())
	wrapped := []string{
		"```json\n" + body + "\n```",
		"```\n" + body + "\n```",
		"  ```JSON\n" + body + "\n```  \n",
		"```" + body + "```",
	}

	for _, text := range wrapped {
		if _, err := Parse(text); err != nil {
			t.Errorf("ответ в обертке не разобран: %v (%q)", err, text)
		}
	}
}

func TestParse_InvalidText(t *testing.T) {
	valid := response(t, baseFields())
	tests := []struct {
		name     string
		response string
	}{
		{"пустой ответ", ""},
		{"текст без JSON", "Конечно! Вот результат анализа."},
		{"текст перед JSON", "Вот результат: " + valid},
		{"текст после JSON", valid + " Надеюсь, это поможет."},
		{"два объекта подряд", valid + valid},
		{"массив вместо объекта", "[" + valid + "]"},
		{"оборванный JSON", valid[:len(valid)-5]},
		{"другие кавычки вместо тройных", "'''" + valid + "'''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.response); !errors.Is(err, ErrInvalidResponse) {
				t.Errorf("ожидалась ErrInvalidResponse, получено: %v", err)
			}
		})
	}
}

func TestParse_InvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		modify func(f map[string]any)
	}{
		{"нет sentiment", func(f map[string]any) { delete(f, "sentiment") }},
		{"sentiment null", func(f map[string]any) { f["sentiment"] = nil }},
		{"sentiment в другом регистре", func(f map[string]any) { f["sentiment"] = "Negative" }},
		{"неизвестный sentiment", func(f map[string]any) { f["sentiment"] = "mixed" }},
		{"нет topics", func(f map[string]any) { delete(f, "topics") }},
		{"пустой список тем", func(f map[string]any) { f["topics"] = []string{} }},
		{"темы не массив", func(f map[string]any) { f["topics"] = "аллергия" }},
		{"тема не из списка", func(f map[string]any) { f["topics"] = []string{"блеск"} }},
		{"аромат вместо запаха", func(f map[string]any) { f["topics"] = []string{"аромат"} }},
		{"оттенок вместо цвета", func(f map[string]any) { f["topics"] = []string{"оттенок"} }},
		{"тема в другом регистре", func(f map[string]any) { f["topics"] = []string{"Запах"} }},
		{"повтор темы", func(f map[string]any) { f["topics"] = []string{"запах", "запах"} }},
		{"нет priority", func(f map[string]any) { delete(f, "priority") }},
		{"неизвестный priority", func(f map[string]any) { f["priority"] = "urgent" }},
		{"high при positive", func(f map[string]any) { f["sentiment"] = "positive" }},
		{"high при neutral", func(f map[string]any) { f["sentiment"] = "neutral" }},
		{"нет summary", func(f map[string]any) { delete(f, "summary") }},
		{"пустой summary", func(f map[string]any) { f["summary"] = "" }},
		{"summary из пробелов", func(f map[string]any) { f["summary"] = "   " }},
		{"summary не строка", func(f map[string]any) { f["summary"] = 42 }},
		{"summary с переносом строки", func(f map[string]any) { f["summary"] = "Первая строка.\nВторая строка." }},
		{"summary без русских букв", func(f map[string]any) { f["summary"] = "Allergic reaction to the cream." }},
		{"summary длиннее 300 символов", func(f map[string]any) { f["summary"] = strings.Repeat("а", 301) }},
		{"summary с нулевым символом", func(f map[string]any) { f["summary"] = "Сыпь на щеках.\x00" }},
		{"неизвестное поле", func(f map[string]any) { f["confidence"] = 0.9 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := baseFields()
			tt.modify(fields)
			if _, err := Parse(response(t, fields)); !errors.Is(err, ErrInvalidResponse) {
				t.Errorf("ожидалась ErrInvalidResponse, получено: %v", err)
			}
		})
	}
}

func TestParse_ErrorDoesNotContainResponseText(t *testing.T) {
	const secret = "SECRET-REVIEW-TEXT"

	fields := baseFields()
	fields["sentiment"] = "mixed"
	fields["summary"] = secret + " и пересказ отзыва"

	responses := []string{response(t, fields), secret + " это не JSON"}
	for _, text := range responses {
		_, err := Parse(text)
		if err == nil {
			t.Fatalf("ожидалась ошибка для %q", text)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("текст ответа попал в текст ошибки: %v", err)
		}
	}
}
