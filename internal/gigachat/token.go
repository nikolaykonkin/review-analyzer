package gigachat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	tokenEndpoint = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	// Токен живет 30 минут: отнимаем минуту, чтобы обновить его заранее
	tokenTTL = 29 * time.Minute
)

// TokenSource - интерфейс для получения токена
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// tokenSource реализует TokenSource с кэшированием
type tokenSource struct {
	mu        sync.Mutex
	http      *http.Client
	endpoint  string
	creds     string
	scope     string
	token     string
	expiresAt time.Time

	// Для тестирования:
	now      func() time.Time
	rquidGen func() (string, error)
}

// NewTokenSource создает источник токена
// Проверяет CA-файл при создании (fail-fast)
func NewTokenSource(creds, scope, caFile string) (TokenSource, error) {
	client, err := newHTTPClient(caFile)
	if err != nil {
		return nil, err
	}
	return &tokenSource{
		http:     client,
		endpoint: tokenEndpoint,
		creds:    creds,
		scope:    scope,
		now:      time.Now,
		rquidGen: newRqUID,
	}, nil
}

// Token возвращает кэшированный токен или запрашивает новый
func (s *tokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && s.now().Before(s.expiresAt) {
		return s.token, nil
	}

	// Запрос токена
	token, err := s.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	s.token = token
	s.expiresAt = s.now().Add(tokenTTL)
	return token, nil
}

// fetchToken делает запрос к OAuth-серверу
func (s *tokenSource) fetchToken(ctx context.Context) (string, error) {
	rquid, err := s.rquidGen()
	if err != nil {
		return "", err
	}

	data := url.Values{}
	data.Set("scope", s.scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("создание запроса токена: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RqUID", rquid)
	req.Header.Set("Authorization", "Basic "+s.creds)

	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("запрос токена: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: статус %d", ErrTokenResponse, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: чтение ответа: %w", ErrTokenResponse, err)
	}

	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("%w: разбор JSON: %w", ErrTokenResponse, err)
	}
	if res.AccessToken == "" {
		return "", fmt.Errorf("%w: пустой access_token", ErrTokenResponse)
	}

	return res.AccessToken, nil
}
