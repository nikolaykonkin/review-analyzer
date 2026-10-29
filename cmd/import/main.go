// Команда import: загрузка случайной выборки отзывов из CSV в PostgreSQL
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nikolaykonkin/review-analyzer/internal/config"
	"github.com/nikolaykonkin/review-analyzer/internal/importer"
	"github.com/nikolaykonkin/review-analyzer/internal/logger"
	"github.com/nikolaykonkin/review-analyzer/internal/storage"
)

const readBufferSize = 1 << 20

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run() error {
	sampleSize := flag.Int("sample", 1000, "размер случайной выборки отзывов")
	seed := flag.Uint64("seed", 1, "seed генератора: при тех же файлах и seed выборка одинакова")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Использование: import [-sample N] [-seed S] файл.csv [файл.csv ...]")
		flag.PrintDefaults()
	}
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		flag.Usage()
		return errors.New("не указан ни один CSV-файл")
	}

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("загрузка конфигурации: %w", err)
	}
	log := logger.New(os.Stdout, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sampler, err := importer.NewSampler(*sampleSize, *seed)
	if err != nil {
		return fmt.Errorf("создание выборки: %w", err)
	}

	for _, name := range files {
		stats, err := readFile(ctx, name, sampler.Add)
		if err != nil {
			return fmt.Errorf("файл %s: %w", filepath.Base(name), err)
		}
		log.Info("файл прочитан",
			slog.String("file", filepath.Base(name)),
			slog.Int("records", stats.Total),
			slog.Int("valid", stats.Valid),
			slog.Any("skipped", stats.Skipped),
		)
	}
	log.Info("выборка сформирована",
		slog.Int("size", len(sampler.Items())),
		slog.Int("valid_seen", sampler.Seen()),
	)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("создание пула подключений: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("проверка подключения к базе данных: %w", err)
	}

	result, err := importer.Insert(ctx, storage.New(pool), sampler.Items())
	if err != nil {
		return fmt.Errorf("загрузка выборки: %w", err)
	}
	log.Info("импорт завершен",
		slog.Int("inserted", result.Inserted),
		slog.Int("duplicates", result.Duplicates),
	)
	return nil
}

// readFile читает один CSV-файл и передает валидные отзывы в handle
func readFile(ctx context.Context, name string, handle func(storage.NewReview)) (importer.Stats, error) {
	f, err := os.Open(name)
	if err != nil {
		return importer.Stats{}, err
	}
	defer f.Close()

	return importer.Read(ctx, bufio.NewReaderSize(f, readBufferSize), handle)
}
