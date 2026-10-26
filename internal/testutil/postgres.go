// Пакет testutil содержит вспомогательные функции для интеграционных тестов
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool создает пул к TEST_DATABASE_URL со своей временной схемой,
// чтобы тесты не трогали рабочие таблицы
// Схема удаляется по завершении теста
// Без переменной окружения тест пропускается
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задан, интеграционный тест пропущен")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("подключение к тестовой базе: %v", err)
	}

	// Имя схемы генерируем сами, а идентификатор экранируем штатным способом
	schema := pgx.Identifier{fmt.Sprintf("test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("создание схемы: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("разбор строки подключения: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("создание пула: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
