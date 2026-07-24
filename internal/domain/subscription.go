package domain

import (
	"time"

	"github.com/google/uuid"
)

// Subscription — запись о подписке пользователя на онлайн-сервис.
type Subscription struct {
	ID          uuid.UUID
	ServiceName string
	Price       int
	UserID      uuid.UUID
	StartDate   MonthYear
	EndDate     *MonthYear
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SubscriptionInput — данные, которые приходят снаружи при создании и обновлении.
type SubscriptionInput struct {
	ServiceName string
	Price       int
	UserID      uuid.UUID
	StartDate   MonthYear
	EndDate     *MonthYear
}

// ListParams — параметры выборки списка подписок.
// Nil-поля фильтра означают "не фильтровать".
type ListParams struct {
	UserID      *uuid.UUID
	ServiceName *string
	Limit       int
	Offset      int
}

// CostParams — параметры подсчёта суммарной стоимости за период [From, To] включительно.
type CostParams struct {
	UserID      *uuid.UUID
	ServiceName *string
	From        MonthYear
	To          MonthYear
}

// ActiveMonths — сколько месяцев подписка попадает в период [from, to] включительно.
// Открытая подписка (EndDate == nil) считается активной до конца периода.
func (s Subscription) ActiveMonths(from, to MonthYear) int {
	start := s.StartDate
	if start.Index() < from.Index() {
		start = from
	}

	end := to
	if s.EndDate != nil && s.EndDate.Index() < to.Index() {
		end = *s.EndDate
	}

	months := start.MonthsInclusive(end)
	if months < 0 {
		return 0
	}
	return months
}
