package bot

import (
	"strings"
	"testing"

	"github.com/nikolaykonkin/review-analyzer/internal/apiclient"
)

func TestFormatReview_EscapesHTML(t *testing.T) {
	summary := "<b>Очень</b> хороший & крем."
	brand := "Brand & Co"
	r := &apiclient.Review{
		ID:          1,
		ProductName: "<script>alert(1)</script>",
		BrandName:   &brand,
		ReviewText:  "<i>Отзыв</i>",
		Rating:      5,
		Status:      "done",
		Summary:     &summary,
	}

	text := FormatReview(r)

	if strings.Contains(text, "<script>") {
		t.Error("не экранирован script в product_name")
	}
	if !strings.Contains(text, "&lt;script&gt;") {
		t.Error("product_name не превращен в html entities")
	}
	if strings.Contains(text, "<b>Очень</b>") {
		t.Error("не экранирован summary")
	}
	if !strings.Contains(text, "Brand &amp; Co") {
		t.Error("не экранирован brand_name")
	}
	if strings.Contains(text, "<i>Отзыв</i>") {
		t.Error("не экранирован текст отзыва")
	}
}

func TestFormatReview_Pending(t *testing.T) {
	r := &apiclient.Review{
		ID:          2,
		ProductName: "Крем",
		Rating:      4,
		Status:      "pending",
	}
	text := FormatReview(r)
	if !strings.Contains(text, "Отзыв еще не обработан") {
		t.Error("нет сообщения о том, что отзыв не обработан")
	}
}

func TestFormatReview_Failed(t *testing.T) {
	r := &apiclient.Review{
		ID:     3,
		Status: "failed",
	}
	text := FormatReview(r)
	if !strings.Contains(text, "Анализ не удался") {
		t.Error("нет сообщения об ошибке анализа")
	}
}

func TestFormatStats(t *testing.T) {
	s := &apiclient.Stats{Total: 100, Pending: 10, Processing: 2, Done: 80, Failed: 8}
	text := FormatStats(s)

	if !strings.Contains(text, "Всего: 100") {
		t.Error("нет общего количества")
	}
	if !strings.Contains(text, "Ошибки (failed): 8") {
		t.Error("нет количества ошибок")
	}
}
