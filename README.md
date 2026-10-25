# review-analyzer

[![CI](https://github.com/nikolaykonkin/review-analyzer/actions/workflows/ci.yml/badge.svg)](https://github.com/nikolaykonkin/review-analyzer/actions/workflows/ci.yml)

Сервис анализа отзывов на косметику и уход с Telegram-интерфейсом. Учебный портфолио-проект.

Статус: в разработке. Реализованы загрузка конфигурации, логирование, подключение к PostgreSQL с автоматическими миграциями, каркас HTTP-сервиса с проверкой `/healthz`, корректная остановка и запуск в Docker Compose. Остальное перечислено в разделе «Что не реализовано и почему».

## Что демонстрирует проект

(раздел в разработке)

## Стек

- Go 1.26 (см. `go.mod`)
- PostgreSQL 17 (образ в `docker-compose.yml`), драйвер `github.com/jackc/pgx/v5`
- HTTP и логирование: стандартная библиотека (`net/http`, `log/slog`)
- Docker (`Dockerfile`) и Docker Compose (`docker-compose.yml`)
- CI: GitHub Actions (`.github/workflows/ci.yml`)

Остальные технологии добавляются по мере появления в коде.

## Быстрый старт

Нужны Docker и Go 1.26.

1. Создайте `.env` из шаблона и замените `change_me` своим паролем в `POSTGRES_PASSWORD`, `DATABASE_URL` и `TEST_DATABASE_URL` (пароль везде одинаковый, без спецсимволов URL):

   ```
   cp .env.example .env
   ```

2. Запустите PostgreSQL и сервис:

   ```
   docker compose up -d --build
   ```

3. Проверьте сервис:

   ```
   curl -i localhost:8080/healthz
   ```

4. Остановка: `docker compose down`. Данные хранятся в томе `postgres_data`; `docker compose down -v` удаляет и их.

Запуск сервиса без Docker (в контейнере только PostgreSQL):

```
docker compose up -d postgres
set -a; source .env; set +a
go run ./cmd/service
```

## Примеры вызовов

(раздел в разработке)

## Схема

Таблица `reviews` (`migrations/001_create_reviews.sql`):

| Поле | Тип | Назначение |
|---|---|---|
| `id` | `BIGINT`, identity | первичный ключ |
| `product_name` | `TEXT`, обязательное | название продукта |
| `brand_name` | `TEXT` | бренд (необязательное) |
| `review_text` | `TEXT`, обязательное | текст отзыва |
| `rating` | `SMALLINT`, 1-5 | рейтинг |
| `reviewed_at` | `DATE`, обязательное | дата отзыва |
| `status` | `TEXT`, по умолчанию `pending` | `pending`, `processing`, `done`, `failed` |
| `sentiment` | `TEXT` | `positive`, `neutral`, `negative` |
| `topics` | `TEXT[]`, по умолчанию пустой | темы отзыва |
| `priority` | `TEXT` | `high`, `normal` |
| `summary` | `TEXT` | краткое содержание |
| `created_at` | `TIMESTAMPTZ`, по умолчанию `now()` | время создания записи |

Ограничение `reviews_done_has_result`: отзыв со статусом `done` обязан иметь `sentiment`, `priority` и `summary`.

Таблица `schema_migrations` (`version`, `applied_at`) создается раннером миграций и хранит имена примененных файлов.

## Как устроено

### Конфигурация

Конфигурация читается из переменных окружения функцией `config.Load` (`internal/config/config.go`); в приложении ей передается `os.Getenv`. Пустая переменная считается незаданной.

| Переменная | По умолчанию | Допустимые значения |
|---|---|---|
| `DATABASE_URL` | обязательна | строка подключения к PostgreSQL |
| `HTTP_PORT` | `8080` | целое число от 1 до 65535 |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` (регистр не важен) |

При недопустимых или пропущенных значениях сервис не стартует: `Load` собирает все ошибки через `errors.Join` и возвращает их разом, процесс завершается с кодом 1 (`cmd/service/main.go`). Значение `DATABASE_URL` содержит пароль и в тексты ошибок не попадает (это проверяет `internal/config/config_test.go`). Шаблон переменных лежит в `.env.example`. Сам файл `.env` приложение не читает: переменные нужно передать через окружение, а Docker Compose подставляет их сам.

### Логирование

Логи пишутся в stdout в формате JSON через `log/slog` (`internal/logger/logger.go`). Минимальный уровень задается переменной `LOG_LEVEL`.

### Хранилище и миграции

SQL-файлы из `migrations/` встраиваются в бинарник (`migrations/embed.go`). Раннер `internal/migrate/migrate.go`:

- `Load` читает `*.sql` в лексикографическом порядке имен;
- `Up` применяет еще не примененные миграции одной транзакцией и записывает их в `schema_migrations`, при ошибке откатывается все;
- `pg_advisory_xact_lock` не дает нескольким экземплярам сервиса мигрировать одновременно.

Сервис применяет миграции при старте, до запуска HTTP-сервера (`cmd/service/main.go`). Соглашение проекта: примененные миграции не редактируются, изменения схемы вносятся новым файлом.

### Корректная остановка

Сервис перехватывает SIGINT и SIGTERM (`cmd/service/main.go`), перестает принимать новые соединения и ждет завершения текущих запросов не дольше 10 секунд (константа `shutdownTimeout`). После остановки HTTP-сервера закрывается пул подключений к базе данных.

### HTTP API

Реализован один маршрут: `GET /healthz` возвращает 200 и тело `ok` (`internal/httpapi/router.go`). Другие методы этого пути получают 405.

### Импорт отзывов

(раздел в разработке)

### Анализатор и промпт

(раздел в разработке)

### Клиент GigaChat

(раздел в разработке)

### Фоновый воркер

(раздел в разработке)

### Telegram-бот

(раздел в разработке)

## Docker

`Dockerfile` собирает статические бинарники (`CGO_ENABLED=0`) и содержит два итоговых образа: `service` и `bot`, выбор через `target`. Образы работают от непривилегированного пользователя `app`; у `service` есть `HEALTHCHECK` по `/healthz`. `docker-compose.yml` запускает PostgreSQL 17 с проверкой готовности и сервис, который стартует после готовности базы. Порты привязаны к `127.0.0.1`; порты на хосте задаются `POSTGRES_HOST_PORT` и `SERVICE_HOST_PORT`. Образ `bot` собирается, но бот пока не реализован и в compose не входит.

## Тесты

Запуск: `go test ./...`. Есть unit-тесты загрузки конфигурации (`internal/config/config_test.go`), логгера (`internal/logger/logger_test.go`), маршрута `/healthz` (`internal/httpapi/router_test.go`) и загрузки миграций (`internal/migrate/migrate_test.go`).

Интеграционные тесты миграций и схемы (`internal/migrate/migrate_test.go`) работают с реальным PostgreSQL и пропускаются, если не задана `TEST_DATABASE_URL`. Запуск с базой из Docker Compose:

```
docker compose up -d postgres
set -a; source .env; set +a
go test -v ./internal/migrate
```

Каждый такой тест создает временную схему и удаляет ее по завершении.

## Мониторинг

(раздел в разработке)

## CI/CD

Workflow `.github/workflows/ci.yml` запускается при push в `main` и для pull request. Проверки: `gofmt -l`, `go vet`, `go build`, `go test`. В CI не задана `TEST_DATABASE_URL`, поэтому интеграционные тесты там пропускаются. Сборка Docker-образов в CI пока не подключена.

## Структура проекта

- `cmd/service/main.go` — точка входа сервиса
- `cmd/bot/main.go` — точка входа бота (пока пустой `main`)
- `internal/config/` — загрузка конфигурации из окружения
- `internal/logger/` — конструктор JSON-логгера
- `internal/httpapi/` — HTTP-маршруты сервиса
- `internal/migrate/` — раннер SQL-миграций
- `migrations/` — SQL-миграции и пакет `embed`
- `Dockerfile`, `.dockerignore`, `docker-compose.yml` — сборка и запуск в Docker
- `.github/workflows/ci.yml` — проверки в GitHub Actions
- `.env.example` — шаблон переменных окружения
- `.gitattributes` — атрибуты путей для git
- `.gitignore` — исключения git
- `go.mod`, `go.sum` — модуль `github.com/nikolaykonkin/review-analyzer` и зависимости

## Что не реализовано и почему

Проект в разработке, поэтому пока не реализованы: импорт отзывов из CSV, анализ через GigaChat, фоновая обработка, REST API (кроме `/healthz`), Telegram-бот (`cmd/bot/main.go` содержит пустой `main`), метрики, Prometheus и Grafana в compose, сборка образов и запуск интеграционных тестов в CI.