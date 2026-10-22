// Пакет logger создает настроенный slog.Logger
package logger

import (
	"io"
	"log/slog"
)

// New возвращает логгер, пишущий JSON в w с минимальным уровнем level
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
