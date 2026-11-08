package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

// mockStorage реализует интерфейс Storage для тестов
type mockStorage struct {
	rev storage.Review
	err error
}

func (m *mockStorage) GetReview(ctx context.Context, id int64) (storage.Review, error) {
	return m.rev, m.err
}

func newTestRouter(store Storage) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(store, log)
}

func TestGetReview_Success(t *testing.T) {
	summary := "Хороший крем."
	store := &mockStorage{
		rev: storage.Review{
			ID:          1,
			ProductName: "Крем",
			ReviewText:  "Отличный крем",
			Rating:      5,
			Status:      storage.StatusDone,
			Summary:     &summary,
		},
	}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/reviews/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d, ожидалось %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, ожидался application/json", contentType)
	}

	// Проверяем JSON-контракт
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("ответ не JSON: %v", err)
	}
	if got["id"] != float64(1) {
		t.Errorf("id = %v, ожидалось 1", got["id"])
	}
	if got["status"] != "done" {
		t.Errorf("status = %v, ожидалось done", got["status"])
	}
	if _, ok := got["brand_name"]; ok {
		t.Error("brand_name должен отсутствовать при nil (omitempty)")
	}
	if topics, ok := got["topics"].([]any); !ok || len(topics) != 0 {
		t.Errorf("topics = %v, ожидался пустой массив", got["topics"])
	}
	if got["summary"] != summary {
		t.Errorf("summary = %v, ожидалось %q", got["summary"], summary)
	}
}

func TestGetReview_InvalidID(t *testing.T) {
	router := newTestRouter(&mockStorage{})

	req := httptest.NewRequest(http.MethodGet, "/reviews/abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("код ответа = %d, ожидалось %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetReview_NotFound(t *testing.T) {
	store := &mockStorage{err: storage.ErrNotFound}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/reviews/999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("код ответа = %d, ожидалось %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetReview_InternalError(t *testing.T) {
	store := &mockStorage{err: errors.New("db error")}
	router := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/reviews/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("код ответа = %d, ожидалось %d", rec.Code, http.StatusInternalServerError)
	}
	// Проверяем, что текст ошибки БД не попал в ответ
	if rec.Body.String() == "db error" {
		t.Error("внутренняя ошибка попала в ответ клиенту")
	}
}
