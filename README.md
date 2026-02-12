# Fan Avia Backend

Небольшая микросервисная система для подбора перелетов под футбольные матчи РПЛ.

## Стек

- Go 1.25+
- gRPC (межсервисное взаимодействие)
- HTTP API (api-gateway)
- PostgreSQL (Supabase)
- Redis (кэш матчей и цен)
- OpenTelemetry + Jaeger
- Docker / Docker Compose
- OpenAPI + Swagger UI
- Внешние API: Premierliga, Travelpayouts

## Сервисы

- `api-gateway` (`cmd/api-gateway`): HTTP-вход, агрегация ответов.
- `match-adapter` (`cmd/match-adapter`): загрузка матчей из Premierliga, запись в Postgres, кэш в Redis, gRPC API матчей.
- `airfare-provider` (`cmd/airfare-provider`): расчет 6 тарифных слотов, запрос цен в Travelpayouts, кэш в Redis, gRPC API цен.
- `protos` (`protos/`): protobuf-контракты и сгенерированные клиенты.

## Структура

- `cmd/api-gateway/cmd/main.go` — запуск HTTP API.
- `cmd/api-gateway/internal/api/http/handlers/` — HTTP-ручки.
- `cmd/match-adapter/internal/application/service/` — бизнес-логика матчей и sync upcoming.
- `cmd/match-adapter/internal/infrastructures/premierliga/` — клиент Premierliga + DTO + мапперы.
- `cmd/match-adapter/internal/infrastructures/db/` — Postgres/Redis репозитории.
- `cmd/airfare-provider/internal/application/service/` — построение слотов и агрегация цен.
- `cmd/airfare-provider/internal/infrastructures/travelpayouts/` — клиент Travelpayouts + мапперы.
- `openapi.yaml` — HTTP контракт api-gateway.
- `docker-compose.yaml` — локальная оркестрация сервисов.

## Конфигурация

У каждого сервиса свой `.env`:

- `cmd/match-adapter/.env`
- `cmd/airfare-provider/.env`
- `cmd/api-gateway/.env` (опционально, если используешь env override)

Важно: Postgres в `docker-compose.yaml` не поднимается, ожидается внешний инстанс (например Supabase).

## Запуск локально (без контейнеров для Go-сервисов)

1. Поднять инфраструктуру:

```bash
docker compose up -d redis jaeger
```

2. Запустить сервисы:

```bash
task up
```

3. Проверить health:

```bash
curl "http://localhost:8080/healthz"
```

## Запуск через Docker Compose (полный стек)

```bash
docker compose up -d --build
```

Остановить:

```bash
docker compose down
```

## Порты и UI

- API Gateway: `http://localhost:8080`
- Swagger UI (openapi): `http://localhost:8081`
- Jaeger UI: `http://localhost:16686`
- Match Adapter gRPC: `localhost:44045`
- Airfare Provider gRPC: `localhost:44044`
- Redis: `localhost:6379`

## Основные HTTP ручки

- `GET /healthz` — healthcheck.
- `GET /v1/matches/{match_id}` — матч по id.
- `GET /v1/matches?ids=16114,16115` — список матчей по id.
- `GET /v1/matches/upcoming?limit=12` — ближайшие матчи.
- `GET /v1/matches/{match_id}/airfare?origin_iata=MOW` — 6 тарифных слотов по матчу.
- `GET /v1/matches/upcoming-with-airfare?limit=12&origin_iata=MOW` — ближайшие матчи + best airfare summary.

## Как сервисы общаются между собой

1. Клиент идет в `api-gateway` по HTTP.
2. `api-gateway` вызывает `match-adapter` по gRPC, когда нужны матчи.
3. `api-gateway` вызывает `airfare-provider` по gRPC, когда нужны цены.
4. `airfare-provider` внутри вызывает `match-adapter` по gRPC для match snapshot (kickoff, destination_iata, tickets_link).
5. `airfare-provider` обращается в Travelpayouts HTTP API за ценами и кэширует ответ в Redis (`airfare:{match_id}:{origin_iata}`).
6. `match-adapter` обращается к Premierliga API, сохраняет матчи в Postgres и кэширует в Redis (`match:{match_id}`).

## Наблюдаемость

- Трейсы отправляются в Jaeger collector (`:14268`) и видны в UI `http://localhost:16686`.
- В логах сервисов есть тайминги HTTP/gRPC, cache hit/miss, ошибки внешних источников.

## Проверка ручек

```bash
curl "http://localhost:8080/v1/matches/upcoming?limit=12"
curl "http://localhost:8080/v1/matches/16114"
curl "http://localhost:8080/v1/matches/16114/airfare?origin_iata=MOW"
curl "http://localhost:8080/v1/matches/upcoming-with-airfare?limit=12&origin_iata=MOW"
```
