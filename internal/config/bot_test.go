package config

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestLoadBot_Success(t *testing.T) {
	env := map[string]string{
		"TELEGRAM_BOT_TOKEN":        "test-token",
		"TELEGRAM_ALLOWED_CHAT_IDS": "123, 456",
		"API_URL":                   "http://example.com",
	}
	cfg, err := LoadBot(fakeEnv(env))
	if err != nil {
		t.Fatalf("LoadBot: %v", err)
	}
	if cfg.Token != "test-token" {
		t.Errorf("Token = %q", cfg.Token)
	}
	if !slices.Equal(cfg.AllowedChatIDs, []int64{123, 456}) {
		t.Errorf("AllowedChatIDs = %v", cfg.AllowedChatIDs)
	}
	if cfg.APIURL != "http://example.com" {
		t.Errorf("APIURL = %q", cfg.APIURL)
	}
}

func TestLoadBot_Defaults(t *testing.T) {
	env := map[string]string{"TELEGRAM_BOT_TOKEN": "test-token"}
	cfg, err := LoadBot(fakeEnv(env))
	if err != nil {
		t.Fatalf("LoadBot: %v", err)
	}
	if cfg.APIURL != "http://localhost:8080" {
		t.Errorf("APIURL = %q, ожидалось http://localhost:8080", cfg.APIURL)
	}
	if len(cfg.AllowedChatIDs) != 0 {
		t.Errorf("AllowedChatIDs = %v, ожидался пустой", cfg.AllowedChatIDs)
	}
}

func TestLoadBot_MissingToken(t *testing.T) {
	_, err := LoadBot(fakeEnv(nil))
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("ожидалась ErrMissingValue, получено: %v", err)
	}
	if !strings.Contains(err.Error(), "TELEGRAM_BOT_TOKEN") {
		t.Errorf("в ошибке нет имени переменной: %v", err)
	}
}

func TestLoadBot_InvalidChatID(t *testing.T) {
	env := map[string]string{
		"TELEGRAM_BOT_TOKEN":        "test-token",
		"TELEGRAM_ALLOWED_CHAT_IDS": "123, abc",
	}
	_, err := LoadBot(fakeEnv(env))
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("ожидалась ErrInvalidValue, получено: %v", err)
	}
	if !strings.Contains(err.Error(), "abc") {
		t.Errorf("в ошибке нет невалидного значения: %v", err)
	}
}
