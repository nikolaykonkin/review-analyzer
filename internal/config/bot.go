package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	envTelegramToken   = "TELEGRAM_BOT_TOKEN"
	envTelegramChatIDs = "TELEGRAM_ALLOWED_CHAT_IDS"
	envAPIURL          = "API_URL"

	defaultAPIURL = "http://localhost:8080"
)

// BotConfig - конфигурация для Telegram-бота
type BotConfig struct {
	Token          string
	AllowedChatIDs []int64
	APIURL         string
}

// LoadBot читает конфигурацию бота из переменных окружения
// TELEGRAM_BOT_TOKEN обязателен
// Если TELEGRAM_ALLOWED_CHAT_IDS пуст, бот доступен всем
// API_URL по умолчанию http://localhost:8080
func LoadBot(getenv func(string) string) (BotConfig, error) {
	var errs []error

	token := getenv(envTelegramToken)
	if token == "" {
		errs = append(errs, fmt.Errorf("%s: %w", envTelegramToken, ErrMissingValue))
	}

	apiURL := getenv(envAPIURL)
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	var allowedIDs []int64
	idsStr := getenv(envTelegramChatIDs)
	if idsStr != "" {
		parts := strings.Split(idsStr, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			id, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w: %q", envTelegramChatIDs, ErrInvalidValue, p))
			} else {
				allowedIDs = append(allowedIDs, id)
			}
		}
	}

	if joined := errors.Join(errs...); joined != nil {
		return BotConfig{}, joined
	}

	return BotConfig{
		Token:          token,
		AllowedChatIDs: allowedIDs,
		APIURL:         apiURL,
	}, nil
}
