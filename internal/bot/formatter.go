package bot

import (
	"fmt"
	"html"
	"strings"

	"github.com/nikolaykonkin/review-analyzer/internal/apiclient"
)

// FormatReview собирает HTML-сообщение для отзыва
// Все динамические данные экранируются, чтобы не сломать разметку Telegram
func FormatReview(r *apiclient.Review) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("<b>Отзыв #%d</b>\n\n", r.ID))
	b.WriteString(fmt.Sprintf("<b>Продукт:</b> %s\n", html.EscapeString(r.ProductName)))
	if r.BrandName != nil && *r.BrandName != "" {
		b.WriteString(fmt.Sprintf("<b>Бренд:</b> %s\n", html.EscapeString(*r.BrandName)))
	}
	b.WriteString(fmt.Sprintf("<b>Рейтинг:</b> %d/5\n", r.Rating))

	switch r.Status {
	case "done":
		b.WriteString("\n<b>Анализ:</b>\n")
		if r.Sentiment != nil {
			b.WriteString(fmt.Sprintf("Тональность: <i>%s</i>\n", html.EscapeString(*r.Sentiment)))
		}
		if len(r.Topics) > 0 {
			b.WriteString(fmt.Sprintf("Темы: <i>%s</i>\n", html.EscapeString(strings.Join(r.Topics, ", "))))
		}
		if r.Priority != nil {
			b.WriteString(fmt.Sprintf("Приоритет: <i>%s</i>\n", html.EscapeString(*r.Priority)))
		}
		if r.Summary != nil {
			b.WriteString(fmt.Sprintf("Кратко: %s\n", html.EscapeString(*r.Summary)))
		}
	case "failed":
		b.WriteString("\n<i>Анализ не удался.</i>\n")
	default:
		b.WriteString("\n<i>Отзыв еще не обработан.</i>\n")
	}

	b.WriteString("\n<b>Текст:</b>\n")
	b.WriteString(html.EscapeString(r.ReviewText))
	return b.String()
}

// FormatStats собирает HTML-сообщение для статистики
func FormatStats(s *apiclient.Stats) string {
	return fmt.Sprintf("<b>Статистика отзывов</b>\n\n"+
		"Всего: %d\n"+
		"Ожидают (pending): %d\n"+
		"В обработке (processing): %d\n"+
		"Готово (done): %d\n"+
		"Ошибки (failed): %d",
		s.Total, s.Pending, s.Processing, s.Done, s.Failed)
}
