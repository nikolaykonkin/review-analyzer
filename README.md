# review-analyzer

[![CI](https://github.com/nikolaykonkin/review-analyzer/actions/workflows/ci.yml/badge.svg)](https://github.com/nikolaykonkin/review-analyzer/actions/workflows/ci.yml)

Сервис анализа отзывов на косметику и уход с Telegram-интерфейсом.

Статус: в разработке. Реализованы загрузка конфигурации, логирование, подключение к PostgreSQL с автоматическими миграциями, слой хранения отзывов, импорт отзывов из CSV, промпт для языковой модели и проверка ее ответа, источник токена и клиент GigaChat API, фоновый воркер (запускается, если задан `GIGACHAT_CREDENTIALS`), HTTP-сервис с проверкой `/healthz` и получением отзыва по id, корректная остановка и запуск в Docker Compose. Остальное перечислено в разделе «Что не реализовано и почему».

## Что демонстрирует проект

(раздел в разработке)

## Стек

- Go 1.26 (см. `go.mod`)
- PostgreSQL 17 (образ в `docker-compose.yml`), драйвер `github.com/jackc/pgx/v5`
- HTTP и логирование: стандартная библиотека (`net/http`, `log/slog`)
- GigaChat API (Сбер) — внешний сервис для анализа отзывов
- Docker (`Dockerfile`) и Docker Compose (`docker-compose.yml`)
- CI: GitHub Actions (`.github/workflows/ci.yml`)

Остальные технологии добавляются по мере появления в коде.

## Быстрый старт

Нужны Docker и Go 1.26.

1. Создайте `.env` из шаблона и замените `change_me` своим паролем в `POSTGRES_PASSWORD`, `DATABASE_URL` и `TEST_DATABASE_URL` (пароль везде одинаковый, без спецсимволов URL):

   ```
   cp .env.example .env
   ```

2. Запустите PostgreSQL и сервис (сервис при старте создает таблицы):

   ```
   docker compose up -d --build
   ```

3. Проверьте сервис:

   ```
   curl -i localhost:8080/healthz
   ```

4. Загрузите демонстрационные отзывы:

   ```
   set -a; source .env; set +a
   go run ./cmd/import testdata/sample_reviews.csv
   ```

5. Получите отзыв по id (замените `1` на реальный id из базы):

   ```
   curl -i localhost:8080/reviews/1
   ```

6. Остановка: `docker compose down`. Данные хранятся в томе `postgres_data`; `docker compose down -v` удаляет и их.

Если в `.env` задан `GIGACHAT_CREDENTIALS`, сервис при старте запустит фоновый воркер: он раз в несколько секунд забирает отзывы из базы, отправляет их в GigaChat и записывает результат анализа обратно. Без ключа сервис работает как HTTP API, а воркер пишет предупреждение в лог и не запускается.

Запуск сервиса без Docker (в контейнере только PostgreSQL):

```
docker compose up -d postgres
set -a; source .env; set +a
go run ./cmd/service
```

## Примеры вызовов

(раздел в разработке)

## Данные

Для демонстрации используется датасет [Sephora Products and Skincare Reviews](https://www.kaggle.com/datasets/nadyinky/sephora-products-and-skincare-reviews) (автор Nady Inky, Kaggle), собранный в марте 2023 года с сайта Sephora. Датасет распространяется по лицензии [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

Изменения относительно оригинала:

- из файлов `reviews_*.csv` берется случайная выборка (по умолчанию 1000 отзывов);
- используются поля `product_name`, `brand_name`, `review_text`, `rating` и `submission_time`; для ключа дедупликации дополнительно читаются `author_id` и `product_id`, но в базу не сохраняются; остальные поля (в том числе данные о внешности авторов) не импортируются;
- записи с пустым текстом, неверным рейтингом или датой пропускаются;
- текст отзыва хранится без изменений, у названий продукта и бренда обрезаются пробелы по краям.

Данные в репозиторий не входят: каталог `data/` указан в `.gitignore`. Чтобы загрузить настоящий датасет, скачайте его по ссылке выше (нужна учетная запись Kaggle) и положите файлы `reviews_*.csv` в `data/`.

Файл `testdata/sample_reviews.csv` содержит вымышленные отзывы в формате датасета, он не является частью датасета Sephora и используется в тестах и для демонстрации.

## Схема

Таблица `reviews` (`migrations/001_create_reviews.sql`, `migrations/002_add_review_source_key.sql`):

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
| `source_key` | `TEXT`, `UNIQUE` | ключ дедупликации исходного отзыва (допускает `NULL`) |

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
| `GIGACHAT_CREDENTIALS` | — | ключ авторизации GigaChat; пусто — фоновый воркер не запускается |
| `GIGACHAT_SCOPE` | `GIGACHAT_API_PERS` | `GIGACHAT_API_PERS`, `GIGACHAT_API_B2B`, `GIGACHAT_API_CORP` |
| `GIGACHAT_MODEL` | `GigaChat` | название модели |
| `GIGACHAT_CA_FILE` | — | путь к PEM-файлу корневого сертификата НУЦ Минцифры; пусто — только системное хранилище |

При недопустимых или пропущенных значениях сервис не стартует: `Load` собирает все ошибки через `errors.Join` и возвращает их разом, процесс завершается с кодом 1 (`cmd/service/main.go`). Значение `DATABASE_URL` содержит пароль и в тексты ошибок не попадает (это проверяет `internal/config/config_test.go`). Ключ `GIGACHAT_CREDENTIALS` в ошибки также не попадает (проверяет `TestLoad_GigaChatCredentialsNotInErrors`). Шаблон переменных лежит в `.env.example`. Сам файл `.env` приложение не читает: переменные нужно передать через окружение, а Docker Compose подставляет их сам.

### Логирование

Логи пишутся в stdout в формате JSON через `log/slog` (`internal/logger/logger.go`). Минимальный уровень задается переменной `LOG_LEVEL`.

### Хранилище и миграции

SQL-файлы из `migrations/` встраиваются в бинарник (`migrations/embed.go`). Раннер `internal/migrate/migrate.go`:

- `Load` читает `*.sql` в лексикографическом порядке имен;
- `Up` применяет еще не примененные миграции одной транзакцией и записывает их в `schema_migrations`, при ошибке откатывается все;
- `pg_advisory_xact_lock` не дает нескольким экземплярам сервиса мигрировать одновременно.

Сервис применяет миграции при старте, до запуска HTTP-сервера (`cmd/service/main.go`). Соглашение проекта: примененные миграции не редактируются, изменения схемы вносятся новым файлом.

Доступ к отзывам реализован в `internal/storage/storage.go` типом `Repository`:

- `InsertReview` вставляет отзыв запросом `INSERT ... ON CONFLICT (source_key) DO NOTHING RETURNING id`; повторная вставка с тем же `SourceKey` не создает дубликата и не возвращает ошибку;
- `GetReview` читает отзыв по `id`, а при отсутствии возвращает `ErrNotFound`;
- ошибки вставки не содержат текст отзыва (это проверяет `TestInsertReview_ErrorHasNoReviewText` в `internal/storage/storage_test.go`).

### Корректная остановка

Сервис перехватывает SIGINT и SIGTERM (`cmd/service/main.go`), перестает принимать новые соединения и ждет завершения текущих запросов не дольше 10 секунд (константа `shutdownTimeout`). После остановки HTTP-сервера закрывается пул подключений к базе данных.

### HTTP API

Маршруты описаны в `internal/httpapi/router.go`:

- `GET /healthz` возвращает 200 и тело `ok`; другие методы этого пути получают 405.
- `GET /reviews/{id}` возвращает отзыв в формате JSON. `id` должен быть целым числом (иначе 400), при отсутствии отзыва возвращается 404, при внутренней ошибке базы — 500. Детали ошибки пишутся в лог, клиенту отдается общий текст без SQL.

Ответ `GET /reviews/{id}` собирается через DTO (`internal/httpapi/dto.go`), отдельный от `storage.Review`, чтобы не смешивать слои БД и API. Поля `brand_name`, `sentiment`, `priority` и `summary` опускаются, если пусты (`omitempty`); `topics` всегда отдается массивом (пустой массив `[]`, если темы еще не заполнены). Хендлеры лежат в `internal/httpapi/handlers.go` и получают зависимости через структуру `Handler` (интерфейс `Storage` и `*slog.Logger`).

### Импорт отзывов

Команда `cmd/import` (`cmd/import/main.go`) загружает случайную выборку отзывов из одного или нескольких CSV-файлов в таблицу `reviews`:

```
go run ./cmd/import [-sample N] [-seed S] файл.csv [файл.csv ...]
```

- `-sample` (по умолчанию 1000) задает размер выборки, `-seed` (по умолчанию 1) задает seed генератора.
- Файл читается потоком (`internal/importer/parser.go`), колонки ищутся по имени в заголовке. Обязательны `author_id`, `rating`, `submission_time`, `review_text`, `product_id`, `product_name` и `brand_name`.
- Запись пропускается, если нарушен формат CSV, пусто название продукта или текст, рейтинг не целое число от 1 до 5 (допустима запись вида `5.0`), дата не в формате `ГГГГ-ММ-ДД`, в тексте невалидный UTF-8 или нулевой байт. Число пропущенных записей по причинам выводится в лог для каждого файла.
- Выборка равномерная по всем переданным файлам (`internal/importer/sampler.go`, алгоритм reservoir sampling): память зависит только от размера выборки. При тех же файлах (в том же порядке) и seed выборка одинакова.
- Повторный запуск безопасен: ключ `source_key` это SHA-256 от `author_id`, `product_id`, `submission_time` и `review_text` (функция `sourceKey` в `parser.go`), а вставка идет с `ON CONFLICT DO NOTHING`. Итог (`inserted` и `duplicates`) выводится в лог.
- Таблицы создает сервис при старте, поэтому перед импортом его нужно хотя бы раз запустить. Тексты отзывов в логи не пишутся.

### Анализатор и промпт

Пакет `internal/analyzer` описывает результат анализа отзыва, промпт для языковой модели и проверку ее ответа. Вызова модели в пакете нет; клиент GigaChat реализован отдельно (`internal/gigachat`), а связка «клиент + проверка ответа» используется фоновым воркером (`internal/worker`).

Что делает пакет:

- `SystemPrompt()` возвращает инструкцию для модели из `internal/analyzer/prompt.txt` (файл встроен через `go:embed`). В промпте описаны формат ответа, допустимые значения полей и темы, есть примеры ответов.
- `UserMessage(reviewText)` оборачивает текст отзыва в разделители `<<<REVIEW>>>` и `<<<END_REVIEW>>>`. Модели передается только текст отзыва, без названия продукта, бренда и рейтинга. Если разделители встречаются в самом тексте, они удаляются (повторно, пока не перестанут появляться), поэтому отзыв не может сам закрыть блок данных.
- `Parse(response)` разбирает и проверяет ответ модели и возвращает `Result`.

Формат ответа модели: один JSON-объект ровно с четырьмя полями.

| Поле | Допустимые значения |
|---|---|
| `sentiment` | `positive`, `neutral`, `negative` |
| `topics` | одна или несколько тем из списка ниже, без повторов |
| `priority` | `high`, `normal` |
| `summary` | одно предложение, не длиннее 300 символов, без переносов строк, с русскими буквами |

Темы (закрытый список, тема вне списка делает ответ невалидным): эффект, запах, текстура, цвет, цена, упаковка, аллергия, соответствие описанию, качество, подходит для типа кожи, состав, другое. Темы «соответствие описанию» (совпало ли обещанное с реальным) и «подходит для типа кожи» (совместимость с типом кожи или волос автора) намеренно разделены, это различие описано в промпте. Список тем задан в коде (`internal/analyzer/result.go`), определения продублированы в `prompt.txt`; расхождение между ними ловит `TestSystemPrompt_HasKeyMarkers`. Копия списка в этом README тестами не проверяется.

Проверка ответа (`Parse`):

- принимается только один JSON-объект; неизвестные поля, текст до или после JSON, массив вместо объекта и оборванный JSON считаются невалидными;
- единственная терпимость: ответ можно обернуть в блок кода из тройных кавычек (с меткой `json` или без), обертка снимается;
- обязательны все четыре поля, значения `sentiment` и `priority` должны точно совпадать с допустимыми (регистр важен), `topics` не пуст и без повторов;
- `priority` равен `high` только при `sentiment` равном `negative`, иначе ответ невалиден;
- при любом нарушении возвращается ошибка, оборачивающая `ErrInvalidResponse`; текст ответа модели в сообщение об ошибке не попадает, потому что модель могла процитировать отзыв.

Приоритет возвращает модель, а код проверяет только согласованность с тональностью. Поэтому нейтральный отзыв с жалобой не получает приоритет `high`: это осознанное упрощение. Так как разбор идет через `encoding/json`, имена полей сопоставляются без учета регистра, а повтор ключа не считается ошибкой; значения полей проверяются строго.

Что с невалидным ответом делать дальше (например, переводить отзыв в статус failed), решает фоновый воркер (см. раздел «Фоновый воркер»).

Защита от prompt injection. Разделители и инструкция в промпте снижают риск, но это не защита. Настоящая защита: строгая проверка ответа по схеме (см. выше) и отсутствие у модели доступа к инструментам. Давать модели доступ к инструментам не планируется: ее ответ только разбирается и проверяется кодом.

### Клиент GigaChat

Пакет `internal/gigachat` реализует источник токена и клиент чата GigaChat API. Клиент используется фоновым воркером: он получает отзывы из базы, отправляет их в GigaChat и записывает результат обратно.

Источник токена (`internal/gigachat/token.go`):

- отправляет `POST https://ngw.devices.sberbank.ru:9443/api/v2/oauth` с заголовками `RqUID` (UUID v4) и `Authorization: Basic <GIGACHAT_CREDENTIALS>`, тело запроса содержит `scope`;
- кэширует токен на 29 минут (30 минут минус минута запаса); документация GigaChat противоречива в поле `expires_at`, поэтому срок жизни считается фиксированным;
- метод `Invalidate()` сбрасывает кэш, чтобы клиент мог принудительно запросить новый токен;
- `RqUID` генерируется через `crypto/rand` (UUID v4), без внешних зависимостей;
- если задан `GIGACHAT_CA_FILE`, клиент использует этот PEM-файл для проверки TLS.

Клиент чата (`internal/gigachat/client.go`):

- отправляет `POST https://api.giga.chat/v1/chat/completions` с сообщениями `system` (промпт из `analyzer.SystemPrompt()`) и `user` (обернутый текст отзыва из `analyzer.UserMessage()`);
- использует `temperature: 0` для детерминированного ответа;
- при получении 401 вызывает `Invalidate()` у источника токена, получает новый токен и делает ровно одну повторную попытку;
- ответ парсится, проверяется на непустой `choices[0].message.content`; текст ответа модели или промпта не попадает в ошибки;
- ошибки оборачивают `ErrUnauthorized`, `ErrTokenResponse`, `ErrCAFile` или `ErrChatResponse` (в зависимости от этапа).

Интеграционный тест `TestClient_Chat_Integration` вызывает реальный GigaChat API, прогоняет ответ через `analyzer.Parse()` и проверяет, что негативный отзыв с жалобой получает `sentiment = negative` и `priority = high`. Без переменной `GIGACHAT_CREDENTIALS` тест пропускается.

### Фоновый воркер

Фоновый воркер (`internal/worker/worker.go`) обрабатывает отзывы со статусом `pending`: отправляет их в GigaChat, проверяет ответ и записывает результат в базу. Воркер запускается вместе с сервисом, если задан `GIGACHAT_CREDENTIALS` (`cmd/service/main.go`); без ключа сервис работает только как HTTP API и пишет предупреждение в лог.

Как устроен цикл:

- раз в 5 секунд воркер забирает из базы пачку до 10 отзывов со статусом `pending`;
- отзывы захватываются атомарно одним SQL-запросом с `FOR UPDATE SKIP LOCKED`: два экземпляра сервиса не возьмут один отзыв;
- захваченные отзывы переводятся в статус `processing` и раскидываются по 3 горутинам через канал;
- каждый отзыв обрабатывается с таймаутом 2 минуты (константа `analyzeTimeout`);
- если GigaChat вернул ошибку или ответ не прошел `analyzer.Parse`, отзыв переводится в `failed`;
- если ответ валиден, отзыв переводится в `done` и заполняются `sentiment`, `topics`, `priority`, `summary`.

При старте воркер вызывает `ResetProcessingReviews`: все отзывы, оставшиеся в статусе `processing` после падения предыдущего запуска, возвращаются в `pending`. При остановке сервиса воркер дожидается завершения текущих задач (через `sync.WaitGroup`), а обновление статуса в базе выполняется с `context.WithoutCancel`, чтобы отмена контекста не оставила отзыв в `processing`.

Связка «GigaChat + проверка ответа» реализована в `GigaChatAnalyzer` (`internal/worker/worker.go`): он вызывает `gigachat.Client.Chat` с промптом из `analyzer.SystemPrompt()`, оборачивает текст отзыва через `analyzer.UserMessage()` и проверяет ответ через `analyzer.Parse()`.

Воркер тестируется с mock-хранилищем и mock-анализатором (`internal/worker/worker_test.go`); реальная база и GigaChat для этих тестов не нужны.

Ограничение: `ResetProcessingReviews` корректен только при одном экземпляре сервиса. Если запущено несколько экземпляров, сброс одного может вернуть в `pending` отзыв, который прямо сейчас обрабатывает другой. Для пет-проекта это осознанное упрощение.

### Telegram-бот

(раздел в разработке)

## Docker

`Dockerfile` собирает статические бинарники (`CGO_ENABLED=0`) и содержит два итоговых образа: `service` и `bot`, выбор через `target`. Образы работают от непривилегированного пользователя `app`; у `service` есть `HEALTHCHECK` по `/healthz`. `docker-compose.yml` запускает PostgreSQL 17 с проверкой готовности и сервис, который стартует после готовности базы. Порты привязаны к `127.0.0.1`; порты на хосте задаются `POSTGRES_HOST_PORT` и `SERVICE_HOST_PORT`. Каталог `data/` исключен из контекста сборки (`.dockerignore`). Образ `bot` собирается, но бот пока не реализован и в compose не входит. Команда импорта в образы не включена.

## Тесты

Запуск: `go test ./...`. Есть unit-тесты загрузки конфигурации (`internal/config/config_test.go`), логгера (`internal/logger/logger_test.go`), маршрута `/healthz` (`internal/httpapi/router_test.go`), загрузки миграций (`internal/migrate/migrate_test.go`) и импорта: разбор и валидация CSV (`internal/importer/parser_test.go`), выборка (`internal/importer/sampler_test.go`), загрузка с подменой хранилища (`internal/importer/importer_test.go`). Для пакета анализатора тесты промпта и сборки сообщения (`internal/analyzer/prompt_test.go`) и разбора ответа модели (`internal/analyzer/result_test.go`); базы данных они не требуют. Для пакета `internal/gigachat` есть unit-тесты источника токена, генератора `RqUID`, загрузки CA-файла и клиента чата (`token_test.go`, `rquid_test.go`, `tls_test.go`, `client_test.go`). Для пакета `internal/worker` есть unit-тесты с mock-хранилищем и mock-анализатором (`worker_test.go`); базы данных и GigaChat они не требуют. Для пакета `internal/httpapi` есть unit-тесты хендлеров (`handlers_test.go`) с mock-хранилищем: успешный ответ, невалидный id, 404 и 500, а также проверка JSON-контракта (поля, `omitempty`, `topics` как массив).

Интеграционные тесты миграций, схемы, хранилища и идемпотентности импорта (`internal/migrate/migrate_test.go`, `internal/storage/storage_test.go`, `TestInsert_IsIdempotentWithRealStorage` в `internal/importer/importer_test.go`) работают с реальным PostgreSQL и пропускаются, если не задана `TEST_DATABASE_URL`. Запуск с базой из Docker Compose:

```
docker compose up -d postgres
set -a; source .env; set +a
go test -v ./internal/migrate ./internal/storage ./internal/importer
```

Каждый такой тест создает временную схему и удаляет ее по завершении (общий помощник: `internal/testutil/postgres.go`).

Интеграционный тест HTTP-хендлера (`TestGetReview_Integration` в `internal/httpapi/handlers_integration_test.go`) проверяет полный путь от HTTP-запроса до реальной базы и пропускается без `TEST_DATABASE_URL`. Запуск:

```
set -a; source .env; set +a
go test -v ./internal/httpapi
```

Интеграционный тест GigaChat (`TestClient_Chat_Integration` в `internal/gigachat/client_integration_test.go`) вызывает реальный API и пропускается, если не задана `GIGACHAT_CREDENTIALS`. Запуск:

```
set -a; source .env; set +a
go test -v ./internal/gigachat -run TestClient_Chat_Integration
```

## Мониторинг

(раздел в разработке)

## CI/CD

Workflow `.github/workflows/ci.yml` запускается при push в `main` и для pull request. Проверки: `gofmt -l`, `go vet`, `go build`, `go test`. В CI не заданы `TEST_DATABASE_URL` и `GIGACHAT_CREDENTIALS`, поэтому интеграционные тесты базы данных и GigaChat там пропускаются. Сборка Docker-образов в CI пока не подключена.

## Структура проекта

- `cmd/service/main.go` — точка входа сервиса
- `cmd/import/main.go` — команда импорта отзывов из CSV
- `cmd/bot/main.go` — точка входа бота (пока пустой `main`)
- `internal/config/` — загрузка конфигурации из окружения
- `internal/logger/` — конструктор JSON-логгера
- `internal/httpapi/` — HTTP-маршруты сервиса, хендлеры и DTO
- `internal/migrate/` — раннер SQL-миграций
- `internal/storage/` — репозиторий отзывов
- `internal/importer/` — чтение и проверка CSV, выборка, загрузка в хранилище
- `internal/analyzer/` — промпт для языковой модели и проверка ее ответа
- `internal/gigachat/` — источник токена и клиент GigaChat API
- `internal/worker/` — фоновый воркер: захват отзывов из базы, отправка в GigaChat, обновление статуса
- `internal/testutil/` — помощники для интеграционных тестов
- `migrations/` — SQL-миграции и пакет `embed`
- `testdata/` — демонстрационный CSV с вымышленными отзывами
- `Dockerfile`, `.dockerignore`, `docker-compose.yml` — сборка и запуск в Docker
- `.github/workflows/ci.yml` — проверки в GitHub Actions
- `.env.example` — шаблон переменных окружения
- `.gitattributes` — атрибуты путей для git
- `.gitignore` — исключения git (в том числе каталога данных `data/`)
- `go.mod`, `go.sum` — модуль `github.com/nikolaykonkin/review-analyzer` и зависимости

## Что не реализовано и почему

Проект в разработке, поэтому пока не реализованы: остальные REST-маршруты (кроме `/healthz` и `GET /reviews/{id}`), Telegram-бот (`cmd/bot/main.go` содержит пустой `main`), метрики, Prometheus и Grafana в compose, сборка образов и запуск интеграционных тестов в CI.