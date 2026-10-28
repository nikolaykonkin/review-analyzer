// Пакет importer читает отзывы из CSV датасета Sephora, проверяет их,
// выбирает случайную выборку и загружает ее в хранилище
package importer

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

// ErrMissingColumn означает, что в заголовке CSV нет обязательной колонки
var ErrMissingColumn = errors.New("в заголовке CSV нет обязательной колонки")

// Reason - причина, по которой запись CSV не попала в импорт
type Reason string

const (
	ReasonMalformed    Reason = "malformed"
	ReasonEmptyProduct Reason = "empty_product_name"
	ReasonEmptyText    Reason = "empty_review_text"
	ReasonBadRating    Reason = "bad_rating"
	ReasonBadDate      Reason = "bad_date"
	ReasonInvalidUTF8  Reason = "invalid_utf8"
	ReasonNulByte      Reason = "nul_byte"
)

// Обязательные колонки CSV: они ищутся по имени, порядок колонок не важен
const (
	colAuthorID    = "author_id"
	colRating      = "rating"
	colSubmitted   = "submission_time"
	colReviewText  = "review_text"
	colProductID   = "product_id"
	colProductName = "product_name"
	colBrandName   = "brand_name"
)

var requiredColumns = []string{
	colAuthorID, colRating, colSubmitted, colReviewText,
	colProductID, colProductName, colBrandName,
}

// ctxCheckEvery - как часто (в записях) Read проверяет отмену контекста
const ctxCheckEvery = 1000

// Stats - счетчики чтения одного CSV-файла: Total прочитанных записей
// (без заголовка), Valid прошедших проверку и Skipped пропущенных по причинам
type Stats struct {
	Total   int
	Valid   int
	Skipped map[Reason]int
}

// Read читает CSV из r и вызывает handle для каждой валидной записи
// Некорректные записи пропускаются и учитываются в Stats
// Ошибкой заканчивается только чтение непригодного файла:
// нет обязательных колонок, сбой ввода-вывода или отмена контекста
func Read(ctx context.Context, r io.Reader, handle func(storage.NewReview)) (Stats, error) {
	stats := Stats{Skipped: make(map[Reason]int)}

	reader := csv.NewReader(r)
	reader.ReuseRecord = true

	header, err := reader.Read()
	if err != nil {
		return stats, fmt.Errorf("чтение заголовка CSV: %w", err)
	}
	columns, err := indexColumns(header)
	if err != nil {
		return stats, err
	}

	for {
		if stats.Total%ctxCheckEvery == 0 {
			if err := ctx.Err(); err != nil {
				return stats, err
			}
		}

		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return stats, nil
		}
		var parseErr *csv.ParseError
		if errors.As(err, &parseErr) {
			stats.Total++
			stats.Skipped[ReasonMalformed]++
			continue
		}
		if err != nil {
			return stats, fmt.Errorf("чтение CSV: %w", err)
		}

		stats.Total++
		review, reason := parseRecord(record, columns)
		if reason != "" {
			stats.Skipped[reason]++
			continue
		}
		stats.Valid++
		handle(review)
	}
}

// indexColumns строит соответствие "имя колонки -> номер" и проверяет,
// что все обязательные колонки на месте
// Безымянные колонки игнорируются
func indexColumns(header []string) (map[string]int, error) {
	columns := make(map[string]int, len(header))
	for i, name := range header {
		if i == 0 {
			name = strings.TrimPrefix(name, "\ufeff")
		}
		if name != "" {
			columns[name] = i
		}
	}

	var missing []string
	for _, name := range requiredColumns {
		if _, ok := columns[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrMissingColumn, strings.Join(missing, ", "))
	}
	return columns, nil
}

// parseRecord проверяет запись и превращает ее в отзыв
// Если запись непригодна, возвращается причина
func parseRecord(record []string, columns map[string]int) (storage.NewReview, Reason) {
	field := func(name string) string { return record[columns[name]] }

	productName := strings.TrimSpace(field(colProductName))
	if productName == "" {
		return storage.NewReview{}, ReasonEmptyProduct
	}
	reviewText := field(colReviewText)
	if strings.TrimSpace(reviewText) == "" {
		return storage.NewReview{}, ReasonEmptyText
	}
	brand := strings.TrimSpace(field(colBrandName))

	for _, s := range []string{productName, brand, reviewText} {
		if !utf8.ValidString(s) {
			return storage.NewReview{}, ReasonInvalidUTF8
		}
		if strings.ContainsRune(s, 0) {
			return storage.NewReview{}, ReasonNulByte
		}
	}

	rating, ok := parseRating(field(colRating))
	if !ok {
		return storage.NewReview{}, ReasonBadRating
	}
	reviewedAt, err := time.Parse("2006-01-02", strings.TrimSpace(field(colSubmitted)))
	if err != nil {
		return storage.NewReview{}, ReasonBadDate
	}

	var brandName *string
	if brand != "" {
		brandName = &brand
	}
	return storage.NewReview{
		SourceKey:   sourceKey(field(colAuthorID), field(colProductID), field(colSubmitted), reviewText),
		ProductName: productName,
		BrandName:   brandName,
		ReviewText:  reviewText,
		Rating:      rating,
		ReviewedAt:  reviewedAt,
	}, ""
}

// parseRating принимает целое число от 1 до 5, в том числе записанное как 5.0
func parseRating(s string) (int, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || value != math.Trunc(value) || value < 1 || value > 5 {
		return 0, false
	}
	return int(value), true
}

// sourceKey возвращает ключ дедупликации: SHA-256 (hex) от переданных полей
// Разделитель 0x1f исключает совпадение разных наборов полей при склейке
// Менять алгоритм нельзя: ключи уже загруженных отзывов перестанут совпадать,
// и повторный импорт создаст дубликаты
func sourceKey(authorID, productID, submittedAt, reviewText string) string {
	hash := sha256.New()
	for _, part := range []string{authorID, productID, submittedAt, reviewText} {
		hash.Write([]byte(part))
		hash.Write([]byte{0x1f})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
