## Миграции базы данных

### Управление миграциями через Goose

Для управления схемой БД используется `goose`. Все миграции лежат в папке `migration/`.

При запуске через `docker compose` миграции накатываются автоматически сервисом `migrator`
до старта бэкенда — ручные шаги нужны только для локальной разработки.

### Установка goose

``` bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Доступные команды

```bash
# Посмотреть статус миграций
sh migration/migrations.sh --status

# Создать новую миграцию
sh migration/migrations.sh --new <название_миграции>

# Накатить все миграции
sh migration/migrations.sh --up

# Откатить последнюю миграцию
sh migration/migrations.sh --down

# Накатить до конкретной версии
sh migration/migrations.sh --up <версия>

# Откатить до конкретной версии
sh migration/migrations.sh --down <версия>
```

### Запуск миграций вручную

``` bash
# Поднимаем только базу
docker compose up -d db

# Накатываем миграции
sh migration/migrations.sh --up

# Проверяем статус
sh migration/migrations.sh --status
```

### Схема

`20260725104500_create_subscriptions_table.sql` создаёт таблицу `subscriptions`:

| Колонка | Тип | Описание |
|-|-|-|
| `id` | `UUID` | Первичный ключ, `gen_random_uuid()` |
| `service_name` | `VARCHAR(255)` | Название сервиса |
| `price` | `INTEGER` | Стоимость месяца в рублях, `>= 0` |
| `user_id` | `UUID` | ID пользователя (внешняя система, FK нет) |
| `start_date` | `DATE` | Начало подписки, всегда первое число месяца |
| `end_date` | `DATE` | Окончание подписки, `NULL` — бессрочная |
| `created_at` | `TIMESTAMPTZ` | Время создания записи |
| `updated_at` | `TIMESTAMPTZ` | Время последнего изменения |

Ограничения гарантируют, что обе даты указывают на первое число месяца
(подписка учитывается с точностью до месяца) и что `end_date >= start_date`.

Индексы: по `user_id`, по `lower(service_name)` (фильтр без учёта регистра)
и по паре `(start_date, end_date)` для выборки за период.
