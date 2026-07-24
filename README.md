# Subscriptions Service

REST-сервис для агрегации данных об онлайн-подписках пользователей.

- CRUDL по подпискам: название сервиса, стоимость месяца в рублях, `user_id` (UUID),
  дата начала и опциональная дата окончания.
- Даты передаются с точностью до месяца в формате `MM-YYYY` (например, `07-2025`).
- Отдельная ручка считает суммарную стоимость подписок за период с фильтрами
  по пользователю и названию сервиса.
- Swagger-документация: `http://localhost:8080/swagger/index.html`.
- Два хранилища на выбор при запуске: `postgres` и `local` (in-memory).

## Быстрый старт

```bash
cp .env.example .env
make build
```

`docker compose` поднимет Postgres, дождётся его готовности, накатит миграции
(сервис `migrator` на базе `goose`) и только после этого запустит сервис.
Отдельных шагов для миграций не нужно.

Сервис поднимется на `http://localhost:8080`, Swagger UI — на
`http://localhost:8080/swagger/index.html`.

## API

Базовый префикс — `/subscriptions`. Все ответы в JSON, ошибки имеют вид
`{ "error": "описание" }`.

### `POST /subscriptions` — создать подписку

```json
{
  "service_name": "Yandex Plus",
  "price": 400,
  "user_id": "60601fee-2bf1-4721-ae6f-7636e79a0cba",
  "start_date": "07-2025"
}
```

`end_date` опционален: если его нет, подписка считается бессрочной.

Ответы:
- `201 Created` — созданная запись
- `400 Bad Request` — невалидное тело, невалидный UUID, неверный формат даты,
  отрицательная цена или `end_date` раньше `start_date`
- `500 Internal Server Error`

```json
{
  "id": "9b7c2f5e-1c1a-4c2b-9d3e-5f6a7b8c9d0e",
  "service_name": "Yandex Plus",
  "price": 400,
  "user_id": "60601fee-2bf1-4721-ae6f-7636e79a0cba",
  "start_date": "07-2025",
  "created_at": "2026-07-25T10:45:00Z",
  "updated_at": "2026-07-25T10:45:00Z"
}
```

### `GET /subscriptions/{id}` — получить подписку

- `200 OK` — запись
- `400 Bad Request` — `id` не UUID
- `404 Not Found` — `{ "error": "subscription not found" }`

### `PUT /subscriptions/{id}` — обновить подписку

Тело совпадает с телом создания и полностью заменяет запись.

- `200 OK` — обновлённая запись
- `400 Bad Request` / `404 Not Found`

### `DELETE /subscriptions/{id}` — удалить подписку

- `204 No Content`
- `400 Bad Request` / `404 Not Found`

### `GET /subscriptions` — список подписок

Query-параметры (все опциональны):

| Параметр | Описание |
|-|-|
| `user_id` | Фильтр по пользователю (UUID) |
| `service_name` | Фильтр по названию сервиса, без учёта регистра |
| `limit` | Размер страницы, `1..500`, по умолчанию `50` |
| `offset` | Смещение, по умолчанию `0` |

```json
{
  "items": [ { "id": "...", "service_name": "Yandex Plus", "price": 400, "...": "..." } ],
  "count": 1,
  "limit": 50,
  "offset": 0
}
```

### `GET /subscriptions/cost` — суммарная стоимость за период

| Параметр | Обязательный | Описание |
|-|-|-|
| `from` | да | Начало периода, `MM-YYYY` |
| `to` | да | Конец периода, `MM-YYYY` |
| `user_id` | нет | Фильтр по пользователю |
| `service_name` | нет | Фильтр по названию сервиса, без учёта регистра |

```bash
curl "http://localhost:8080/subscriptions/cost?from=07-2025&to=12-2025&service_name=Yandex%20Plus"
```

```json
{ "total": 2400, "currency": "RUB", "from": "07-2025", "to": "12-2025" }
```

- `400 Bad Request` — отсутствует или неверен формат `from`/`to`, либо `to` раньше `from`

### `GET /health` — проверка живости

Проверяет доступность хранилища (для `postgres` — пинг пула соединений).

- `200 OK` — `{ "status": "ok" }`
- `503 Service Unavailable` — хранилище недоступно

## Как считается стоимость за период

