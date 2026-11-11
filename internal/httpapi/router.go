// Пакет httpapi содержит HTTP-маршруты сервиса
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/nikolaykonkin/review-analyzer/internal/metrics"
)

// NewRouter возвращает маршрутизатор сервиса
func NewRouter(storage Storage, m *metrics.Metrics, log *slog.Logger) http.Handler {
	h := NewHandler(storage, log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /reviews/{id}", h.handleGetReview)
	mux.HandleFunc("GET /reviews/latest", h.handleGetLatestReview)
	mux.HandleFunc("GET /reviews/random", h.handleGetRandomReview)
	mux.HandleFunc("GET /stats", h.handleGetStats)

	// Эндпоинт для Prometheus
	mux.Handle("GET /metrics", promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{}))

	// Оборачиваем в middleware для сбора метрик
	return m.Middleware(mux)
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
