package migrate

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/nikolaykonkin/review-analyzer/internal/testutil"
	"github.com/nikolaykonkin/review-analyzer/migrations"
)

func TestLoad_SortsAndFilters(t *testing.T) {
	fsys := fstest.MapFS{
		"002_b.sql": {Data: []byte("SELECT 2;")},
		"001_a.sql": {Data: []byte("SELECT 1;")},
		"README.md": {Data: []byte("не миграция")},
		"sub/x.sql": {Data: []byte("SELECT 3;")},
	}

	got, err := Load(fsys)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	want := []string{"001_a.sql", "002_b.sql"}
	if len(got) != len(want) {
		t.Fatalf("получено %d миграций, ожидалось %d: %v", len(got), len(want), got)
	}
	for i, version := range want {
		if got[i].Version != version {
			t.Errorf("миграция %d: %q, ожидалось %q", i, got[i].Version, version)
		}
	}
}

func TestUp_AppliesEmbeddedMigrationsOnce(t *testing.T) {
	pool := testutil.NewPool(t)
	ctx := context.Background()

	ms, err := Load(migrations.FS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ms) == 0 {
		t.Fatal("встроенных миграций нет")
	}

	applied, err := Up(ctx, pool, ms)
	if err != nil {
		t.Fatalf("первый Up: %v", err)
	}
	if applied != len(ms) {
		t.Errorf("применено %d миграций, ожидалось %d", applied, len(ms))
	}

	applied, err = Up(ctx, pool, ms)
	if err != nil {
		t.Fatalf("повторный Up: %v", err)
	}
	if applied != 0 {
		t.Errorf("повторный запуск применил %d миграций, ожидалось 0", applied)
	}
}

func TestUp_RollsBackOnError(t *testing.T) {
	pool := testutil.NewPool(t)
	ctx := context.Background()

	ms := []Migration{
		{Version: "001_ok.sql", SQL: "CREATE TABLE t_ok (id INT)"},
		{Version: "002_bad.sql", SQL: "THIS IS NOT SQL"},
	}
	if _, err := Up(ctx, pool, ms); err == nil {
		t.Fatal("ожидалась ошибка из-за неверного SQL")
	}

	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('t_ok') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatalf("проверка таблицы: %v", err)
	}
	if exists {
		t.Error("таблица первой миграции осталась после отката")
	}
}

func TestReviewsSchema_Constraints(t *testing.T) {
	pool := testutil.NewPool(t)
	ctx := context.Background()

	ms, err := Load(migrations.FS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := Up(ctx, pool, ms); err != nil {
		t.Fatalf("Up: %v", err)
	}

	insert := func(rating int, status string) error {
		_, err := pool.Exec(ctx,
			`INSERT INTO reviews (product_name, review_text, rating, reviewed_at, status)
			 VALUES ('product', 'text', $1, '2024-01-01', $2)`,
			rating, status)
		return err
	}

	if err := insert(5, "pending"); err != nil {
		t.Errorf("корректная вставка отклонена: %v", err)
	}
	if err := insert(6, "pending"); err == nil {
		t.Error("рейтинг 6 должен отклоняться")
	}
	if err := insert(5, "unknown"); err == nil {
		t.Error("неизвестный статус должен отклоняться")
	}
	if err := insert(5, "done"); err == nil {
		t.Error("статус done без результата анализа должен отклоняться")
	}
}
