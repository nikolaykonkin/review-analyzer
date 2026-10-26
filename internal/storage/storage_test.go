package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nikolaykonkin/review-analyzer/internal/migrate"
	"github.com/nikolaykonkin/review-analyzer/internal/testutil"
	"github.com/nikolaykonkin/review-analyzer/migrations"
)

// newTestRepo создает репозиторий во временной схеме с примененными миграциями
func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	pool := testutil.NewPool(t)

	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatalf("загрузка миграций: %v", err)
	}
	if _, err := migrate.Up(context.Background(), pool, ms); err != nil {
		t.Fatalf("применение миграций: %v", err)
	}
	return New(pool)
}

func sampleReview(key string) NewReview {
	brand := "LANEIGE"
	return NewReview{
		SourceKey:   key,
		ProductName: "Lip Sleeping Mask",
		BrandName:   &brand,
		ReviewText:  "Great lip mask",
		Rating:      5,
		ReviewedAt:  time.Date(2023, 3, 21, 0, 0, 0, 0, time.UTC),
	}
}

func TestInsertAndGetReview(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	id, inserted, err := repo.InsertReview(ctx, sampleReview("key-1"))
	if err != nil {
		t.Fatalf("InsertReview: %v", err)
	}
	if !inserted || id == 0 {
		t.Fatalf("inserted = %v, id = %d, ожидалась успешная вставка", inserted, id)
	}

	got, err := repo.GetReview(ctx, id)
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.ProductName != "Lip Sleeping Mask" || got.ReviewText != "Great lip mask" || got.Rating != 5 {
		t.Errorf("исходные поля прочитаны неверно: %+v", got)
	}
	if got.BrandName == nil || *got.BrandName != "LANEIGE" {
		t.Errorf("BrandName = %v, ожидалось LANEIGE", got.BrandName)
	}
	if date := got.ReviewedAt.Format("2006-01-02"); date != "2023-03-21" {
		t.Errorf("ReviewedAt = %s, ожидалось 2023-03-21", date)
	}
	if got.Status != StatusPending {
		t.Errorf("Status = %q, ожидалось %q", got.Status, StatusPending)
	}
	if got.Sentiment != nil || got.Priority != nil || got.Summary != nil {
		t.Errorf("результат анализа должен быть пустым: %+v", got)
	}
	if len(got.Topics) != 0 {
		t.Errorf("Topics = %v, ожидался пустой список", got.Topics)
	}
}

func TestInsertReview_NullBrand(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	review := sampleReview("key-null-brand")
	review.BrandName = nil

	id, _, err := repo.InsertReview(ctx, review)
	if err != nil {
		t.Fatalf("InsertReview: %v", err)
	}
	got, err := repo.GetReview(ctx, id)
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.BrandName != nil {
		t.Errorf("BrandName = %q, ожидался NULL", *got.BrandName)
	}
}

func TestInsertReview_DuplicateSourceKey(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if _, inserted, err := repo.InsertReview(ctx, sampleReview("dup")); err != nil || !inserted {
		t.Fatalf("первая вставка: inserted = %v, err = %v", inserted, err)
	}

	id, inserted, err := repo.InsertReview(ctx, sampleReview("dup"))
	if err != nil {
		t.Fatalf("повторная вставка вернула ошибку: %v", err)
	}
	if inserted || id != 0 {
		t.Errorf("повторная вставка: inserted = %v, id = %d, ожидалось false и 0", inserted, id)
	}
}

func TestInsertReview_EmptySourceKey(t *testing.T) {
	repo := newTestRepo(t)

	_, _, err := repo.InsertReview(context.Background(), sampleReview(""))
	if !errors.Is(err, ErrEmptySourceKey) {
		t.Fatalf("ожидалась ErrEmptySourceKey, получено: %v", err)
	}
}

func TestInsertReview_ErrorHasNoReviewText(t *testing.T) {
	repo := newTestRepo(t)

	review := sampleReview("bad-rating")
	review.Rating = 6
	review.ReviewText = "SECRET-REVIEW-TEXT"

	_, _, err := repo.InsertReview(context.Background(), review)
	if err == nil {
		t.Fatal("рейтинг 6 должен приводить к ошибке")
	}
	if strings.Contains(err.Error(), "SECRET-REVIEW-TEXT") {
		t.Errorf("текст отзыва попал в текст ошибки: %v", err)
	}
}

func TestGetReview_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	_, err := repo.GetReview(context.Background(), 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидалась ErrNotFound, получено: %v", err)
	}
}
