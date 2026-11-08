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
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nikolaykonkin/review-analyzer/internal/config"
	"github.com/nikolaykonkin/review-analyzer/internal/gigachat"
	"github.com/nikolaykonkin/review-analyzer/internal/httpapi"
	"github.com/nikolaykonkin/review-analyzer/internal/logger"
	"github.com/nikolaykonkin/review-analyzer/internal/migrate"
	"github.com/nikolaykonkin/review-analyzer/internal/storage"
	"github.com/nikolaykonkin/review-analyzer/internal/worker"
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

	repo := storage.New(pool)

	// Запускаем фоновый воркер, если настроен GigaChat
	var wg sync.WaitGroup
	if cfg.GigaChat.Enabled() {
		ts, err := gigachat.NewTokenSource(cfg.GigaChat.Credentials, cfg.GigaChat.Scope, cfg.GigaChat.CAFile)
		if err != nil {
			return fmt.Errorf("создание источника токена: %w", err)
		}
		gcClient, err := gigachat.NewClient(ts, cfg.GigaChat.Model, cfg.GigaChat.CAFile)
		if err != nil {
			return fmt.Errorf("создание клиента gigachat: %w", err)
		}

		w := worker.New(repo, worker.NewGigaChatAnalyzer(gcClient), log)
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Run(ctx)
		}()
		log.Info("фоновый воркер запущен")
	} else {
		log.Warn("GIGACHAT_CREDENTIALS не задан, фоновый воркер не запущен")
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           httpapi.NewRouter(repo, log), // <-- Передаем репозиторий и логгер
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

	// Ждем завершения текущих задач воркера
	wg.Wait()

	log.Info("сервис остановлен")
	return nil
}
