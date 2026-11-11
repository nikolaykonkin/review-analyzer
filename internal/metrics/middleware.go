package metrics

import (
	"net/http"
	"strconv"
	"time"
)

// responseWriter перехватывает статус-код ответа
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Middleware возвращает http.Handler, который собирает метрики запросов
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Пропускаем сбор метрик для самого эндпоинта /metrics, чтобы не искажать статистику
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()

		// В Go 1.22+ ServeMux заполняет r.Pattern после маршрутизации
		// Если Pattern пуст (например, 404), используем "not_found"
		route := r.Pattern
		if route == "" {
			route = "not_found"
		}

		m.HTTPRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rw.status)).Inc()
		m.HTTPRequestDuration.WithLabelValues(r.Method, route).Observe(duration)
	})
}
