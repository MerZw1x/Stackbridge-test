package postgres

import (
	"backend/internal/domain"
	"backend/internal/model"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// columns перечислены в одном порядке во всех запросах и в scanSubscription.
const columns = `id, service_name, price, user_id, start_date, end_date, created_at, updated_at`

type SubscriptionsRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionsRepository(pool *pgxpool.Pool) *SubscriptionsRepository {
	return &SubscriptionsRepository{pool: pool}
}

func (r *SubscriptionsRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *SubscriptionsRepository) Create(ctx context.Context, in domain.SubscriptionInput) (domain.Subscription, error) {
	const q = `
		INSERT INTO subscriptions (service_name, price, user_id, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + columns

	row := r.pool.QueryRow(ctx, q, in.ServiceName, in.Price, in.UserID, in.StartDate.Time, endDate(in))

	sub, err := scanSubscription(row)
	if err != nil {
		return domain.Subscription{}, err
	}
	return sub.ToDomain(), nil
}

func (r *SubscriptionsRepository) Get(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	const q = `SELECT ` + columns + ` FROM subscriptions WHERE id = $1`

	sub, err := scanSubscription(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return domain.Subscription{}, err
	}
	return sub.ToDomain(), nil
}

func (r *SubscriptionsRepository) Update(ctx context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error) {
	const q = `
		UPDATE subscriptions
		SET service_name = $2,
		    price        = $3,
		    user_id      = $4,
		    start_date   = $5,
		    end_date     = $6,
		    updated_at   = NOW()
		WHERE id = $1
		RETURNING ` + columns

	row := r.pool.QueryRow(ctx, q, id, in.ServiceName, in.Price, in.UserID, in.StartDate.Time, endDate(in))

	sub, err := scanSubscription(row)
	if err != nil {
		return domain.Subscription{}, err
	}
	return sub.ToDomain(), nil
}

func (r *SubscriptionsRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM subscriptions WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *SubscriptionsRepository) List(ctx context.Context, params domain.ListParams) ([]domain.Subscription, error) {
	const q = `
		SELECT ` + columns + `
		FROM subscriptions
		WHERE ($1::uuid IS NULL OR user_id = $1::uuid)
		  AND ($2::text IS NULL OR lower(service_name) = lower($2::text))
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.pool.Query(ctx, q, nullUUID(params.UserID), nullText(params.ServiceName), params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subs := make([]domain.Subscription, 0)
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub.ToDomain())
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return subs, nil
}

// Cost суммирует price × число месяцев, которыми подписка пересекается с периодом.
// Подписка без end_date считается активной до конца периода.
func (r *SubscriptionsRepository) Cost(ctx context.Context, params domain.CostParams) (int64, error) {
	const q = `
		WITH matched AS (
			SELECT price,
			       GREATEST(start_date, $3::date)                AS period_start,
			       LEAST(COALESCE(end_date, $4::date), $4::date) AS period_end
			FROM subscriptions
			WHERE start_date <= $4::date
			  AND (end_date IS NULL OR end_date >= $3::date)
			  AND ($1::uuid IS NULL OR user_id = $1::uuid)
			  AND ($2::text IS NULL OR lower(service_name) = lower($2::text))
		)
		SELECT COALESCE(SUM(
			price::bigint * (
				  (EXTRACT(YEAR FROM period_end)::int * 12 + EXTRACT(MONTH FROM period_end)::int)
				- (EXTRACT(YEAR FROM period_start)::int * 12 + EXTRACT(MONTH FROM period_start)::int)
				+ 1
			)
		), 0)::bigint
		FROM matched`

	var total int64
	err := r.pool.QueryRow(ctx, q,
		nullUUID(params.UserID),
		nullText(params.ServiceName),
		params.From.Time,
		params.To.Time,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

func scanSubscription(row pgx.Row) (model.Subscription, error) {
	var sub model.Subscription

	err := row.Scan(
		&sub.ID,
		&sub.ServiceName,
		&sub.Price,
		&sub.UserID,
		&sub.StartDate,
		&sub.EndDate,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Subscription{}, model.ErrNotFound
		}
		return model.Subscription{}, err
	}

	return sub, nil
}

func endDate(in domain.SubscriptionInput) *time.Time {
	if in.EndDate == nil {
		return nil
	}
	return &in.EndDate.Time
}

// nullUUID и nullText превращают "фильтр не задан" в SQL NULL.
func nullUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func nullText(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
