package importer

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

const testHeader = ",author_id,rating,is_recommended,helpfulness,total_feedback_count," +
	"total_neg_feedback_count,total_pos_feedback_count,submission_time,review_text,review_title," +
	"skin_tone,eye_color,skin_type,hair_color,product_id,product_name,brand_name,price_usd\n"

// row строит строку CSV в формате датасета
// Поля, не важные для импорта, заполнены заглушками
// Текст передается уже в виде CSV-поля
func row(authorID, rating, date, text, productID, product, brand string) string {
	return strings.Join([]string{
		"0", authorID, rating, "1.0", "1.0", "0", "0", "0", date, text, "title",
		"", "", "", "", productID, product, brand, "10.0",
	}, ",") + "\n"
}

func readAll(t *testing.T, data string) ([]storage.NewReview, Stats, error) {
	t.Helper()
	var got []storage.NewReview
	stats, err := Read(context.Background(), strings.NewReader(data), func(r storage.NewReview) {
		got = append(got, r)
	})
	return got, stats, err
}

// readSampleFile читает демонстрационный файл из каталога testdata
func readSampleFile(t *testing.T) ([]storage.NewReview, Stats) {
	t.Helper()
	f, err := os.Open("../../testdata/sample_reviews.csv")
	if err != nil {
		t.Fatalf("открытие демонстрационного файла: %v", err)
	}
	defer f.Close()

	var got []storage.NewReview
	stats, err := Read(context.Background(), f, func(r storage.NewReview) {
		got = append(got, r)
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return got, stats
}

func TestRead_ValidRecord(t *testing.T) {
	got, stats, err := readAll(t, testHeader+row("42", "5", "2023-03-21", "Great mask", "P1", "Lip Mask", "LANEIGE"))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if stats.Total != 1 || stats.Valid != 1 || len(got) != 1 {
		t.Fatalf("stats = %+v, отзывов %d, ожидалась одна валидная запись", stats, len(got))
	}

	review := got[0]
	if review.ProductName != "Lip Mask" || review.ReviewText != "Great mask" || review.Rating != 5 {
		t.Errorf("поля прочитаны неверно: %+v", review)
	}
	if review.BrandName == nil || *review.BrandName != "LANEIGE" {
		t.Errorf("BrandName = %v, ожидалось LANEIGE", review.BrandName)
	}
	if date := review.ReviewedAt.Format("2006-01-02"); date != "2023-03-21" {
		t.Errorf("ReviewedAt = %s, ожидалось 2023-03-21", date)
	}
	if len(review.SourceKey) != 64 {
		t.Errorf("длина SourceKey = %d, ожидалось 64 (hex SHA-256)", len(review.SourceKey))
	}
}

func TestRead_EmptyBrandBecomesNil(t *testing.T) {
	got, _, err := readAll(t, testHeader+row("1", "4", "2023-01-01", "Nice", "P1", "Balm", "  "))
	if err != nil || len(got) != 1 {
		t.Fatalf("Read: err = %v, отзывов %d", err, len(got))
	}
	if got[0].BrandName != nil {
		t.Errorf("BrandName = %q, ожидался nil", *got[0].BrandName)
	}
}

func TestRead_AcceptsRatingWrittenAsFloat(t *testing.T) {
	got, _, err := readAll(t, testHeader+row("1", "5.0", "2023-01-01", "Nice", "P1", "Balm", "B"))
	if err != nil || len(got) != 1 || got[0].Rating != 5 {
		t.Fatalf("Read: err = %v, отзывы = %+v", err, got)
	}
}

func TestRead_SkipsInvalidRecords(t *testing.T) {
	tests := []struct {
		name   string
		row    string
		reason Reason
	}{
		{"пустой продукт", row("1", "5", "2023-01-01", "text", "P1", "  ", "B"), ReasonEmptyProduct},
		{"пустой текст", row("1", "5", "2023-01-01", "", "P1", "Product", "B"), ReasonEmptyText},
		{"текст из пробелов", row("1", "5", "2023-01-01", "   ", "P1", "Product", "B"), ReasonEmptyText},
		{"рейтинг не число", row("1", "abc", "2023-01-01", "text", "P1", "Product", "B"), ReasonBadRating},
		{"рейтинг дробный", row("1", "4.5", "2023-01-01", "text", "P1", "Product", "B"), ReasonBadRating},
		{"рейтинг ноль", row("1", "0", "2023-01-01", "text", "P1", "Product", "B"), ReasonBadRating},
		{"рейтинг шесть", row("1", "6", "2023-01-01", "text", "P1", "Product", "B"), ReasonBadRating},
		{"дата в другом формате", row("1", "5", "21/03/2023", "text", "P1", "Product", "B"), ReasonBadDate},
		{"пустая дата", row("1", "5", "", "text", "P1", "Product", "B"), ReasonBadDate},
		{"нулевой байт", row("1", "5", "2023-01-01", "bad\x00text", "P1", "Product", "B"), ReasonNulByte},
		{"невалидный UTF-8", row("1", "5", "2023-01-01", "bad\xffbyte", "P1", "Product", "B"), ReasonInvalidUTF8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, stats, err := readAll(t, testHeader+tt.row)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("невалидная запись попала в результат: %+v", got)
			}
			if stats.Total != 1 || stats.Valid != 0 || stats.Skipped[tt.reason] != 1 {
				t.Errorf("stats = %+v, ожидалась одна пропущенная запись с причиной %q", stats, tt.reason)
			}
		})
	}
}

