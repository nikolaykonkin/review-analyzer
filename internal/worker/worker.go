// Пакет worker содержит фоновую обработку отзывов через GigaChat
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nikolaykonkin/review-analyzer/internal/analyzer"
	"github.com/nikolaykonkin/review-analyzer/internal/gigachat"
	"github.com/nikolaykonkin/review-analyzer/internal/metrics"
	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

const (
	defaultBatchSize   = 10
	defaultConcurrency = 3
	defaultInterval    = 5 * time.Second
	analyzeTimeout     = 2 * time.Minute
)

// Storage - интерфейс для работы с базой, нужен для тестируемости
type Storage interface {
	FetchPendingReviews(ctx context.Context, limit int) ([]storage.Review, error)
	UpdateReviewDone(ctx context.Context, id int64, result analyzer.Result) error
	UpdateReviewFailed(ctx context.Context, id int64) error
	ResetProcessingReviews(ctx context.Context) error
}

// Analyzer - интерфейс для анализа текста отзыва
type Analyzer interface {
	Analyze(ctx context.Context, reviewText string) (analyzer.Result, error)
}

// Worker - фоновый обработчик отзывов
type Worker struct {
	storage     Storage
	analyzer    Analyzer
	metrics     *metrics.Metrics
	log         *slog.Logger
	batchSize   int
	concurrency int
	interval    time.Duration
}

// New создает воркер
func New(storage Storage, analyzer Analyzer, m *metrics.Metrics, log *slog.Logger) *Worker {
	return &Worker{
		storage:     storage,
		analyzer:    analyzer,
		metrics:     m,
		log:         log,
		batchSize:   defaultBatchSize,
		concurrency: defaultConcurrency,
		interval:    defaultInterval,
	}
}

// Run запускает пул воркеров
// Блокируется до отмены контекста
func (w *Worker) Run(ctx context.Context) {
	// Сбрасываем застрявшие отзывы при старте
	if err := w.storage.ResetProcessingReviews(ctx); err != nil {
		w.log.Error("сброс застрявших отзывов", slog.Any("error", err))
	}

	jobs := make(chan storage.Review, w.batchSize)
	var wg sync.WaitGroup

	// Запускаем пул обработчиков
	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.workerLoop(ctx, jobs)
		}()
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			w.log.Info("воркер остановлен")
			return
		case <-ticker.C:
			reviews, err := w.storage.FetchPendingReviews(ctx, w.batchSize)
			if err != nil {
				w.log.Error("захват отзывов", slog.Any("error", err))
				continue
			}
			for _, rev := range reviews {
				select {
				case jobs <- rev:
				case <-ctx.Done():
					close(jobs)
					wg.Wait()
					return
				}
			}
		}
	}
}

// workerLoop читает задачи из канала и обрабатывает их
func (w *Worker) workerLoop(ctx context.Context, jobs <-chan storage.Review) {
	for rev := range jobs {
		w.processJob(ctx, rev)
	}
}

// processJob обрабатывает один отзыв с таймаутом
func (w *Worker) processJob(ctx context.Context, rev storage.Review) {
	analyzeCtx, cancel := context.WithTimeout(ctx, analyzeTimeout)
	defer cancel()

	start := time.Now()
	result, err := w.analyzer.Analyze(analyzeCtx, rev.ReviewText)
	duration := time.Since(start).Seconds()

	// Записываем метрику длительности запроса к GigaChat
	w.metrics.GigaChatDuration.Observe(duration)

	// Создаем контекст для обновления статуса в базе
	// context.WithoutCancel гарантирует, что обновление пройдет,
	// даже если основной контекст отменен (например, при остановке сервиса)
	dbCtx, dbCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer dbCancel()

	if err != nil {
		w.log.Error("анализ отзыва", slog.Int64("review_id", rev.ID), slog.Any("error", err))
		w.metrics.GigaChatProcessedTotal.WithLabelValues("failed").Inc()
		if err := w.storage.UpdateReviewFailed(dbCtx, rev.ID); err != nil {
			w.log.Error("обновление статуса failed", slog.Int64("review_id", rev.ID), slog.Any("error", err))
		}
		return
	}

	w.metrics.GigaChatProcessedTotal.WithLabelValues("done").Inc()
	if err := w.storage.UpdateReviewDone(dbCtx, rev.ID, result); err != nil {
		w.log.Error("обновление статуса done", slog.Int64("review_id", rev.ID), slog.Any("error", err))
		return
	}
	w.log.Info("отзыв обработан", slog.Int64("review_id", rev.ID), slog.String("sentiment", string(result.Sentiment)))
}

// GigaChatAnalyzer склеивает клиент GigaChat и парсер ответа
type GigaChatAnalyzer struct {
	client *gigachat.Client
}

// NewGigaChatAnalyzer создает анализатор
func NewGigaChatAnalyzer(client *gigachat.Client) *GigaChatAnalyzer {
	return &GigaChatAnalyzer{client: client}
}

// Analyze отправляет текст в GigaChat и валидирует ответ
func (a *GigaChatAnalyzer) Analyze(ctx context.Context, reviewText string) (analyzer.Result, error) {
	prompt := analyzer.SystemPrompt()
	msg := analyzer.UserMessage(reviewText)

	resp, err := a.client.Chat(ctx, prompt, msg)
	if err != nil {
		return analyzer.Result{}, fmt.Errorf("чат: %w", err)
	}

	result, err := analyzer.Parse(resp)
	if err != nil {
		return analyzer.Result{}, fmt.Errorf("разбор ответа: %w", err)
	}
	return result, nil
}
