package gigachat

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewHTTPClient_EmptyCA(t *testing.T) {
	client, err := newHTTPClient("")
	if err != nil {
		t.Fatalf("newHTTPClient: %v", err)
	}
	if client == nil {
		t.Fatal("клиент не создан")
	}
}

func TestNewHTTPClient_InvalidCAFile(t *testing.T) {
	_, err := newHTTPClient("/nonexistent/path/ca.pem")
	if !errors.Is(err, ErrCAFile) {
		t.Fatalf("ожидалась ErrCAFile, получено: %v", err)
	}
}

func TestNewHTTPClient_InvalidPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(path, []byte("not a pem"), 0600); err != nil {
		t.Fatalf("создание файла: %v", err)
	}

	_, err := newHTTPClient(path)
	if !errors.Is(err, ErrCAFile) {
		t.Fatalf("ожидалась ErrCAFile, получено: %v", err)
	}
}
