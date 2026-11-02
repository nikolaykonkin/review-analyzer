// Пакет config загружает конфигурацию приложения из переменных окружения
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
)

const (
	envDatabaseURL = "DATABASE_URL"
	envHTTPPort    = "HTTP_PORT"
	envLogLevel    = "LOG_LEVEL"

	envGigaChatCredentials = "GIGACHAT_CREDENTIALS"
	envGigaChatScope       = "GIGACHAT_SCOPE"
	envGigaChatModel       = "GIGACHAT_MODEL"
	envGigaChatCAFile      = "GIGACHAT_CA_FILE"

	defaultHTTPPort = "8080"
	defaultLogLevel = "info"

	defaultGigaChatScope = "GIGACHAT_API_PERS"
	defaultGigaChatModel = "GigaChat"
)

// gigaChatScopes - допустимые значения GIGACHAT_SCOPE (версия API GigaChat)
var gigaChatScopes = []string{"GIGACHAT_API_PERS", "GIGACHAT_API_B2B", "GIGACHAT_API_CORP"}

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
	GigaChat    GigaChat
}

// GigaChat - настройки доступа к GigaChat API
// Все они необязательны: пока Credentials пуст, анализ отзывов не настроен
type GigaChat struct {
	// Credentials - ключ авторизации из личного кабинета; секрет, в ошибки и логи не попадает
	Credentials string
	// Scope - версия API: GIGACHAT_API_PERS, GIGACHAT_API_B2B или GIGACHAT_API_CORP
	Scope string
	// Model - название модели, которая обрабатывает отзывы
	Model string
	// CAFile - путь к файлу корневого сертификата НУЦ Минцифры; пусто - только системное хранилище
	CAFile string
}

// Enabled сообщает, задан ли ключ авторизации GigaChat
func (g GigaChat) Enabled() bool {
	return g.Credentials != ""
}

// Load читает конфигурацию через getenv (в приложении это os.Getenv)
// Пустая переменная считается незаданной: для необязательных подставляется значение по умолчанию
// Все найденные ошибки возвращаются разом через errors.Join, чтобы исправить окружение за один запуск
// Значения HTTP_PORT, LOG_LEVEL и GIGACHAT_SCOPE не засекречены и попадают в текст ошибки;
// DATABASE_URL и GIGACHAT_CREDENTIALS - секреты, их значения в ошибки не попадают никогда
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

	scope := valueOrDefault(strings.TrimSpace(getenv(envGigaChatScope)), defaultGigaChatScope)
	if !slices.Contains(gigaChatScopes, scope) {
		errs = append(errs, fmt.Errorf("%s: %w: %q (допустимо: %s)",
			envGigaChatScope, ErrInvalidValue, scope, strings.Join(gigaChatScopes, ", ")))
	}

	if joined := errors.Join(errs...); joined != nil {
		return Config{}, joined
	}
	return Config{
		DatabaseURL: databaseURL,
		HTTPPort:    port,
		LogLevel:    level,
		GigaChat: GigaChat{
			Credentials: strings.TrimSpace(getenv(envGigaChatCredentials)),
			Scope:       scope,
			Model:       valueOrDefault(strings.TrimSpace(getenv(envGigaChatModel)), defaultGigaChatModel),
			CAFile:      strings.TrimSpace(getenv(envGigaChatCAFile)),
		},
	}, nil
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
