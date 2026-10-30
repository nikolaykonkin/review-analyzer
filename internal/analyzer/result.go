// Пакет analyzer описывает результат анализа отзыва, промпт для языковой
// модели и проверку ее ответа
// Вызова самой модели в этом пакете нет
package analyzer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrInvalidResponse означает, что ответ модели не соответствует схеме
var ErrInvalidResponse = errors.New("невалидный ответ модели")

// Sentiment - тональность отзыва
type Sentiment string

const (
	SentimentPositive Sentiment = "positive"
	SentimentNeutral  Sentiment = "neutral"
	SentimentNegative Sentiment = "negative"
)

// Priority - приоритет отзыва для разбора
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
)

// maxSummaryRunes - максимальная длина summary в символах
const maxSummaryRunes = 300

// topics - закрытый список допустимых тем
// Тот же список с определениями перечислен в prompt.txt;
// расхождение ловит TestSystemPrompt_HasKeyMarkers
// Темы "соответствие описанию" (ожидание против реальности) и "подходит
// для типа кожи" (совместимость с типом кожи или волос) намеренно разные
var topics = []string{
	"эффект", "запах", "текстура", "цвет", "цена", "упаковка",
	"аллергия", "соответствие описанию", "качество",
	"подходит для типа кожи", "состав", "другое",
}

// Topics возвращает допустимые темы отзыва в порядке списка
func Topics() []string {
	return slices.Clone(topics)
}

// Result - проверенный результат анализа отзыва
type Result struct {
	Sentiment Sentiment
	Topics    []string
	Priority  Priority
	Summary   string
}

// rawResult - ответ модели до проверки
// Указатели нужны, чтобы отличить отсутствующее поле от пустого значения
type rawResult struct {
	Sentiment *string  `json:"sentiment"`
	Topics    []string `json:"topics"`
	Priority  *string  `json:"priority"`
	Summary   *string  `json:"summary"`
}

// Parse разбирает и проверяет ответ модели
// Допустим один JSON-объект с полями sentiment, topics, priority и summary;
// при желании он может быть обернут в блок кода из тройных кавычек
// Все остальное считается невалидным, и ошибка оборачивает ErrInvalidResponse
// Текст ответа в сообщения об ошибках не попадает: модель могла процитировать отзыв
func Parse(response string) (Result, error) {
	dec := json.NewDecoder(strings.NewReader(stripCodeFence(response)))
	dec.DisallowUnknownFields()

	var raw rawResult
	if err := dec.Decode(&raw); err != nil {
		return Result{}, fmt.Errorf("%w: разбор JSON: %w", ErrInvalidResponse, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Result{}, invalid("после JSON есть лишние данные")
	}
	return validate(raw)
}

// stripCodeFence снимает обертку из тройных кавычек вместе с необязательной
// меткой json на первой строке
// Если обертки нет, возвращает строку как есть
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 6 || !strings.HasPrefix(s, "```") || !strings.HasSuffix(s, "```") {
		return s
	}
	s = strings.TrimSuffix(strings.TrimPrefix(s, "```"), "```")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		if label := strings.TrimSpace(s[:i]); label == "" || strings.EqualFold(label, "json") {
			s = s[i+1:]
		}
	}
	return strings.TrimSpace(s)
}

func validate(raw rawResult) (Result, error) {
	if raw.Sentiment == nil {
		return Result{}, invalid("поле sentiment отсутствует")
	}
	sentiment := Sentiment(*raw.Sentiment)
	switch sentiment {
	case SentimentPositive, SentimentNeutral, SentimentNegative:
	default:
		return Result{}, invalid("sentiment не входит в допустимый список")
	}

	if len(raw.Topics) == 0 {
		return Result{}, invalid("topics: нужна хотя бы одна тема")
	}
	seen := make(map[string]bool, len(raw.Topics))
	for _, topic := range raw.Topics {
		if !slices.Contains(topics, topic) {
			return Result{}, invalid("topics: тема не входит в допустимый список")
		}
		if seen[topic] {
			return Result{}, invalid("topics: темы повторяются")
		}
		seen[topic] = true
	}

	if raw.Priority == nil {
		return Result{}, invalid("поле priority отсутствует")
	}
	priority := Priority(*raw.Priority)
	switch priority {
	case PriorityHigh, PriorityNormal:
	default:
		return Result{}, invalid("priority не входит в допустимый список")
	}
	if priority == PriorityHigh && sentiment != SentimentNegative {
		return Result{}, invalid("priority high допустим только при sentiment negative")
	}

	if raw.Summary == nil {
		return Result{}, invalid("поле summary отсутствует")
	}
	summary := strings.TrimSpace(*raw.Summary)
	if reason := checkSummary(summary); reason != "" {
		return Result{}, invalid(reason)
	}

	return Result{
		Sentiment: sentiment,
		Topics:    slices.Clone(raw.Topics),
		Priority:  priority,
		Summary:   summary,
	}, nil
}

// checkSummary возвращает причину, по которой summary недопустим, или пустую строку
func checkSummary(summary string) string {
	switch {
	case summary == "":
		return "summary пуст"
	case strings.ContainsAny(summary, "\x00\uFFFD"):
		return "summary содержит недопустимые символы"
	case strings.ContainsAny(summary, "\r\n"):
		return "summary должен быть без переносов строк"
	case utf8.RuneCountInString(summary) > maxSummaryRunes:
		return fmt.Sprintf("summary длиннее %d символов", maxSummaryRunes)
	case strings.IndexFunc(summary, isCyrillic) < 0:
		return "summary должен быть на русском языке"
	}
	return ""
}

func isCyrillic(r rune) bool {
	return unicode.Is(unicode.Cyrillic, r)
}

func invalid(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidResponse, reason)
}
