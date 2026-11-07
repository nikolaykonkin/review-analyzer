package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nikolaykonkin/review-analyzer/internal/analyzer"
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

func TestFetchPendingReviews(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// Вставляем 3 отзыва
	for i := 0; i < 3; i++ {
		_, _, _ = repo.InsertReview(ctx, sampleReview(fmt.Sprintf("key-%d", i)))
	}

	// Забираем 2
	reviews, err := repo.FetchPendingReviews(ctx, 2)
	if err != nil {
		t.Fatalf("FetchPendingReviews: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("захвачено %d отзывов, ожидалось 2", len(reviews))
	}

	// Проверяем, что в базе они стали processing
	for _, rev := range reviews {
		got, _ := repo.GetReview(ctx, rev.ID)
		if got.Status != StatusProcessing {
			t.Errorf("статус = %q, ожидался processing", got.Status)
		}
	}
}

func TestUpdateReviewDoneAndFailed(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	id1, _, _ := repo.InsertReview(ctx, sampleReview("done-key"))
	id2, _, _ := repo.InsertReview(ctx, sampleReview("failed-key"))

	// Захватываем, чтобы перевести в processing (иначе нельзя сделать done)
	_, _ = repo.FetchPendingReviews(ctx, 10)

	result := analyzer.Result{
		Sentiment: analyzer.SentimentPositive,
		Topics:    []string{"эффект"},
		Priority:  analyzer.PriorityNormal,
		Summary:   "Хороший крем.",
	}
	if err := repo.UpdateReviewDone(ctx, id1, result); err != nil {
		t.Fatalf("UpdateReviewDone: %v", err)
	}
	if err := repo.UpdateReviewFailed(ctx, id2); err != nil {
		t.Fatalf("UpdateReviewFailed: %v", err)
	}

	gotDone, _ := repo.GetReview(ctx, id1)
	if gotDone.Status != StatusDone || *gotDone.Sentiment != "positive" {
		t.Errorf("отзыв done заполнен неверно: %+v", gotDone)
	}

	gotFailed, _ := repo.GetReview(ctx, id2)
	if gotFailed.Status != StatusFailed {
		t.Errorf("статус = %q, ожидался failed", gotFailed.Status)
	}
}

func TestResetProcessingReviews(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, _, _ = repo.InsertReview(ctx, sampleReview("reset-key"))
	// Захватываем, чтобы перевести в processing
	_, _ = repo.FetchPendingReviews(ctx, 1)

	// Сбрасываем
	if err := repo.ResetProcessingReviews(ctx); err != nil {
		t.Fatalf("ResetProcessingReviews: %v", err)
	}

	reviews, _ := repo.FetchPendingReviews(ctx, 1)
	if len(reviews) != 1 {
		t.Errorf("после сброса не удалось снова захватить отзыв")
	}
}
