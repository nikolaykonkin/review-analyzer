package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/nikolaykonkin/review-analyzer/internal/apiclient"
)

// startKeyboard возвращает инлайн-клавиатуру для главного меню
func startKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Последний отзыв", CallbackData: "cb_latest"},
				{Text: "Случайный отзыв", CallbackData: "cb_random"},
			},
			{
				{Text: "Статистика", CallbackData: "cb_stats"},
				{Text: "Помощь", CallbackData: "cb_help"},
			},
		},
	}
}

func (b *Bot) handleStart(ctx context.Context, api *tg.Bot, update *models.Update) {
	text := "<b>Привет!</b> Я бот для работы с отзывами на косметику.\n\n" +
		"Вы можете посмотреть последний, случайный отзыв или статистику, используя кнопки ниже.\n" +
		"Также можно ввести команду <code>/review &lt;id&gt;</code>, чтобы получить конкретный отзыв."
	b.sendText(ctx, api, update.Message.Chat.ID, text, startKeyboard())
}

func (b *Bot) handleHelp(ctx context.Context, api *tg.Bot, update *models.Update) {
	text := "<b>Доступные команды:</b>\n\n" +
		"/start - главное меню\n" +
		"/help - это сообщение\n" +
		"/review &lt;id&gt; - получить отзыв по номеру (например, <code>/review 15</code>)"
	b.sendText(ctx, api, update.Message.Chat.ID, text, nil)
}

func (b *Bot) handleReview(ctx context.Context, api *tg.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	// Текст выглядит как "/review 15", нужно извлечь "15"
	parts := strings.Fields(update.Message.Text)
	if len(parts) != 2 {
		b.sendText(ctx, api, chatID, "Формат: <code>/review &lt;id&gt;</code>", nil)
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.sendText(ctx, api, chatID, "ID отзыва должен быть целым числом.", nil)
		return
	}

	rev, err := b.client.GetReview(ctx, id)
	if err != nil {
		if errors.Is(err, apiclient.ErrNotFound) {
			b.sendText(ctx, api, chatID, fmt.Sprintf("Отзыв #%d не найден.", id), nil)
			return
		}
		b.log.Error("получение отзыва из API", slog.Int64("id", id), slog.Any("error", err))
		b.sendText(ctx, api, chatID, "Ошибка при обращении к сервису. Попробуйте позже.", nil)
		return
	}

	b.sendText(ctx, api, chatID, FormatReview(rev), nil)
}

func (b *Bot) handleCallbackLatest(ctx context.Context, api *tg.Bot, update *models.Update) {
	b.answerCallback(ctx, api, update.CallbackQuery.ID)

	chatID, ok := callbackChatID(update.CallbackQuery.Message)
	if !ok {
		b.log.Error("неизвестный тип сообщения в callback",
			slog.String("callback_data", update.CallbackQuery.Data))
		return
	}

	rev, err := b.client.GetLatestReview(ctx)
	if err != nil {
		b.handleFetchError(ctx, api, chatID, err)
		return
	}
	b.sendText(ctx, api, chatID, FormatReview(rev), nil)
}

func (b *Bot) handleCallbackRandom(ctx context.Context, api *tg.Bot, update *models.Update) {
	b.answerCallback(ctx, api, update.CallbackQuery.ID)

	chatID, ok := callbackChatID(update.CallbackQuery.Message)
	if !ok {
		b.log.Error("неизвестный тип сообщения в callback",
			slog.String("callback_data", update.CallbackQuery.Data))
		return
	}

	rev, err := b.client.GetRandomReview(ctx)
	if err != nil {
		b.handleFetchError(ctx, api, chatID, err)
		return
	}
	b.sendText(ctx, api, chatID, FormatReview(rev), nil)
}

func (b *Bot) handleCallbackStats(ctx context.Context, api *tg.Bot, update *models.Update) {
	b.answerCallback(ctx, api, update.CallbackQuery.ID)

	chatID, ok := callbackChatID(update.CallbackQuery.Message)
	if !ok {
		b.log.Error("неизвестный тип сообщения в callback",
			slog.String("callback_data", update.CallbackQuery.Data))
		return
	}

	stats, err := b.client.GetStats(ctx)
	if err != nil {
		b.handleFetchError(ctx, api, chatID, err)
		return
	}
	b.sendText(ctx, api, chatID, FormatStats(stats), nil)
}

func (b *Bot) handleCallbackHelp(ctx context.Context, api *tg.Bot, update *models.Update) {
	b.answerCallback(ctx, api, update.CallbackQuery.ID)

	chatID, ok := callbackChatID(update.CallbackQuery.Message)
	if !ok {
		b.log.Error("неизвестный тип сообщения в callback",
			slog.String("callback_data", update.CallbackQuery.Data))
		return
	}

	text := "<b>Доступные команды:</b>\n\n" +
		"/start - главное меню\n" +
		"/review &lt;id&gt; - получить отзыв по номеру (например, <code>/review 15</code>)"
	b.sendText(ctx, api, chatID, text, nil)
}

// handleFetchError обрабатывает ошибки API при запросах из кнопок
func (b *Bot) handleFetchError(ctx context.Context, api *tg.Bot, chatID int64, err error) {
	if errors.Is(err, apiclient.ErrNotFound) {
		b.sendText(ctx, api, chatID, "В базе пока нет отзывов.", nil)
		return
	}
	b.log.Error("запрос к API из кнопки", slog.Any("error", err))
	b.sendText(ctx, api, chatID, "Ошибка при обращении к сервису. Попробуйте позже.", nil)
}
