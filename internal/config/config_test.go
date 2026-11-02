package config

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
)

const (
	testSecret      = "s3cr3t-pass"
	testDatabaseURL = "postgres://user:" + testSecret + "@localhost:5432/db"
	testGigaKey     = "giga-s3cr3t-key"
)

func fakeEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

// baseEnv возвращает минимально допустимое окружение, дополненное extra
func baseEnv(extra map[string]string) map[string]string {
	env := map[string]string{"DATABASE_URL": testDatabaseURL}
	for key, value := range extra {
		env[key] = value
	}
	return env
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(fakeEnv(baseEnv(nil)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cfg.DatabaseURL != testDatabaseURL {
		t.Errorf("DatabaseURL прочитан неверно")
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, ожидалось 8080", cfg.HTTPPort)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, ожидалось %v", cfg.LogLevel, slog.LevelInfo)
	}
}

func TestLoad_GigaChatDefaults(t *testing.T) {
	cfg, err := Load(fakeEnv(baseEnv(nil)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cfg.GigaChat.Enabled() || cfg.GigaChat.Credentials != "" {
		t.Errorf("без GIGACHAT_CREDENTIALS GigaChat не должен считаться настроенным")
	}
	if cfg.GigaChat.Scope != "GIGACHAT_API_PERS" {
		t.Errorf("Scope = %q, ожидалось GIGACHAT_API_PERS", cfg.GigaChat.Scope)
	}
	if cfg.GigaChat.Model != "GigaChat" {
		t.Errorf("Model = %q, ожидалось GigaChat", cfg.GigaChat.Model)
	}
	if cfg.GigaChat.CAFile != "" {
		t.Errorf("CAFile = %q, ожидалась пустая строка", cfg.GigaChat.CAFile)
	}
}

func TestLoad_GigaChatValues(t *testing.T) {
	cfg, err := Load(fakeEnv(baseEnv(map[string]string{
		"GIGACHAT_CREDENTIALS": "  " + testGigaKey + "\n",
		"GIGACHAT_SCOPE":       "GIGACHAT_API_CORP",
		"GIGACHAT_MODEL":       "GigaChat-2-Max",
		"GIGACHAT_CA_FILE":     "/certs/russian_trusted_root_ca_pem.crt",
	})))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !cfg.GigaChat.Enabled() || cfg.GigaChat.Credentials != testGigaKey {
		t.Errorf("Credentials прочитан неверно (пробелы по краям должны обрезаться)")
	}
	if cfg.GigaChat.Scope != "GIGACHAT_API_CORP" {
		t.Errorf("Scope = %q", cfg.GigaChat.Scope)
	}
	if cfg.GigaChat.Model != "GigaChat-2-Max" {
		t.Errorf("Model = %q", cfg.GigaChat.Model)
	}
	if cfg.GigaChat.CAFile != "/certs/russian_trusted_root_ca_pem.crt" {
		t.Errorf("CAFile = %q", cfg.GigaChat.CAFile)
	}
}

func TestLoad_GigaChatAllScopes(t *testing.T) {
	for _, scope := range []string{"GIGACHAT_API_PERS", "GIGACHAT_API_B2B", "GIGACHAT_API_CORP"} {
		cfg, err := Load(fakeEnv(baseEnv(map[string]string{"GIGACHAT_SCOPE": scope})))
		if err != nil {
			t.Errorf("scope %q: неожиданная ошибка: %v", scope, err)
			continue
		}
		if cfg.GigaChat.Scope != scope {
			t.Errorf("Scope = %q, ожидалось %q", cfg.GigaChat.Scope, scope)
		}
	}
}

func TestLoad_ValidValues(t *testing.T) {
	cfg, err := Load(fakeEnv(baseEnv(map[string]string{
		"HTTP_PORT": "9090",
		"LOG_LEVEL": "DEBUG",
	})))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d, ожидалось 9090", cfg.HTTPPort)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, ожидалось %v", cfg.LogLevel, slog.LevelDebug)
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	_, err := Load(fakeEnv(nil))
	if !errors.Is(err, ErrMissingValue) {
		t.Fatalf("ожидалась ошибка ErrMissingValue, получено: %v", err)
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("в тексте ошибки нет имени переменной: %v", err)
	}
}

func TestLoad_ReportsAllErrorsAtOnce(t *testing.T) {
	_, err := Load(fakeEnv(map[string]string{"HTTP_PORT": "abc"}))
	if !errors.Is(err, ErrMissingValue) {
		t.Errorf("нет ErrMissingValue: %v", err)
	}
	if !errors.Is(err, ErrInvalidValue) {
		t.Errorf("нет ErrInvalidValue: %v", err)
	}
}

func TestLoad_Invalid(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"порт не число", baseEnv(map[string]string{"HTTP_PORT": "abc"}), []string{"HTTP_PORT"}},
		{"порт ноль", baseEnv(map[string]string{"HTTP_PORT": "0"}), []string{"HTTP_PORT"}},
		{"порт больше 65535", baseEnv(map[string]string{"HTTP_PORT": "65536"}), []string{"HTTP_PORT"}},
		{"неизвестный уровень", baseEnv(map[string]string{"LOG_LEVEL": "verbose"}), []string{"LOG_LEVEL"}},
		{"неизвестный scope", baseEnv(map[string]string{"GIGACHAT_SCOPE": "GIGACHAT_API_FREE"}), []string{"GIGACHAT_SCOPE"}},
		{"scope в другом регистре", baseEnv(map[string]string{"GIGACHAT_SCOPE": "gigachat_api_pers"}), []string{"GIGACHAT_SCOPE"}},
		{
			"обе ошибки сразу",
			baseEnv(map[string]string{"HTTP_PORT": "abc", "LOG_LEVEL": "verbose"}),
			[]string{"HTTP_PORT", "LOG_LEVEL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(fakeEnv(tt.env))
			if !errors.Is(err, ErrInvalidValue) {
				t.Fatalf("ожидалась ошибка ErrInvalidValue, получено: %v", err)
			}
			for _, part := range tt.want {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("в тексте ошибки нет %q: %v", part, err)
				}
			}
			if strings.Contains(err.Error(), testSecret) {
				t.Errorf("секрет из DATABASE_URL попал в текст ошибки: %v", err)
			}
		})
	}
}

func TestLoad_GigaChatCredentialsNotInErrors(t *testing.T) {
	_, err := Load(fakeEnv(baseEnv(map[string]string{
		"GIGACHAT_CREDENTIALS": testGigaKey,
		"GIGACHAT_SCOPE":       "unknown",
		"HTTP_PORT":            "abc",
	})))
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("ожидалась ошибка ErrInvalidValue, получено: %v", err)
	}
	if strings.Contains(err.Error(), testGigaKey) {
		t.Errorf("ключ GigaChat попал в текст ошибки: %v", err)
	}
}
