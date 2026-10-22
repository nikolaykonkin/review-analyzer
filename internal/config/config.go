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
	envHTTPPort = "HTTP_PORT"
	envLogLevel = "LOG_LEVEL"

	defaultHTTPPort = "8080"
	defaultLogLevel = "info"
)

// ErrInvalidValue означает, что значение переменной окружения недопустимо
var ErrInvalidValue = errors.New("недопустимое значение")

// Config - конфигурация сервиса
type Config struct {
	HTTPPort int
	LogLevel slog.Level
}

// Load читает конфигурацию через getenv (в приложении это os.Getenv)
// Пустая переменная считается незаданной и заменяется значением по умолчанию
// Все найденные ошибки возвращаются разом через errors.Join, чтобы исправить окружение за один запуск
// Значения переменных этого пакета не засекречены, поэтому попадают в текст ошибки
func Load(getenv func(string) string) (Config, error) {
	var errs []error

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
	return Config{HTTPPort: port, LogLevel: level}, nil
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