Период `[from, to]` включает обе границы. Для каждой подписки берётся пересечение
её срока действия с периодом, и стоимость умножается на число месяцев в этом
пересечении:

```
months = min(end_date, to) - max(start_date, from) + 1
total  = Σ price × months
```

Подписка без `end_date` считается активной до конца периода, подписки, не
пересекающиеся с периодом, дают `0`.

Пример: подписка за 400 ₽ с `start_date = 07-2025` без даты окончания при запросе
периода `07-2025 … 12-2025` даёт `400 × 6 = 2400`.

В Postgres расчёт выполняется одним запросом (`GREATEST`/`LEAST` по границам
периода), в in-memory хранилище — той же формулой в Go, поэтому оба хранилища
дают одинаковый результат.

## Запуск

### 1. Через docker compose (postgres)

```bash
cp .env.example .env   # при необходимости отредактировать, STORAGE=postgres
make build
```

### 2. Через docker compose (in-memory)

В `.env` выставить `STORAGE=local` — Postgres в этом режиме не используется,
но контейнер БД всё равно поднимется (можно остановить: `docker compose stop db`).

### 3. Локально

```bash
export $(cat .env | xargs)
go run ./cmd
```

Для локального запуска с Postgres сначала поднимите базу и накатите миграции:

```bash
docker compose up -d db
sh migration/migrations.sh --up
```

## Переменные окружения

| Переменная | Описание |
|-|-|
| `SERVER_PORT` | Порт HTTP-сервера |
| `STORAGE` | `postgres` или `local` |
| `LOG_LEVEL` | `debug`, `info`, `warn` или `error` |
| `DATABASE_HOST` | Хост Postgres (при `STORAGE=postgres`) |
| `DATABASE_PORT` | Порт Postgres |
| `DATABASE_NAME` | Имя БД |
| `DATABASE_USER` | Пользователь |
| `DATABASE_PASSWORD` | Пароль |

## Миграции

Описание команд — в [migration/MIGRATION.md](migration/MIGRATION.md).

## Swagger

Документация генерируется из аннотаций в коде через `swaggo/swag`:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
make swagger
```

Готовые `docs/swagger.json` и `docs/swagger.yaml` лежат в репозитории,
UI доступен на `/swagger/index.html`.

## Логирование

Логи структурированные, `log/slog` в JSON. Каждому запросу присваивается
`X-Request-ID` (или переиспользуется из заголовка), по которому связываются
запись о запросе и сообщения об ошибках. Уровень задаётся через `LOG_LEVEL`.

```json
{"time":"2026-07-25T10:45:00Z","level":"INFO","msg":"request completed","request_id":"...","method":"POST","path":"/subscriptions","status":201,"duration_ms":3}
```

## Makefile

```bash
make up              # docker compose up -d
make build           # docker compose up -d --build
make down            # docker compose down
make clean           # down -v --rmi all
make test            # go test -v ./...
make smoke           # сквозная проверка API на поднятом сервисе
make swagger         # перегенерировать swagger-документацию
make migrate-up      # накатить миграции локально
make migrate-status  # статус миграций
```

## Тесты

```bash
go test ./...
```

Покрыты: конфигурация, разбор и арифметика дат `MM-YYYY`, бизнес-правила сервиса
(валидация периода, нормализация пагинации), HTTP-слой (коды ответов,
валидация запросов, маршрутизация `/subscriptions/cost` мимо `/subscriptions/:id`)
и in-memory репозиторий, включая расчёт стоимости и конкурентный доступ.

Сквозная проверка поднятого сервиса (33 проверки: CRUDL, фильтры, расчёт
стоимости, коды ошибок, swagger) — `scripts/smoke.sh`. Работает одинаково для
обоих хранилищ:

```bash
make build && make smoke
```

## Структура

```
cmd/                      точка входа, сборка зависимостей, graceful shutdown
docs/                     сгенерированная swagger-документация
internal/config/          конфигурация из окружения (cleanenv)
internal/domain/          доменные типы: Subscription, MonthYear, параметры выборки
internal/model/           представление в хранилище и доменные ошибки
internal/handler/         HTTP-слой на fiber: роуты, DTO, валидация, логирование
internal/service/         бизнес-логика и интерфейс репозитория
internal/repository/      реализации репозитория: postgres и local
migration/                sql-миграции goose и скрипт управления
scripts/                  сквозная проверка API на поднятом сервисе
```
