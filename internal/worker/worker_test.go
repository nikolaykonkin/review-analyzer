package worker

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nikolaykonkin/review-analyzer/internal/analyzer"
	"github.com/nikolaykonkin/review-analyzer/internal/metrics"
	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

// mockStorage реализует интерфейс Storage для тестов
type mockStorage struct {
	mu          sync.Mutex
	reviews     []storage.Review
	doneIDs     []int64
	failIDs     []int64
	resetCalled bool
}

func (m *mockStorage) FetchPendingReviews(ctx context.Context, limit int) ([]storage.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.reviews) == 0 {
		return nil, nil
	}
	n := limit
	if n > len(m.reviews) {
		n = len(m.reviews)
	}
	taken := m.reviews[:n]
	m.reviews = m.reviews[n:]
	return taken, nil
}

func (m *mockStorage) UpdateReviewDone(ctx context.Context, id int64, result analyzer.Result) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.doneIDs = append(m.doneIDs, id)
	return nil
}

func (m *mockStorage) UpdateReviewFailed(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failIDs = append(m.failIDs, id)
	return nil
}

func (m *mockStorage) ResetProcessingReviews(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetCalled = true
	return nil
}

// mockAnalyzer реализует интерфейс Analyzer
type mockAnalyzer struct {
	err error
}

func (m *mockAnalyzer) Analyze(ctx context.Context, reviewText string) (analyzer.Result, error) {
	if m.err != nil {
		return analyzer.Result{}, m.err
	}
	return analyzer.Result{Sentiment: analyzer.SentimentPositive}, nil
}

func TestWorker_ProcessesAndFails(t *testing.T) {
	store := &mockStorage{
		reviews: []storage.Review{
			{ID: 1, ReviewText: "ok"},
			{ID: 2, ReviewText: "bad"},
		},
	}
	// Один анализатор обрабатывает оба сценария: для "bad" возвращает ошибку, для остальных - успех
	// Так в одном тесте проверяются и UpdateReviewDone, и UpdateReviewFailed
	an := &mockAnalyzerWithLogic{}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := New(store, an, metrics.New(), log)
	w.interval = 10 * time.Millisecond // Ускоряем для теста

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	w.Run(ctx)

	if !store.resetCalled {
		t.Error("ResetProcessingReviews не был вызван")
	}
	if len(store.doneIDs) != 1 || store.doneIDs[0] != 1 {
		t.Errorf("doneIDs = %v, ожидался [1]", store.doneIDs)
	}
	if len(store.failIDs) != 1 || store.failIDs[0] != 2 {
		t.Errorf("failIDs = %v, ожидался [2]", store.failIDs)
	}
}

type mockAnalyzerWithLogic struct{}

func (m *mockAnalyzerWithLogic) Analyze(ctx context.Context, reviewText string) (analyzer.Result, error) {
	if reviewText == "bad" {
		return analyzer.Result{}, errors.New("fake error")
	}
	return analyzer.Result{Sentiment: analyzer.SentimentPositive}, nil
}
