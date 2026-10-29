package importer

import (
	"context"
	"fmt"

	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

// Inserter - то, куда загружаются отзывы: в приложении это *storage.Repository
type Inserter interface {
	InsertReview(ctx context.Context, review storage.NewReview) (int64, bool, error)
}

// Result - итог загрузки выборки: Inserted новых отзывов и Duplicates уже существовавших
type Result struct {
	Inserted   int
	Duplicates int
}

// Insert загружает отзывы по одному
// Повторная загрузка тех же отзывов безопасна: уже существующие считаются дубликатами
// При ошибке возвращается результат по уже обработанным отзывам
func Insert(ctx context.Context, store Inserter, reviews []storage.NewReview) (Result, error) {
	var result Result
	for i, review := range reviews {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		_, inserted, err := store.InsertReview(ctx, review)
		if err != nil {
			return result, fmt.Errorf("загрузка отзыва %d из %d: %w", i+1, len(reviews), err)
		}
		if inserted {
			result.Inserted++
		} else {
			result.Duplicates++
		}
	}
	return result, nil
}
