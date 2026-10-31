package analyzer

import (
	_ "embed"
	"strings"
)

const (
	reviewOpen  = "<<<REVIEW>>>"
	reviewClose = "<<<END_REVIEW>>>"
)

//go:embed prompt.txt
var systemPrompt string

// SystemPrompt возвращает инструкцию для модели (содержимое prompt.txt)
func SystemPrompt() string {
	return systemPrompt
}

// UserMessage оборачивает текст отзыва в разделители, которые упомянуты в промпте
// Последовательности разделителей внутри самого текста удаляются до тех пор,
// пока они не перестанут появляться (в том числе после склейки остатков),
// поэтому отзыв не может сам "закрыть" блок данных
// Это снижает риск prompt injection, но не исключает его
func UserMessage(reviewText string) string {
	stripper := strings.NewReplacer(reviewOpen, "", reviewClose, "")

	text := reviewText
	for {
		cleaned := stripper.Replace(text)
		if cleaned == text {
			break
		}
		text = cleaned
	}
	return reviewOpen + "\n" + text + "\n" + reviewClose
}
