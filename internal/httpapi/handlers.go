package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

// Storage - интерфейс для работы с базой, нужен для тестируемости
type Storage interface {
	GetReview(ctx context.Context, id int64) (storage.Review, error)
	GetLatestReview(ctx context.Context) (storage.Review, error)
	GetRandomReview(ctx context.Context) (storage.Review, error)
	CountByStatus(ctx context.Context) (map[storage.Status]int, error)
}

// Handler содержит зависимости для HTTP-обработчиков
type Handler struct {
	storage Storage
	log     *slog.Logger
}

// NewHandler создает Handler
func NewHandler(storage Storage, log *slog.Logger) *Handler {
	return &Handler{
		storage: storage,
		log:     log,
	}
}

// handleGetReview обрабатывает GET /reviews/{id}
func (h *Handler) handleGetReview(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	rev, err := h.storage.GetReview(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.log.Error("получение отзыва", slog.Int64("id", id), slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, newReviewResponse(rev))
}

// handleGetLatestReview обрабатывает GET /reviews/latest
func (h *Handler) handleGetLatestReview(w http.ResponseWriter, r *http.Request) {
	rev, err := h.storage.GetLatestReview(r.Context())
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.log.Error("получение последнего отзыва", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, newReviewResponse(rev))
}

// handleGetRandomReview обрабатывает GET /reviews/random
func (h *Handler) handleGetRandomReview(w http.ResponseWriter, r *http.Request) {
	rev, err := h.storage.GetRandomReview(r.Context())
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.log.Error("получение случайного отзыва", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, newReviewResponse(rev))
}

// handleGetStats обрабатывает GET /stats
func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	counts, err := h.storage.CountByStatus(r.Context())
	if err != nil {
		h.log.Error("подсчет статистики", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := statsResponse{
		Pending:    counts[storage.StatusPending],
		Processing: counts[storage.StatusProcessing],
		Done:       counts[storage.StatusDone],
		Failed:     counts[storage.StatusFailed],
		Total:      counts[storage.StatusPending] + counts[storage.StatusProcessing] + counts[storage.StatusDone] + counts[storage.StatusFailed],
	}

	h.writeJSON(w, resp)
}

// writeJSON сериализует v в JSON и пишет в w
func (h *Handler) writeJSON(w http.ResponseWriter, v any) {
	// Сначала сериализуем в память
	// Если json.Marshal вернет ошибку, мы еще можем отдать клиенту корректный статус 500
	data, err := json.Marshal(v)
	if err != nil {
		h.log.Error("сериализация ответа", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Только после успешной сериализации отправляем заголовки
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
