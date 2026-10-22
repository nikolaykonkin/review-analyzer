package config

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func fakeEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(fakeEnv(nil))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, ожидалось 8080", cfg.HTTPPort)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, ожидалось %v", cfg.LogLevel, slog.LevelInfo)
	}
}

func TestLoad_ValidValues(t *testing.T) {
	cfg, err := Load(fakeEnv(map[string]string{
		"HTTP_PORT": "9090",
		"LOG_LEVEL": "DEBUG",
	}))
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

func TestLoad_Invalid(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"порт не число", map[string]string{"HTTP_PORT": "abc"}, []string{"HTTP_PORT"}},
		{"порт ноль", map[string]string{"HTTP_PORT": "0"}, []string{"HTTP_PORT"}},
		{"порт больше 65535", map[string]string{"HTTP_PORT": "65536"}, []string{"HTTP_PORT"}},
		{"неизвестный уровень", map[string]string{"LOG_LEVEL": "verbose"}, []string{"LOG_LEVEL"}},
		{
			"обе ошибки сразу",
			map[string]string{"HTTP_PORT": "abc", "LOG_LEVEL": "verbose"},
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
		})
	}
}
