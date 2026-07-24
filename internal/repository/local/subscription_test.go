package local

import (
	"backend/internal/domain"
	"backend/internal/model"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func input(serviceName string, price int, userID uuid.UUID, start domain.MonthYear, end *domain.MonthYear) domain.SubscriptionInput {
	return domain.SubscriptionInput{
		ServiceName: serviceName,
		Price:       price,
		UserID:      userID,
		StartDate:   start,
		EndDate:     end,
	}
}

func TestSubscriptionsRepository_CreateAndGet(t *testing.T) {
	repo := NewSubscriptionsRepository()
	ctx := context.Background()

	userID := uuid.New()
	end := domain.NewMonthYear(2025, time.December)

	created, err := repo.Create(ctx, input("Yandex Plus", 400, userID, domain.NewMonthYear(2025, time.July), &end))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "Yandex Plus", created.ServiceName)
	assert.Equal(t, 400, created.Price)
	assert.Equal(t, userID, created.UserID)
	assert.Equal(t, domain.NewMonthYear(2025, time.July), created.StartDate)
	require.NotNil(t, created.EndDate)
	assert.Equal(t, end, *created.EndDate)
	assert.False(t, created.CreatedAt.IsZero())

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, got)
}

func TestSubscriptionsRepository_Get_NotFound(t *testing.T) {
	_, err := NewSubscriptionsRepository().Get(context.Background(), uuid.New())
	assert.ErrorIs(t, err, model.ErrNotFound)
}

func TestSubscriptionsRepository_Update(t *testing.T) {
	repo := NewSubscriptionsRepository()
	ctx := context.Background()

	created, err := repo.Create(ctx, input("Netflix", 900, uuid.New(), domain.NewMonthYear(2025, time.January), nil))
	require.NoError(t, err)

	end := domain.NewMonthYear(2026, time.January)
	updated, err := repo.Update(ctx, created.ID, input("Netflix Premium", 1200, created.UserID, domain.NewMonthYear(2025, time.March), &end))
	require.NoError(t, err)

	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "Netflix Premium", updated.ServiceName)
	assert.Equal(t, 1200, updated.Price)
	assert.Equal(t, domain.NewMonthYear(2025, time.March), updated.StartDate)
	require.NotNil(t, updated.EndDate)
	assert.Equal(t, end, *updated.EndDate)
	assert.Equal(t, created.CreatedAt, updated.CreatedAt)
}

func TestSubscriptionsRepository_Update_NotFound(t *testing.T) {
	repo := NewSubscriptionsRepository()

	_, err := repo.Update(context.Background(), uuid.New(), input("Netflix", 900, uuid.New(), domain.NewMonthYear(2025, time.January), nil))
	assert.ErrorIs(t, err, model.ErrNotFound)
}

func TestSubscriptionsRepository_Delete(t *testing.T) {
	repo := NewSubscriptionsRepository()
	ctx := context.Background()

	created, err := repo.Create(ctx, input("Spotify", 300, uuid.New(), domain.NewMonthYear(2025, time.July), nil))
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, created.ID))
	assert.ErrorIs(t, repo.Delete(ctx, created.ID), model.ErrNotFound)

	_, err = repo.Get(ctx, created.ID)
	assert.ErrorIs(t, err, model.ErrNotFound)
}

func TestSubscriptionsRepository_List_Filters(t *testing.T) {
	repo := NewSubscriptionsRepository()
	ctx := context.Background()

	alice, bob := uuid.New(), uuid.New()
	start := domain.NewMonthYear(2025, time.July)

	_, err := repo.Create(ctx, input("Yandex Plus", 400, alice, start, nil))
	require.NoError(t, err)
	_, err = repo.Create(ctx, input("Netflix", 900, alice, start, nil))
	require.NoError(t, err)
	_, err = repo.Create(ctx, input("Yandex Plus", 400, bob, start, nil))
	require.NoError(t, err)

	all, err := repo.List(ctx, domain.ListParams{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 3)

	byUser, err := repo.List(ctx, domain.ListParams{UserID: &alice, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, byUser, 2)

	name := "yandex plus" // фильтр по названию не зависит от регистра
	byName, err := repo.List(ctx, domain.ListParams{ServiceName: &name, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, byName, 2)

	byBoth, err := repo.List(ctx, domain.ListParams{UserID: &bob, ServiceName: &name, Limit: 10})
	require.NoError(t, err)
	require.Len(t, byBoth, 1)
	assert.Equal(t, bob, byBoth[0].UserID)
}

func TestSubscriptionsRepository_List_Pagination(t *testing.T) {
	repo := NewSubscriptionsRepository()
	ctx := context.Background()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		_, err := repo.Create(ctx, input("Service", 100, userID, domain.NewMonthYear(2025, time.July), nil))
		require.NoError(t, err)
	}

	page, err := repo.List(ctx, domain.ListParams{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, page, 2)

	page, err = repo.List(ctx, domain.ListParams{Limit: 2, Offset: 4})
	require.NoError(t, err)
	assert.Len(t, page, 1)

	page, err = repo.List(ctx, domain.ListParams{Limit: 2, Offset: 100})
	require.NoError(t, err)
	assert.Empty(t, page)
}

func TestSubscriptionsRepository_Cost(t *testing.T) {
	repo := NewSubscriptionsRepository()
	ctx := context.Background()

	userID := uuid.New()
	other := uuid.New()
	endOct := domain.NewMonthYear(2025, time.October)

	// 6 месяцев × 400 в периоде 07-2025..12-2025 (подписка бессрочная).
	_, err := repo.Create(ctx, input("Yandex Plus", 400, userID, domain.NewMonthYear(2025, time.July), nil))
	require.NoError(t, err)
	// 07..10 = 4 месяца × 900.
	_, err = repo.Create(ctx, input("Netflix", 900, userID, domain.NewMonthYear(2025, time.July), &endOct))
	require.NoError(t, err)
	// Другой пользователь — попадает только в общий итог.
	_, err = repo.Create(ctx, input("Spotify", 300, other, domain.NewMonthYear(2025, time.July), nil))
	require.NoError(t, err)

	from := domain.NewMonthYear(2025, time.July)
	to := domain.NewMonthYear(2025, time.December)

	total, err := repo.Cost(ctx, domain.CostParams{From: from, To: to})
	require.NoError(t, err)
	assert.Equal(t, int64(6*400+4*900+6*300), total)

	byUser, err := repo.Cost(ctx, domain.CostParams{UserID: &userID, From: from, To: to})
	require.NoError(t, err)
	assert.Equal(t, int64(6*400+4*900), byUser)

	name := "netflix"
	byName, err := repo.Cost(ctx, domain.CostParams{ServiceName: &name, From: from, To: to})
	require.NoError(t, err)
	assert.Equal(t, int64(4*900), byName)
}

func TestSubscriptionsRepository_Cost_OutsidePeriod(t *testing.T) {
	repo := NewSubscriptionsRepository()
	ctx := context.Background()

	end := domain.NewMonthYear(2024, time.December)
	_, err := repo.Create(ctx, input("Old", 500, uuid.New(), domain.NewMonthYear(2024, time.January), &end))
	require.NoError(t, err)

	total, err := repo.Cost(ctx, domain.CostParams{
		From: domain.NewMonthYear(2025, time.July),
		To:   domain.NewMonthYear(2025, time.December),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestSubscriptionsRepository_ConcurrentCreate(t *testing.T) {
	repo := NewSubscriptionsRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Create(ctx, input("Service", 100, uuid.New(), domain.NewMonthYear(2025, time.July), nil))
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	assert.Len(t, repo.subs, 50)
}
