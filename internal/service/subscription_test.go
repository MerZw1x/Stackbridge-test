package service

import (
	"backend/internal/domain"
	"backend/internal/model"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	createFunc func(ctx context.Context, in domain.SubscriptionInput) (domain.Subscription, error)
	getFunc    func(ctx context.Context, id uuid.UUID) (domain.Subscription, error)
	updateFunc func(ctx context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error)
	deleteFunc func(ctx context.Context, id uuid.UUID) error
	listFunc   func(ctx context.Context, params domain.ListParams) ([]domain.Subscription, error)
	costFunc   func(ctx context.Context, params domain.CostParams) (int64, error)
}

func (m *mockRepository) Create(ctx context.Context, in domain.SubscriptionInput) (domain.Subscription, error) {
	return m.createFunc(ctx, in)
}

func (m *mockRepository) Get(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	return m.getFunc(ctx, id)
}

func (m *mockRepository) Update(ctx context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error) {
	return m.updateFunc(ctx, id, in)
}

func (m *mockRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.deleteFunc(ctx, id)
}

func (m *mockRepository) List(ctx context.Context, params domain.ListParams) ([]domain.Subscription, error) {
	return m.listFunc(ctx, params)
}

func (m *mockRepository) Cost(ctx context.Context, params domain.CostParams) (int64, error) {
	return m.costFunc(ctx, params)
}

func (m *mockRepository) Ping(_ context.Context) error {
	return nil
}

func newService(repo SubscriptionsRepository) *SubscriptionsService {
	return NewSubscriptionsService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func input() domain.SubscriptionInput {
	return domain.SubscriptionInput{
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      uuid.New(),
		StartDate:   domain.NewMonthYear(2025, time.July),
	}
}

func TestCreate_Success(t *testing.T) {
	in := input()

	repo := &mockRepository{
		createFunc: func(_ context.Context, got domain.SubscriptionInput) (domain.Subscription, error) {
			assert.Equal(t, in, got)
			return domain.Subscription{ID: uuid.New(), ServiceName: got.ServiceName, Price: got.Price}, nil
		},
	}

	sub, err := newService(repo).Create(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, in.ServiceName, sub.ServiceName)
	assert.Equal(t, in.Price, sub.Price)
}

func TestCreate_EndBeforeStart(t *testing.T) {
	in := input()
	end := domain.NewMonthYear(2025, time.June)
	in.EndDate = &end

	repo := &mockRepository{
		createFunc: func(_ context.Context, _ domain.SubscriptionInput) (domain.Subscription, error) {
			t.Fatal("repository must not be called for an invalid period")
			return domain.Subscription{}, nil
		},
	}

	_, err := newService(repo).Create(context.Background(), in)
	assert.ErrorIs(t, err, model.ErrInvalidPeriod)
}

func TestCreate_SameMonthPeriodIsValid(t *testing.T) {
	in := input()
	end := in.StartDate
	in.EndDate = &end

	repo := &mockRepository{
		createFunc: func(_ context.Context, _ domain.SubscriptionInput) (domain.Subscription, error) {
			return domain.Subscription{ID: uuid.New()}, nil
		},
	}

	_, err := newService(repo).Create(context.Background(), in)
	assert.NoError(t, err)
}

func TestCreate_RepoError(t *testing.T) {
	repoErr := errors.New("boom")
	repo := &mockRepository{
		createFunc: func(_ context.Context, _ domain.SubscriptionInput) (domain.Subscription, error) {
			return domain.Subscription{}, repoErr
		},
	}

	_, err := newService(repo).Create(context.Background(), input())
	assert.ErrorIs(t, err, repoErr)
}

func TestGet_NotFound(t *testing.T) {
	repo := &mockRepository{
		getFunc: func(_ context.Context, _ uuid.UUID) (domain.Subscription, error) {
			return domain.Subscription{}, model.ErrNotFound
		},
	}

	_, err := newService(repo).Get(context.Background(), uuid.New())
	assert.ErrorIs(t, err, model.ErrNotFound)
}

func TestUpdate_InvalidPeriod(t *testing.T) {
	in := input()
	end := domain.NewMonthYear(2024, time.January)
	in.EndDate = &end

	_, err := newService(&mockRepository{}).Update(context.Background(), uuid.New(), in)
	assert.ErrorIs(t, err, model.ErrInvalidPeriod)
}

func TestDelete_NotFound(t *testing.T) {
	repo := &mockRepository{
		deleteFunc: func(_ context.Context, _ uuid.UUID) error {
			return model.ErrNotFound
		},
	}

	err := newService(repo).Delete(context.Background(), uuid.New())
	assert.ErrorIs(t, err, model.ErrNotFound)
}

func TestList_NormalizesPaging(t *testing.T) {
	cases := map[string]struct {
		limit, offset         int
		wantLimit, wantOffset int
	}{
		"defaults":      {limit: 0, offset: 0, wantLimit: defaultLimit, wantOffset: 0},
		"negative":      {limit: -10, offset: -5, wantLimit: defaultLimit, wantOffset: 0},
		"above_max":     {limit: maxLimit + 1, offset: 10, wantLimit: maxLimit, wantOffset: 10},
		"kept_as_given": {limit: 25, offset: 5, wantLimit: 25, wantOffset: 5},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &mockRepository{
				listFunc: func(_ context.Context, params domain.ListParams) ([]domain.Subscription, error) {
					assert.Equal(t, tc.wantLimit, params.Limit)
					assert.Equal(t, tc.wantOffset, params.Offset)
					return nil, nil
				},
			}

			_, err := newService(repo).List(context.Background(), domain.ListParams{Limit: tc.limit, Offset: tc.offset})
			require.NoError(t, err)
		})
	}
}

func TestCost_Success(t *testing.T) {
	params := domain.CostParams{
		From: domain.NewMonthYear(2025, time.July),
		To:   domain.NewMonthYear(2025, time.December),
	}

	repo := &mockRepository{
		costFunc: func(_ context.Context, got domain.CostParams) (int64, error) {
			assert.Equal(t, params, got)
			return 2400, nil
		},
	}

	total, err := newService(repo).Cost(context.Background(), params)
	require.NoError(t, err)
	assert.Equal(t, int64(2400), total)
}

func TestCost_ToBeforeFrom(t *testing.T) {
	repo := &mockRepository{
		costFunc: func(_ context.Context, _ domain.CostParams) (int64, error) {
			t.Fatal("repository must not be called for an invalid period")
			return 0, nil
		},
	}

	_, err := newService(repo).Cost(context.Background(), domain.CostParams{
		From: domain.NewMonthYear(2025, time.December),
		To:   domain.NewMonthYear(2025, time.July),
	})
	assert.ErrorIs(t, err, model.ErrInvalidPeriod)
}

func TestCost_SingleMonthPeriodIsValid(t *testing.T) {
	month := domain.NewMonthYear(2025, time.July)

	repo := &mockRepository{
		costFunc: func(_ context.Context, _ domain.CostParams) (int64, error) {
			return 400, nil
		},
	}

	total, err := newService(repo).Cost(context.Background(), domain.CostParams{From: month, To: month})
	require.NoError(t, err)
	assert.Equal(t, int64(400), total)
}
