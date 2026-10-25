// Команда service: HTTP API и фоновый воркер анализа отзывов
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nikolaykonkin/review-analyzer/internal/config"
	"github.com/nikolaykonkin/review-analyzer/internal/httpapi"
	"github.com/nikolaykonkin/review-analyzer/internal/logger"
	"github.com/nikolaykonkin/review-analyzer/internal/migrate"
	"github.com/nikolaykonkin/review-analyzer/migrations"
)

const (
	startupTimeout    = 30 * time.Second
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("загрузка конфигурации: %w", err)
	}

	log := logger.New(os.Stdout, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Подключение и миграции ограничены общим таймаутом запуска
	startCtx, cancelStart := context.WithTimeout(ctx, startupTimeout)
	defer cancelStart()

	pool, err := pgxpool.New(startCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("создание пула подключений: %w", err)
	}
	// Пул закрывается при выходе из run, то есть уже после остановки HTTP-сервера
	defer pool.Close()

	if err := pool.Ping(startCtx); err != nil {
		return fmt.Errorf("проверка подключения к базе данных: %w", err)
	}

	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		return fmt.Errorf("загрузка миграций: %w", err)
	}
	applied, err := migrate.Up(startCtx, pool, ms)
	if err != nil {
		return fmt.Errorf("применение миграций: %w", err)
	}
	log.Info("миграции применены", slog.Int("applied", applied), slog.Int("total", len(ms)))

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http-сервер запускается", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http-сервер: %w", err)
	case <-ctx.Done():
		log.Info("получен сигнал остановки")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("остановка http-сервера: %w", err)
	}
	log.Info("сервис остановлен")
	return nil
}
