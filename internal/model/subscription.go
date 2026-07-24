package model

import (
	"backend/internal/domain"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("subscription not found")
	ErrInvalidPeriod = errors.New("end date is before start date")
)

// Subscription — представление подписки в хранилище.
// start_date и end_date лежат в БД как DATE и всегда указывают на первое число месяца.
type Subscription struct {
	ID          uuid.UUID  `db:"id"`
	ServiceName string     `db:"service_name"`
	Price       int        `db:"price"`
	UserID      uuid.UUID  `db:"user_id"`
	StartDate   time.Time  `db:"start_date"`
	EndDate     *time.Time `db:"end_date"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (s Subscription) ToDomain() domain.Subscription {
	sub := domain.Subscription{
		ID:          s.ID,
		ServiceName: s.ServiceName,
		Price:       s.Price,
		UserID:      s.UserID,
		StartDate:   domain.MonthYearOf(s.StartDate),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}

	if s.EndDate != nil {
		end := domain.MonthYearOf(*s.EndDate)
		sub.EndDate = &end
	}

	return sub
}
