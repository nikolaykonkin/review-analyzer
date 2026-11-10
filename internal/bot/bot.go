// Пакет bot содержит Telegram-бота для доступа к API сервиса
package bot

import (
	"context"
	"log/slog"
	"slices"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/nikolaykonkin/review-analyzer/internal/apiclient"
)

// Bot оборачивает Telegram-клиент и зависимости
type Bot struct {
	api    *tg.Bot
	client *apiclient.Client
	log    *slog.Logger
}

// New создает и настраивает бота
func New(token string, allowedIDs []int64, client *apiclient.Client, log *slog.Logger) (*Bot, error) {
	b := &Bot{
		client: client,
		log:    log,
	}

	opts := []tg.Option{
		tg.WithDefaultHandler(b.handleDefault),
		tg.WithMiddlewares(whitelistMiddleware(allowedIDs, log)),
	}

	api, err := tg.New(token, opts...)
	if err != nil {
		return nil, err
	}
	b.api = api

	// Регистрация команд
	api.RegisterHandler(tg.HandlerTypeMessageText, "/start", tg.MatchTypeExact, b.handleStart)
	api.RegisterHandler(tg.HandlerTypeMessageText, "/help", tg.MatchTypeExact, b.handleHelp)
	api.RegisterHandler(tg.HandlerTypeMessageText, "/review", tg.MatchTypePrefix, b.handleReview)

	// Регистрация колбэков от инлайн-кнопок
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "cb_latest", tg.MatchTypeExact, b.handleCallbackLatest)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "cb_random", tg.MatchTypeExact, b.handleCallbackRandom)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "cb_stats", tg.MatchTypeExact, b.handleCallbackStats)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "cb_help", tg.MatchTypeExact, b.handleCallbackHelp)

	return b, nil
}

// Start запускает long polling
func (b *Bot) Start(ctx context.Context) {
	b.log.Info("telegram-бот запущен")
	b.api.Start(ctx)
	b.log.Info("telegram-бот остановлен")
}

// callbackChatID извлекает chat_id из MaybeInaccessibleMessage
// Возвращает false, если сообщение недоступно или chat_id не найден
func callbackChatID(msg models.MaybeInaccessibleMessage) (int64, bool) {
	// Обычное, доступное сообщение
	if msg.Message != nil {
		return msg.Message.Chat.ID, true
	}
	// Недоступное сообщение (удаленное и т.п.)
	if msg.InaccessibleMessage != nil {
		return msg.InaccessibleMessage.Chat.ID, true
	}
	return 0, false
}

// whitelistMiddleware блокирует сообщения от чатов, не входящих в белый список
// Если список пуст, разрешены все
func whitelistMiddleware(allowedIDs []int64, log *slog.Logger) tg.Middleware {
	return func(next tg.HandlerFunc) tg.HandlerFunc {
		return func(ctx context.Context, bot *tg.Bot, update *models.Update) {
			if len(allowedIDs) == 0 {
				next(ctx, bot, update)
				return
			}

			var chatID int64
			if update.Message != nil {
				chatID = update.Message.Chat.ID
			} else if update.CallbackQuery != nil {
				id, ok := callbackChatID(update.CallbackQuery.Message)
				if !ok {
					log.Warn("не удалось извлечь chat_id из callback",
						slog.String("callback_data", update.CallbackQuery.Data))
					return
				}
				chatID = id
			}

			if !slices.Contains(allowedIDs, chatID) {
				log.Warn("сообщение от неразрешенного chat_id", slog.Int64("chat_id", chatID))
				return
			}

			next(ctx, bot, update)
		}
	}
}

// handleDefault отвечает на обычные текстовые сообщения, не являющиеся командами
func (b *Bot) handleDefault(ctx context.Context, api *tg.Bot, update *models.Update) {
	b.sendText(ctx, api, update.Message.Chat.ID,
		"Я понимаю только команды. Введите /help для списка команд.", nil)
}

// sendText отправляет текстовое сообщение с HTML-разметкой
func (b *Bot) sendText(ctx context.Context, api *tg.Bot, chatID int64, text string, markup *models.InlineKeyboardMarkup) {
	params := &tg.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}

	if _, err := api.SendMessage(ctx, params); err != nil {
		b.log.Error("отправка сообщения", slog.Int64("chat_id", chatID), slog.Any("error", err))
	}
}

// answerCallback подтверждает получение callback-запроса,
// чтобы убрать индикатор загрузки с инлайн-кнопки
func (b *Bot) answerCallback(ctx context.Context, api *tg.Bot, id string) {
	if _, err := api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{
		CallbackQueryID: id,
	}); err != nil {
		b.log.Error("ответ на callback", slog.Any("error", err))
	}
}
