package gigachat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestToken_Success(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("RqUID") == "" {
			t.Error("заголовок RqUID не передан")
		}
		if r.Header.Get("Authorization") != "Basic test-creds" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("разбор формы: %v", err)
		}
		if r.Form.Get("scope") != "GIGACHAT_API_PERS" {
			t.Errorf("scope = %q", r.Form.Get("scope"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"token-123"}`))
	}))
	defer server.Close()

	ts := &tokenSource{
		http:     http.DefaultClient,
		endpoint: server.URL,
		creds:    "test-creds",
		scope:    "GIGACHAT_API_PERS",
		now:      time.Now,
		rquidGen: newRqUID,
	}

	token, err := ts.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "token-123" {
		t.Errorf("token = %q", token)
	}
	if requests.Load() != 1 {
		t.Errorf("запросов: %d, ожидалось 1", requests.Load())
	}
}

func TestToken_CachesAndRefreshes(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		w.WriteHeader(http.StatusOK)
		// Возвращаем токен вида "token-1", "token-2" и т.д.
		_, _ = fmt.Fprintf(w, `{"access_token":"token-%d"}`, n)
	}))
	defer server.Close()

	now := time.Now()
	ts := &tokenSource{
		http:     http.DefaultClient,
		endpoint: server.URL,
		creds:    "test-creds",
		scope:    "GIGACHAT_API_PERS",
		now:      func() time.Time { return now },
		rquidGen: newRqUID,
	}

	// Первый запрос
	token1, err := ts.Token(context.Background())
	if err != nil {
		t.Fatalf("первый Token: %v", err)
	}
	if token1 != "token-1" {
		t.Errorf("первый токен = %q, ожидался token-1", token1)
	}

	// Второй запрос - из кэша
	token2, err := ts.Token(context.Background())
	if err != nil {
		t.Fatalf("второй Token: %v", err)
	}
	if token2 != "token-1" {
		t.Errorf("второй токен = %q, ожидался token-1 (из кэша)", token2)
	}
	if requests.Load() != 1 {
		t.Errorf("после кэша запросов: %d, ожидалось 1", requests.Load())
	}

	// Прокручиваем время за TTL
	now = now.Add(tokenTTL + time.Minute)
	token3, err := ts.Token(context.Background())
	if err != nil {
		t.Fatalf("третий Token: %v", err)
	}
	if token3 != "token-2" {
		t.Errorf("третий токен = %q, ожидался token-2", token3)
	}
	if requests.Load() != 2 {
		t.Errorf("после истечения запросов: %d, ожидалось 2", requests.Load())
	}
}

func TestToken_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	ts := &tokenSource{
		http:     http.DefaultClient,
		endpoint: server.URL,
		creds:    "bad",
		scope:    "GIGACHAT_API_PERS",
		now:      time.Now,
		rquidGen: newRqUID,
	}

	_, err := ts.Token(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("ожидалась ErrUnauthorized, получено: %v", err)
	}
}

func TestToken_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{bad-json}`))
	}))
	defer server.Close()

	ts := &tokenSource{
		http:     http.DefaultClient,
		endpoint: server.URL,
		creds:    "test-creds",
		scope:    "GIGACHAT_API_PERS",
		now:      time.Now,
		rquidGen: newRqUID,
	}

	_, err := ts.Token(context.Background())
	if !errors.Is(err, ErrTokenResponse) {
		t.Fatalf("ожидалась ErrTokenResponse, получено: %v", err)
	}
	if strings.Contains(err.Error(), "bad-json") {
		t.Errorf("тело ответа попало в ошибку: %v", err)
	}
}
