// Пакет apiclient реализует HTTP-клиент к API сервиса review-analyzer
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	// ErrNotFound означает, что ресурс не найден (404)
	ErrNotFound = errors.New("ресурс не найден")
	// ErrAPI означает ошибку на стороне API (не 200 и не 404)
	ErrAPI = errors.New("ошибка API")
)

// Review - структура отзыва, возвращаемая API
type Review struct {
	ID          int64    `json:"id"`
	ProductName string   `json:"product_name"`
	BrandName   *string  `json:"brand_name,omitempty"`
	ReviewText  string   `json:"review_text"`
	Rating      int      `json:"rating"`
	Status      string   `json:"status"`
	Sentiment   *string  `json:"sentiment,omitempty"`
	Topics      []string `json:"topics"`
	Priority    *string  `json:"priority,omitempty"`
	Summary     *string  `json:"summary,omitempty"`
}

// Stats - статистика по статусам отзывов
type Stats struct {
	Pending    int `json:"pending"`
	Processing int `json:"processing"`
	Done       int `json:"done"`
	Failed     int `json:"failed"`
	Total      int `json:"total"`
}

// Client - клиент для взаимодействия с API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New создает клиент
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("создание запроса: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("запрос: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: статус %d", ErrAPI, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("чтение ответа: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("разбор JSON: %w", err)
	}
	return nil
}

// GetReview возвращает отзыв по ID
func (c *Client) GetReview(ctx context.Context, id int64) (*Review, error) {
	var rev Review
	path := fmt.Sprintf("/reviews/%d", id)
	if err := c.getJSON(ctx, path, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

// GetLatestReview возвращает последний отзыв
func (c *Client) GetLatestReview(ctx context.Context) (*Review, error) {
	var rev Review
	if err := c.getJSON(ctx, "/reviews/latest", &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

// GetRandomReview возвращает случайный отзыв
func (c *Client) GetRandomReview(ctx context.Context) (*Review, error) {
	var rev Review
	if err := c.getJSON(ctx, "/reviews/random", &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

// GetStats возвращает статистику по статусам
func (c *Client) GetStats(ctx context.Context) (*Stats, error) {
	var stats Stats
	if err := c.getJSON(ctx, "/stats", &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// Health проверяет доступность сервиса
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("создание запроса: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("запрос: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: статус %d", ErrAPI, resp.StatusCode)
	}
	return nil
}
