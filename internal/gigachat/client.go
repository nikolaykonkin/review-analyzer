package gigachat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const chatEndpoint = "https://api.giga.chat/v1/chat/completions"

// Client - клиент для взаимодействия с API чата GigaChat
type Client struct {
	http     *http.Client
	endpoint string
	ts       TokenSource
	model    string
}

// NewClient создает клиент чата
// Использует кастомный сертификат, если задан caFile
func NewClient(ts TokenSource, model string, caFile string) (*Client, error) {
	client, err := newHTTPClient(caFile)
	if err != nil {
		return nil, err
	}
	return &Client{
		http:     client,
		endpoint: chatEndpoint,
		ts:       ts,
		model:    model,
	}, nil
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// Chat отправляет запрос к GigaChat и возвращает ответ модели
// При получении 401 делает одну повторную попытку с обновленным токеном
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	token, err := c.ts.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("получение токена: %w", err)
	}

	res, err := c.doRequest(ctx, token, systemPrompt, userMessage)
	if errors.Is(err, ErrUnauthorized) {
		// Токен мог истечь или быть отозван - сбрасываем кэш и пробуем еще раз
		c.ts.Invalidate()
		token, err = c.ts.Token(ctx)
		if err != nil {
			return "", fmt.Errorf("повторное получение токена: %w", err)
		}
		res, err = c.doRequest(ctx, token, systemPrompt, userMessage)
	}
	return res, err
}

func (c *Client) doRequest(ctx context.Context, token, systemPrompt, userMessage string) (string, error) {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature: 0, // Детерминированный ответ
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("сериализация запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("запрос к gigachat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		// Тело ответа не читаем, чтобы избежать попадания sensitive data в ошибку
		return "", fmt.Errorf("%w: статус %d", ErrChatResponse, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // Лимит 10 МБ
	if err != nil {
		return "", fmt.Errorf("%w: чтение ответа: %w", ErrChatResponse, err)
	}

	var res chatResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("%w: разбор JSON: %w", ErrChatResponse, err)
	}
	if len(res.Choices) == 0 || res.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("%w: пустой ответ", ErrChatResponse)
	}
	return res.Choices[0].Message.Content, nil
}
