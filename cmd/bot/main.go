// Команда bot: Telegram-бот, тонкий клиент к HTTP API сервиса
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nikolaykonkin/review-analyzer/internal/apiclient"
	"github.com/nikolaykonkin/review-analyzer/internal/bot"
	"github.com/nikolaykonkin/review-analyzer/internal/config"
	"github.com/nikolaykonkin/review-analyzer/internal/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadBot(os.Getenv)
	if err != nil {
		return fmt.Errorf("загрузка конфигурации бота: %w", err)
	}

	log := logger.New(os.Stdout, slog.LevelInfo)
	client := apiclient.New(cfg.APIURL)

	// Проверяем доступность API при старте
	if err := client.Health(context.Background()); err != nil {
		log.Warn("API сервиса недоступно при старте", slog.String("url", cfg.APIURL), slog.Any("error", err))
	}

	tgBot, err := bot.New(cfg.Token, cfg.AllowedChatIDs, client, log)
	if err != nil {
		return fmt.Errorf("создание бота: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tgBot.Start(ctx)
	return nil
}
