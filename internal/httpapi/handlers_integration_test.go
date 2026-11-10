package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nikolaykonkin/review-analyzer/internal/httpapi"
	"github.com/nikolaykonkin/review-analyzer/internal/migrate"
	"github.com/nikolaykonkin/review-analyzer/internal/storage"
	"github.com/nikolaykonkin/review-analyzer/internal/testutil"
	"github.com/nikolaykonkin/review-analyzer/migrations"
)

// TestGetReview_Integration проверяет полный путь:
// HTTP-запрос -> хендлер -> реальный Repository -> реальная база
// Пропускается без TEST_DATABASE_URL
func TestGetReview_Integration(t *testing.T) {
	pool := testutil.NewPool(t)
	ctx := context.Background()

	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatalf("загрузка миграций: %v", err)
	}
	if _, err := migrate.Up(ctx, pool, ms); err != nil {
		t.Fatalf("применение миграций: %v", err)
	}

	repo := storage.New(pool)
	rev := storage.NewReview{
		SourceKey:   "httpapi-integration-key",
		ProductName: "Интеграционный крем",
		ReviewText:  "Отличный крем",
		Rating:      5,
		ReviewedAt:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	id, inserted, err := repo.InsertReview(ctx, rev)
	if err != nil {
		t.Fatalf("вставка отзыва: %v", err)
	}
	if !inserted {
		t.Fatalf("отзыв не вставлен, id = %d", id)
	}

	// Вставляем второй отзыв, чтобы /reviews/latest вернул именно его
	rev2 := rev
	rev2.SourceKey = "httpapi-integration-key-2"
	id2, _, _ := repo.InsertReview(ctx, rev2)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpapi.NewRouter(repo, log)

	// GET /reviews/{id}
	url := fmt.Sprintf("/reviews/%d", id)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /reviews/{id}: код = %d, ожидалось %d", rec.Code, http.StatusOK)
	}
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("GET /reviews/{id}: ответ не JSON: %v", err)
	}
	if got["product_name"] != "Интеграционный крем" {
		t.Errorf("GET /reviews/{id}: product_name = %v", got["product_name"])
	}

	// GET /reviews/latest
	req = httptest.NewRequest(http.MethodGet, "/reviews/latest", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /reviews/latest: код = %d", rec.Code)
	}
	var latest map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&latest)
	if latest["id"] != float64(id2) {
		t.Errorf("GET /reviews/latest: id = %v, ожидался %d", latest["id"], id2)
	}

	// GET /stats
	req = httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /stats: код = %d", rec.Code)
	}

	var stats struct {
		Pending int `json:"pending"`
		Total   int `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("GET /stats: ответ не JSON: %v", err)
	}
	if stats.Total != 2 || stats.Pending != 2 {
		t.Errorf("GET /stats: %+v, ожидалось total=2, pending=2", stats)
	}
}
