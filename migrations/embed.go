// Пакет migrations встраивает SQL-миграции в бинарник
package migrations

import "embed"

// FS содержит файлы *.sql из этого каталога
//
//go:embed *.sql
var FS embed.FS
