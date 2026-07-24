package service

import (
	"backend/internal/domain"
	"backend/internal/model"
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// defaultLimit и maxLimit ограничивают размер страницы в листинге.
const (
	defaultLimit = 50
	maxLimit     = 500
)

type SubscriptionsRepository interface {
	Create(ctx context.Context, in domain.SubscriptionInput) (domain.Subscription, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Subscription, error)
	Update(ctx context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, params domain.ListParams) ([]domain.Subscription, error)
	Cost(ctx context.Context, params domain.CostParams) (int64, error)
	Ping(ctx context.Context) error
}

type SubscriptionsService struct {
	repo SubscriptionsRepository
	log  *slog.Logger
}

func NewSubscriptionsService(repo SubscriptionsRepository, log *slog.Logger) *SubscriptionsService {
	return &SubscriptionsService{repo: repo, log: log}
}

func (s *SubscriptionsService) Create(ctx context.Context, in domain.SubscriptionInput) (domain.Subscription, error) {
	if err := validatePeriod(in); err != nil {
		return domain.Subscription{}, err
	}

	sub, err := s.repo.Create(ctx, in)
	if err != nil {
		s.log.Error("create subscription failed",
			"user_id", in.UserID, "service_name", in.ServiceName, "error", err)
		return domain.Subscription{}, err
	}

	s.log.Info("subscription created",
		"id", sub.ID, "user_id", sub.UserID, "service_name", sub.ServiceName, "price", sub.Price)
	return sub, nil
}

func (s *SubscriptionsService) Get(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	sub, err := s.repo.Get(ctx, id)
	if err != nil {
		s.log.Debug("get subscription failed", "id", id, "error", err)
		return domain.Subscription{}, err
	}

	return sub, nil
}

func (s *SubscriptionsService) Update(ctx context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error) {
	if err := validatePeriod(in); err != nil {
		return domain.Subscription{}, err
	}

	sub, err := s.repo.Update(ctx, id, in)
	if err != nil {
		s.log.Debug("update subscription failed", "id", id, "error", err)
		return domain.Subscription{}, err
	}

	s.log.Info("subscription updated", "id", sub.ID, "user_id", sub.UserID, "service_name", sub.ServiceName)
	return sub, nil
}

func (s *SubscriptionsService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.log.Debug("delete subscription failed", "id", id, "error", err)
		return err
	}

	s.log.Info("subscription deleted", "id", id)
	return nil
}

func (s *SubscriptionsService) List(ctx context.Context, params domain.ListParams) ([]domain.Subscription, error) {
	params.Limit = normalizeLimit(params.Limit)
	if params.Offset < 0 {
		params.Offset = 0
	}

	subs, err := s.repo.List(ctx, params)
	if err != nil {
		s.log.Error("list subscriptions failed", "error", err)
		return nil, err
	}

	s.log.Debug("subscriptions listed", "count", len(subs), "limit", params.Limit, "offset", params.Offset)
	return subs, nil
}

// Cost считает суммарную стоимость подписок, попавших в период, с учётом фильтров.
func (s *SubscriptionsService) Cost(ctx context.Context, params domain.CostParams) (int64, error) {
	if params.From.MonthsInclusive(params.To) <= 0 {
		return 0, model.ErrInvalidPeriod
	}

	total, err := s.repo.Cost(ctx, params)
	if err != nil {
		s.log.Error("calculate cost failed",
			"from", params.From, "to", params.To, "error", err)
		return 0, err
	}

	s.log.Info("cost calculated",
		"from", params.From, "to", params.To, "total", total)
	return total, nil
}

func (s *SubscriptionsService) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

func validatePeriod(in domain.SubscriptionInput) error {
	if in.EndDate != nil && in.EndDate.Index() < in.StartDate.Index() {
		return model.ErrInvalidPeriod
	}
	return nil
}

func normalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultLimit
	case limit > maxLimit:
		return maxLimit
	default:
		return limit
	}
}
