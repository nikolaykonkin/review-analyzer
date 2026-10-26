// Пакет storage отвечает за хранение отзывов в PostgreSQL
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound означает, что запись не найдена
	ErrNotFound = errors.New("запись не найдена")
	// ErrEmptySourceKey означает, что у отзыва не задан ключ дедупликации
	ErrEmptySourceKey = errors.New("пустой ключ дедупликации")
)

// Status - статус обработки отзыва. Значения совпадают с ограничением
// в migrations/001_create_reviews.sql
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// NewReview - исходные данные отзыва для вставки (результата анализа еще нет)
type NewReview struct {
	// SourceKey - ключ дедупликации: вставка с уже существующим ключом пропускается
	SourceKey   string
	ProductName string
	BrandName   *string
	ReviewText  string
	Rating      int
	ReviewedAt  time.Time
}

// Review - отзыв, как он хранится в базе
type Review struct {
	ID          int64
	ProductName string
	BrandName   *string
	ReviewText  string
	Rating      int
	ReviewedAt  time.Time
	Status      Status
	Sentiment   *string
	Topics      []string
	Priority    *string
	Summary     *string
	CreatedAt   time.Time
}

// Repository - доступ к таблице reviews
type Repository struct {
	pool *pgxpool.Pool
}

// New создает репозиторий поверх пула подключений
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const insertReviewSQL = `
INSERT INTO reviews (source_key, product_name, brand_name, review_text, rating, reviewed_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (source_key) DO NOTHING
RETURNING id`

// InsertReview добавляет отзыв и возвращает его id
// Если отзыв с таким SourceKey уже есть, вставка пропускается: возвращаются 0 и false без ошибки
func (r *Repository) InsertReview(ctx context.Context, review NewReview) (int64, bool, error) {
	if review.SourceKey == "" {
		return 0, false, ErrEmptySourceKey
	}

	var id int64
	err := r.pool.QueryRow(ctx, insertReviewSQL,
		review.SourceKey, review.ProductName, review.BrandName,
		review.ReviewText, review.Rating, review.ReviewedAt,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("вставка отзыва: %w", err)
	}
	return id, true, nil
}

const getReviewSQL = `
SELECT id, product_name, brand_name, review_text, rating, reviewed_at,
       status, sentiment, topics, priority, summary, created_at
FROM reviews
WHERE id = $1`

// GetReview возвращает отзыв по id или ErrNotFound
func (r *Repository) GetReview(ctx context.Context, id int64) (Review, error) {
	var (
		review Review
		status string
	)
	err := r.pool.QueryRow(ctx, getReviewSQL, id).Scan(
		&review.ID, &review.ProductName, &review.BrandName, &review.ReviewText,
		&review.Rating, &review.ReviewedAt, &status, &review.Sentiment,
		&review.Topics, &review.Priority, &review.Summary, &review.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, fmt.Errorf("отзыв %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Review{}, fmt.Errorf("чтение отзыва %d: %w", id, err)
	}
	review.Status = Status(status)
	return review, nil
}
