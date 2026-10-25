// Пакет config загружает конфигурацию приложения из переменных окружения
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

const (
	envDatabaseURL = "DATABASE_URL"
	envHTTPPort    = "HTTP_PORT"
	envLogLevel    = "LOG_LEVEL"

	defaultHTTPPort = "8080"
	defaultLogLevel = "info"
)

var (
	// ErrInvalidValue означает, что значение переменной окружения недопустимо
	ErrInvalidValue = errors.New("недопустимое значение")
	// ErrMissingValue означает, что обязательная переменная не задана
	ErrMissingValue = errors.New("обязательная переменная не задана")
)

// Config - конфигурация сервиса
type Config struct {
	DatabaseURL string
	HTTPPort    int
	LogLevel    slog.Level
}

// Load читает конфигурацию через getenv (в приложении это os.Getenv)
// Пустая переменная считается незаданной: для необязательных подставляется значение по умолчанию
// Все найденные ошибки возвращаются разом через errors.Join, чтобы исправить окружение за один запуск
// Значения HTTP_PORT и LOG_LEVEL не засекречены и попадают в текст ошибки;
// DATABASE_URL содержит пароль, поэтому его значение в ошибки не попадает никогда
func Load(getenv func(string) string) (Config, error) {
	var errs []error

	databaseURL := getenv(envDatabaseURL)
	if databaseURL == "" {
		errs = append(errs, fmt.Errorf("%s: %w", envDatabaseURL, ErrMissingValue))
	}

	port, err := parsePort(valueOrDefault(getenv(envHTTPPort), defaultHTTPPort))
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", envHTTPPort, err))
	}

	level, err := parseLogLevel(valueOrDefault(getenv(envLogLevel), defaultLogLevel))
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", envLogLevel, err))
	}

	if joined := errors.Join(errs...); joined != nil {
		return Config{}, joined
	}
	return Config{DatabaseURL: databaseURL, HTTPPort: port, LogLevel: level}, nil
}

func valueOrDefault(value, def string) string {
	if value == "" {
		return def
	}
	return value
}

func parsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%w: %q (нужно целое число от 1 до 65535)", ErrInvalidValue, s)
	}
	return port, nil
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: %q (допустимо: debug, info, warn, error)", ErrInvalidValue, s)
	}
}
