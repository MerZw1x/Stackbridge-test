package local

import (
	"backend/internal/domain"
	"backend/internal/model"
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SubscriptionsRepository — in-memory хранилище, повторяющее семантику postgres-репозитория.
// Используется при STORAGE=local: сервис поднимается без БД.
type SubscriptionsRepository struct {
	mu   sync.RWMutex
	subs map[uuid.UUID]model.Subscription
}

func NewSubscriptionsRepository() *SubscriptionsRepository {
	return &SubscriptionsRepository{subs: make(map[uuid.UUID]model.Subscription)}
}

func (r *SubscriptionsRepository) Ping(_ context.Context) error {
	return nil
}

func (r *SubscriptionsRepository) Create(_ context.Context, in domain.SubscriptionInput) (domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	sub := model.Subscription{
		ID:          uuid.New(),
		ServiceName: in.ServiceName,
		Price:       in.Price,
		UserID:      in.UserID,
		StartDate:   in.StartDate.Time,
		EndDate:     endDate(in),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	r.subs[sub.ID] = sub
	return sub.ToDomain(), nil
}

func (r *SubscriptionsRepository) Get(_ context.Context, id uuid.UUID) (domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sub, ok := r.subs[id]
	if !ok {
		return domain.Subscription{}, model.ErrNotFound
	}
	return sub.ToDomain(), nil
}

func (r *SubscriptionsRepository) Update(_ context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sub, ok := r.subs[id]
	if !ok {
		return domain.Subscription{}, model.ErrNotFound
	}

	sub.ServiceName = in.ServiceName
	sub.Price = in.Price
	sub.UserID = in.UserID
	sub.StartDate = in.StartDate.Time
	sub.EndDate = endDate(in)
	sub.UpdatedAt = time.Now().UTC()

	r.subs[id] = sub
	return sub.ToDomain(), nil
}

func (r *SubscriptionsRepository) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.subs[id]; !ok {
		return model.ErrNotFound
	}

	delete(r.subs, id)
	return nil
}

func (r *SubscriptionsRepository) List(_ context.Context, params domain.ListParams) ([]domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]domain.Subscription, 0)
	for _, sub := range r.subs {
		if !matches(sub, params.UserID, params.ServiceName) {
			continue
		}
		matched = append(matched, sub.ToDomain())
	}

	// Тот же порядок, что и в postgres: created_at DESC, id DESC.
	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID.String() > matched[j].ID.String()
	})

	return paginate(matched, params.Limit, params.Offset), nil
}

func (r *SubscriptionsRepository) Cost(_ context.Context, params domain.CostParams) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total int64
	for _, sub := range r.subs {
		if !matches(sub, params.UserID, params.ServiceName) {
			continue
		}

		months := sub.ToDomain().ActiveMonths(params.From, params.To)
		total += int64(sub.Price) * int64(months)
	}

	return total, nil
}

func matches(sub model.Subscription, userID *uuid.UUID, serviceName *string) bool {
	if userID != nil && sub.UserID != *userID {
		return false
	}
	if serviceName != nil && !strings.EqualFold(sub.ServiceName, *serviceName) {
		return false
	}
	return true
}

func paginate(subs []domain.Subscription, limit, offset int) []domain.Subscription {
	if offset >= len(subs) {
		return []domain.Subscription{}
	}

	end := offset + limit
	if end > len(subs) {
		end = len(subs)
	}

	return subs[offset:end]
}

func endDate(in domain.SubscriptionInput) *time.Time {
	if in.EndDate == nil {
		return nil
	}
	end := in.EndDate.Time
	return &end
}
