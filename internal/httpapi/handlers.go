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
		// Детали ошибки пишем в лог, клиенту отдаем общий текст
		h.log.Error("получение отзыва", slog.Int64("id", id), slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(newReviewResponse(rev)); err != nil {
		h.log.Error("кодирование ответа", slog.Int64("id", id), slog.Any("error", err))
	}
}