func TestRead_MalformedRecordIsSkippedAndReadingContinues(t *testing.T) {
	data := testHeader +
		"0,1,5\n" + // слишком мало полей
		row("2", "5", "2023-01-01", "Fine", "P1", "Product", "B")

	got, stats, err := readAll(t, data)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("отзывов %d, ожидался один", len(got))
	}
	if stats.Total != 2 || stats.Skipped[ReasonMalformed] != 1 {
		t.Errorf("stats = %+v, ожидались 2 записи и одна malformed", stats)
	}
}

func TestRead_MultilineQuotedText(t *testing.T) {
	text := "\"line one\nline two, with \"\"quotes\"\"\""
	got, _, err := readAll(t, testHeader+row("1", "5", "2023-01-01", text, "P1", "Product", "B"))
	if err != nil || len(got) != 1 {
		t.Fatalf("Read: err = %v, отзывов %d", err, len(got))
	}
	want := "line one\nline two, with \"quotes\""
	if got[0].ReviewText != want {
		t.Errorf("ReviewText = %q, ожидалось %q", got[0].ReviewText, want)
	}
}

func TestRead_MissingColumn(t *testing.T) {
	header := strings.Replace(testHeader, "review_text", "text", 1)

	_, _, err := readAll(t, header+row("1", "5", "2023-01-01", "text", "P1", "Product", "B"))
	if !errors.Is(err, ErrMissingColumn) {
		t.Fatalf("ожидалась ErrMissingColumn, получено: %v", err)
	}
	if !strings.Contains(err.Error(), "review_text") {
		t.Errorf("в тексте ошибки нет имени колонки: %v", err)
	}
}

func TestRead_SourceKey(t *testing.T) {
	read := func(authorID, text string) string {
		t.Helper()
		got, _, err := readAll(t, testHeader+row(authorID, "5", "2023-01-01", text, "P1", "Product", "B"))
		if err != nil || len(got) != 1 {
			t.Fatalf("Read: err = %v, отзывов %d", err, len(got))
		}
		return got[0].SourceKey
	}

	first := read("1", "same text")
	again := read("1", "same text")
	otherAuthor := read("2", "same text")
	otherText := read("1", "other text")

	if first != again {
		t.Error("одинаковые записи дали разные ключи")
	}
	if first == otherAuthor {
		t.Error("разные авторы дали одинаковые ключи")
	}
	if first == otherText {
		t.Error("разные тексты дали одинаковые ключи")
	}
}

func TestRead_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	data := testHeader + row("1", "5", "2023-01-01", "text", "P1", "Product", "B")
	_, err := Read(ctx, strings.NewReader(data), func(storage.NewReview) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ожидалась context.Canceled, получено: %v", err)
	}
}

func TestRead_SampleFile(t *testing.T) {
	got, stats := readSampleFile(t)

	if stats.Total != 10 || stats.Valid != 10 || len(stats.Skipped) != 0 {
		t.Fatalf("stats = %+v, ожидалось 10 валидных записей без пропусков", stats)
	}

	var multiline, withoutBrand, negative int
	for _, review := range got {
		if strings.Contains(review.ReviewText, "\n") {
			multiline++
		}
		if review.BrandName == nil {
			withoutBrand++
		}
		if review.Rating <= 2 {
			negative++
		}
	}
	if multiline != 1 || withoutBrand != 1 || negative != 3 {
		t.Errorf("многострочных %d, без бренда %d, негативных %d; ожидалось 1, 1 и 3", multiline, withoutBrand, negative)
	}
}
