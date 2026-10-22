package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNew_WritesJSON(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelInfo)

	log.Info("проверка", slog.String("key", "value"))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("вывод не является JSON: %v: %q", err, buf.String())
	}
	if record["msg"] != "проверка" {
		t.Errorf("msg = %v, ожидалось %q", record["msg"], "проверка")
	}
	if record["level"] != "INFO" {
		t.Errorf("level = %v, ожидалось INFO", record["level"])
	}
	if record["key"] != "value" {
		t.Errorf("key = %v, ожидалось value", record["key"])
	}
}

func TestNew_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelWarn)

	log.Info("не должно попасть в вывод")
	if buf.Len() != 0 {
		t.Fatalf("сообщение уровня INFO записано при уровне WARN: %q", buf.String())
	}

	log.Warn("должно попасть в вывод")
	if buf.Len() == 0 {
		t.Fatal("сообщение уровня WARN не записано")
	}
}
