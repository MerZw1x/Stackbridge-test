-- +goose Up
CREATE TABLE IF NOT EXISTS subscriptions (
    id           UUID PRIMARY KEY       DEFAULT gen_random_uuid(),
    service_name VARCHAR(255) NOT NULL,
    price        INTEGER      NOT NULL,
    user_id      UUID         NOT NULL,
    start_date   DATE         NOT NULL,
    end_date     DATE,
    created_at   TIMESTAMPTZ  NOT NULL  DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL  DEFAULT NOW(),

    CONSTRAINT subscriptions_price_check      CHECK (price >= 0),
    CONSTRAINT subscriptions_period_check     CHECK (end_date IS NULL OR end_date >= start_date),
    -- Подписка учитывается с точностью до месяца: обе даты всегда первое число.
    CONSTRAINT subscriptions_start_day_check  CHECK (EXTRACT(DAY FROM start_date) = 1),
    CONSTRAINT subscriptions_end_day_check    CHECK (end_date IS NULL OR EXTRACT(DAY FROM end_date) = 1)
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id      ON subscriptions (user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_service_name ON subscriptions (lower(service_name));
CREATE INDEX IF NOT EXISTS idx_subscriptions_period       ON subscriptions (start_date, end_date);

-- +goose Down
DROP TABLE IF EXISTS subscriptions;
