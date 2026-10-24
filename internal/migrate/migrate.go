// Пакет migrate применяет SQL-миграции к PostgreSQL
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"path"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// lockKey - произвольное число для advisory-блокировки
// Все экземпляры сервиса используют одно и то же, поэтому миграции не идут параллельно
const lockKey = 727001

const createTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version    TEXT        PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// Migration - одна SQL-миграция
type Migration struct {
	// Version - имя файла, например 001_create_reviews.sql
	Version string
	SQL     string
}

// Load читает файлы *.sql из корня fsys
// Миграции возвращаются в лексикографическом порядке имен (fs.ReadDir сортирует по имени)
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("чтение каталога миграций: %w", err)
	}

	var result []Migration
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}
		data, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("чтение миграции %s: %w", entry.Name(), err)
		}
		result = append(result, Migration{Version: entry.Name(), SQL: string(data)})
	}
	return result, nil
}

// Up применяет еще не примененные миграции одной транзакцией и возвращает их число
// При любой ошибке откатывается все
func Up(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("начало транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return 0, fmt.Errorf("advisory-блокировка: %w", err)
	}
	if _, err := tx.Exec(ctx, createTableSQL); err != nil {
		return 0, fmt.Errorf("создание schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, tx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, m := range migrations {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			return 0, fmt.Errorf("миграция %s: %w", m.Version, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", m.Version); err != nil {
			return 0, fmt.Errorf("запись миграции %s: %w", m.Version, err)
		}
		count++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("фиксация транзакции: %w", err)
	}
	return count, nil
}

func appliedVersions(ctx context.Context, tx pgx.Tx) (map[string]struct{}, error) {
	rows, err := tx.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("чтение schema_migrations: %w", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("чтение schema_migrations: %w", err)
	}

	applied := make(map[string]struct{}, len(versions))
	for _, v := range versions {
		applied[v] = struct{}{}
	}
	return applied, nil
}
