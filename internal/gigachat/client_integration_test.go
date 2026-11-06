package gigachat_test

import (
	"context"
	"os"
	"testing"

	"github.com/nikolaykonkin/review-analyzer/internal/analyzer"
	"github.com/nikolaykonkin/review-analyzer/internal/gigachat"
)

func TestClient_Chat_Integration(t *testing.T) {
	creds := os.Getenv("GIGACHAT_CREDENTIALS")
	if creds == "" {
		t.Skip("GIGACHAT_CREDENTIALS не задан, интеграционный тест пропущен")
	}

	scope := os.Getenv("GIGACHAT_SCOPE")
	if scope == "" {
		scope = "GIGACHAT_API_PERS"
	}

	model := os.Getenv("GIGACHAT_MODEL")
	if model == "" {
		model = "GigaChat"
	}

	caFile := os.Getenv("GIGACHAT_CA_FILE")

	ts, err := gigachat.NewTokenSource(creds, scope, caFile)
	if err != nil {
		t.Fatalf("создание источника токена: %v", err)
	}

	client, err := gigachat.NewClient(ts, model, caFile)
	if err != nil {
		t.Fatalf("создание клиента: %v", err)
	}

	// Промпт и сообщение пользователя из пакета analyzer
	prompt := analyzer.SystemPrompt()
	userMsg := analyzer.UserMessage("Крем вызвал красную сыпь на щеках, пришлось сразу смыть.")

	ctx := context.Background()
	respText, err := client.Chat(ctx, prompt, userMsg)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Проверяем, что ответ модели проходит нашу строгую валидацию
	result, err := analyzer.Parse(respText)
	if err != nil {
		t.Fatalf("Parse: %v\nОтвет модели: %s", err, respText)
	}

	if result.Sentiment != analyzer.SentimentNegative {
		t.Errorf("Sentiment = %q, ожидался %q", result.Sentiment, analyzer.SentimentNegative)
	}
	if result.Priority != analyzer.PriorityHigh {
		t.Errorf("Priority = %q, ожидался %q (в отзыве есть жалоба на сыпь)", result.Priority, analyzer.PriorityHigh)
	}

	// Логируем успешный результат для наглядности
	t.Logf("Успешный ответ модели: %+v", result)
}
