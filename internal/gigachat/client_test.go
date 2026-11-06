package gigachat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type mockTokenSource struct {
	token     string
	callCount int32
}

func (m *mockTokenSource) Token(ctx context.Context) (string, error) {
	atomic.AddInt32(&m.callCount, 1)
	return m.token, nil
}

func (m *mockTokenSource) Invalidate() {
	m.token = "new-token" // Симулируем, что сервер выдал новый токен
}

func TestChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-1" {
			t.Errorf("ожидался токен token-1, получен %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("неверный Content-Type: %q", r.Header.Get("Content-Type"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	ts := &mockTokenSource{token: "token-1"}
	c := &Client{
		http:     http.DefaultClient,
		endpoint: server.URL,
		ts:       ts,
		model:    "GigaChat",
	}

	res, err := c.Chat(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if res != "ok" {
		t.Errorf("ответ = %q, ожидался %q", res, "ok")
	}
	if atomic.LoadInt32(&ts.callCount) != 1 {
		t.Errorf("вызовов Token: %d, ожидался 1", ts.callCount)
	}
}

func TestChat_RetryOn401(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n == 1 {
			// Первый запрос возвращает 401
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Второй запрос должен прийти с новым токеном
		if r.Header.Get("Authorization") != "Bearer new-token" {
			t.Errorf("ожидался обновленный токен new-token, получен %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	ts := &mockTokenSource{token: "token-1"}
	c := &Client{
		http:     http.DefaultClient,
		endpoint: server.URL,
		ts:       ts,
		model:    "GigaChat",
	}

	res, err := c.Chat(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if res != "ok" {
		t.Errorf("ответ = %q", res)
	}
	if atomic.LoadInt32(&requests) != 2 {
		t.Errorf("запросов к серверу: %d, ожидалось 2", requests)
	}
	if atomic.LoadInt32(&ts.callCount) != 2 {
		t.Errorf("вызовов Token: %d, ожидалось 2", ts.callCount)
	}
}

func TestChat_ErrorHasNoPayload(t *testing.T) {
	const secret = "SECRET-PAYLOAD"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Некорректный JSON с секретными данными
		_, _ = w.Write([]byte(`{"error":"` + secret + `"}`))
	}))
	defer server.Close()

	ts := &mockTokenSource{token: "token-1"}
	c := &Client{
		http:     http.DefaultClient,
		endpoint: server.URL,
		ts:       ts,
		model:    "GigaChat",
	}

	_, err := c.Chat(context.Background(), "sys", "user")
	if !errors.Is(err, ErrChatResponse) {
		t.Fatalf("ожидалась ErrChatResponse, получено: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("payload попал в ошибку: %v", err)
	}
}
