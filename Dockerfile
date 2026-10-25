# Сборка: статические бинарники (CGO_ENABLED=0)
FROM golang:1.26-alpine AS build
WORKDIR /src

# Сначала зависимости: слой кэшируется, пока go.mod и go.sum не менялись
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service ./cmd/service \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

# Общая основа итоговых образов: непривилегированный пользователь
FROM alpine:3 AS runtime
RUN adduser -D -H -u 10001 app
USER app
WORKDIR /app

# Образ сервиса
# Всегда собирается с явной целью: --target service
FROM runtime AS service
COPY --from=build /out/service /app/service
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${HTTP_PORT:-8080}/healthz" || exit 1
ENTRYPOINT ["/app/service"]

# Образ бота (бот пока не реализован)
# Цель: --target bot
FROM runtime AS bot
COPY --from=build /out/bot /app/bot
ENTRYPOINT ["/app/bot"]