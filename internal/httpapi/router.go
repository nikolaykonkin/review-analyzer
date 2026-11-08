// Пакет httpapi содержит HTTP-маршруты сервиса
package httpapi

import (
	"log/slog"
	"net/http"
)

// NewRouter возвращает маршрутизатор сервиса
func NewRouter(storage Storage, log *slog.Logger) http.Handler {
	h := NewHandler(storage, log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /reviews/{id}", h.handleGetReview)

	return mux
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
