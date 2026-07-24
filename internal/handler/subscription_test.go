package handler

import (
	"backend/internal/domain"
	"backend/internal/model"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockService struct {
	createFunc func(ctx context.Context, in domain.SubscriptionInput) (domain.Subscription, error)
	getFunc    func(ctx context.Context, id uuid.UUID) (domain.Subscription, error)
	updateFunc func(ctx context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error)
	deleteFunc func(ctx context.Context, id uuid.UUID) error
	listFunc   func(ctx context.Context, params domain.ListParams) ([]domain.Subscription, error)
	costFunc   func(ctx context.Context, params domain.CostParams) (int64, error)
	pingFunc   func(ctx context.Context) error
}

func (m *mockService) Create(ctx context.Context, in domain.SubscriptionInput) (domain.Subscription, error) {
	return m.createFunc(ctx, in)
}

func (m *mockService) Get(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	return m.getFunc(ctx, id)
}

func (m *mockService) Update(ctx context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error) {
	return m.updateFunc(ctx, id, in)
}

func (m *mockService) Delete(ctx context.Context, id uuid.UUID) error {
	return m.deleteFunc(ctx, id)
}

func (m *mockService) List(ctx context.Context, params domain.ListParams) ([]domain.Subscription, error) {
	return m.listFunc(ctx, params)
}

func (m *mockService) Cost(ctx context.Context, params domain.CostParams) (int64, error) {
	return m.costFunc(ctx, params)
}

func (m *mockService) Ping(ctx context.Context) error {
	if m.pingFunc == nil {
		return nil
	}
	return m.pingFunc(ctx)
}

func setupApp(svc SubscriptionsService) *fiber.App {
	app := fiber.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	NewSubscriptionsHandler(svc, logger).Register(app)
	return app
}

func do(t *testing.T, app *fiber.App, method, path string, body any) *http.Response {
	t.Helper()

	var payload []byte
	switch v := body.(type) {
	case nil:
	case []byte:
		payload = v
	default:
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		payload = raw
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var out T
	require.NoError(t, json.Unmarshal(raw, &out), "body: %s", raw)
	return out
}

func price(v int) *int { return &v }

func validRequest() subscriptionRequest {
	return subscriptionRequest{
		ServiceName: "Yandex Plus",
		Price:       price(400),
		UserID:      "60601fee-2bf1-4721-ae6f-7636e79a0cba",
		StartDate:   "07-2025",
	}
}

func sampleSubscription() domain.Subscription {
	end := domain.NewMonthYear(2025, time.December)
	return domain.Subscription{
		ID:          uuid.MustParse("9b7c2f5e-1c1a-4c2b-9d3e-5f6a7b8c9d0e"),
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      uuid.MustParse("60601fee-2bf1-4721-ae6f-7636e79a0cba"),
		StartDate:   domain.NewMonthYear(2025, time.July),
		EndDate:     &end,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

func TestCreateHandler_Success(t *testing.T) {
	expected := sampleSubscription()

	app := setupApp(&mockService{
		createFunc: func(_ context.Context, in domain.SubscriptionInput) (domain.Subscription, error) {
			assert.Equal(t, "Yandex Plus", in.ServiceName)
			assert.Equal(t, 400, in.Price)
			assert.Equal(t, expected.UserID, in.UserID)
			assert.Equal(t, domain.NewMonthYear(2025, time.July), in.StartDate)
			assert.Nil(t, in.EndDate)
			return expected, nil
		},
	})

	resp := do(t, app, http.MethodPost, "/subscriptions", validRequest())
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)

	body := decode[subscriptionResponse](t, resp)
	assert.Equal(t, expected.ID, body.ID)
	assert.Equal(t, "07-2025", body.StartDate)
	require.NotNil(t, body.EndDate)
	assert.Equal(t, "12-2025", *body.EndDate)
}

func TestCreateHandler_WithEndDate(t *testing.T) {
	end := "12-2025"
	req := validRequest()
	req.EndDate = &end

	app := setupApp(&mockService{
		createFunc: func(_ context.Context, in domain.SubscriptionInput) (domain.Subscription, error) {
			require.NotNil(t, in.EndDate)
			assert.Equal(t, domain.NewMonthYear(2025, time.December), *in.EndDate)
			return sampleSubscription(), nil
		},
	})

	resp := do(t, app, http.MethodPost, "/subscriptions", req)
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
}

func TestCreateHandler_InvalidJSON(t *testing.T) {
	app := setupApp(&mockService{})

	resp := do(t, app, http.MethodPost, "/subscriptions", []byte(`{"service_name": "broken"`))
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "invalid request body", decode[errorResponse](t, resp).Error)
}

func TestCreateHandler_ValidationFailures(t *testing.T) {
	app := setupApp(&mockService{})

	cases := map[string]func(r *subscriptionRequest){
		"empty_service_name": func(r *subscriptionRequest) { r.ServiceName = "" },
		"missing_price":      func(r *subscriptionRequest) { r.Price = nil },
		"negative_price":     func(r *subscriptionRequest) { r.Price = price(-1) },
		"empty_user_id":      func(r *subscriptionRequest) { r.UserID = "" },
		"invalid_user_id":    func(r *subscriptionRequest) { r.UserID = "not-a-uuid" },
		"missing_start_date": func(r *subscriptionRequest) { r.StartDate = "" },
		"bad_start_date":     func(r *subscriptionRequest) { r.StartDate = "2025-07" },
		"bad_end_date":       func(r *subscriptionRequest) { end := "13-2025"; r.EndDate = &end },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validRequest()
			mutate(&req)

			resp := do(t, app, http.MethodPost, "/subscriptions", req)
			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
			assert.NotEmpty(t, decode[errorResponse](t, resp).Error)
		})
	}
}

func TestCreateHandler_ZeroPriceAllowed(t *testing.T) {
	req := validRequest()
	req.Price = price(0)

	app := setupApp(&mockService{
		createFunc: func(_ context.Context, in domain.SubscriptionInput) (domain.Subscription, error) {
			assert.Equal(t, 0, in.Price)
			return sampleSubscription(), nil
		},
	})

	resp := do(t, app, http.MethodPost, "/subscriptions", req)
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
}

func TestCreateHandler_InvalidPeriod(t *testing.T) {
	app := setupApp(&mockService{
		createFunc: func(_ context.Context, _ domain.SubscriptionInput) (domain.Subscription, error) {
			return domain.Subscription{}, model.ErrInvalidPeriod
		},
	})

	resp := do(t, app, http.MethodPost, "/subscriptions", validRequest())
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, model.ErrInvalidPeriod.Error(), decode[errorResponse](t, resp).Error)
}

func TestCreateHandler_ServiceError(t *testing.T) {
	app := setupApp(&mockService{
		createFunc: func(_ context.Context, _ domain.SubscriptionInput) (domain.Subscription, error) {
			return domain.Subscription{}, errors.New("boom")
		},
	})

	resp := do(t, app, http.MethodPost, "/subscriptions", validRequest())
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "failed to create subscription", decode[errorResponse](t, resp).Error)
}

func TestGetHandler_Success(t *testing.T) {
	expected := sampleSubscription()

	app := setupApp(&mockService{
		getFunc: func(_ context.Context, id uuid.UUID) (domain.Subscription, error) {
			assert.Equal(t, expected.ID, id)
			return expected, nil
		},
	})

	resp := do(t, app, http.MethodGet, "/subscriptions/"+expected.ID.String(), nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body := decode[subscriptionResponse](t, resp)
	assert.Equal(t, expected.ServiceName, body.ServiceName)
	assert.Equal(t, expected.Price, body.Price)
}

func TestGetHandler_InvalidID(t *testing.T) {
	app := setupApp(&mockService{})

	resp := do(t, app, http.MethodGet, "/subscriptions/not-a-uuid", nil)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "invalid subscription id", decode[errorResponse](t, resp).Error)
}

func TestGetHandler_NotFound(t *testing.T) {
	app := setupApp(&mockService{
		getFunc: func(_ context.Context, _ uuid.UUID) (domain.Subscription, error) {
			return domain.Subscription{}, model.ErrNotFound
		},
	})

	resp := do(t, app, http.MethodGet, "/subscriptions/"+uuid.NewString(), nil)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	assert.Equal(t, model.ErrNotFound.Error(), decode[errorResponse](t, resp).Error)
}

func TestUpdateHandler_Success(t *testing.T) {
	expected := sampleSubscription()

	app := setupApp(&mockService{
		updateFunc: func(_ context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error) {
			assert.Equal(t, expected.ID, id)
			assert.Equal(t, "Yandex Plus", in.ServiceName)
			return expected, nil
		},
	})

	resp := do(t, app, http.MethodPut, "/subscriptions/"+expected.ID.String(), validRequest())
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, expected.ID, decode[subscriptionResponse](t, resp).ID)
}

func TestUpdateHandler_NotFound(t *testing.T) {
	app := setupApp(&mockService{
		updateFunc: func(_ context.Context, _ uuid.UUID, _ domain.SubscriptionInput) (domain.Subscription, error) {
			return domain.Subscription{}, model.ErrNotFound
		},
	})

	resp := do(t, app, http.MethodPut, "/subscriptions/"+uuid.NewString(), validRequest())
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestDeleteHandler_Success(t *testing.T) {
	id := uuid.New()

	app := setupApp(&mockService{
		deleteFunc: func(_ context.Context, got uuid.UUID) error {
			assert.Equal(t, id, got)
			return nil
		},
	})

	resp := do(t, app, http.MethodDelete, "/subscriptions/"+id.String(), nil)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestDeleteHandler_NotFound(t *testing.T) {
	app := setupApp(&mockService{
		deleteFunc: func(_ context.Context, _ uuid.UUID) error {
			return model.ErrNotFound
		},
	})

	resp := do(t, app, http.MethodDelete, "/subscriptions/"+uuid.NewString(), nil)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestListHandler_Success(t *testing.T) {
	userID := uuid.New()

	app := setupApp(&mockService{
		listFunc: func(_ context.Context, params domain.ListParams) ([]domain.Subscription, error) {
			require.NotNil(t, params.UserID)
			assert.Equal(t, userID, *params.UserID)
			require.NotNil(t, params.ServiceName)
			assert.Equal(t, "Yandex Plus", *params.ServiceName)
			assert.Equal(t, 10, params.Limit)
			assert.Equal(t, 5, params.Offset)
			return []domain.Subscription{sampleSubscription()}, nil
		},
	})

	resp := do(t, app, http.MethodGet,
		"/subscriptions?user_id="+userID.String()+"&service_name=Yandex%20Plus&limit=10&offset=5", nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body := decode[listResponse](t, resp)
	assert.Equal(t, 1, body.Count)
	require.Len(t, body.Items, 1)
	assert.Equal(t, "07-2025", body.Items[0].StartDate)
}

func TestListHandler_NoFilters(t *testing.T) {
	app := setupApp(&mockService{
		listFunc: func(_ context.Context, params domain.ListParams) ([]domain.Subscription, error) {
			assert.Nil(t, params.UserID)
			assert.Nil(t, params.ServiceName)
			return nil, nil
		},
	})

	resp := do(t, app, http.MethodGet, "/subscriptions", nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body := decode[listResponse](t, resp)
	assert.Equal(t, 0, body.Count)
	assert.Empty(t, body.Items)
}

func TestListHandler_InvalidUserID(t *testing.T) {
	app := setupApp(&mockService{})

	resp := do(t, app, http.MethodGet, "/subscriptions?user_id=oops", nil)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "invalid user_id", decode[errorResponse](t, resp).Error)
}

func TestCostHandler_Success(t *testing.T) {
	userID := uuid.New()

	app := setupApp(&mockService{
		costFunc: func(_ context.Context, params domain.CostParams) (int64, error) {
			assert.Equal(t, domain.NewMonthYear(2025, time.July), params.From)
			assert.Equal(t, domain.NewMonthYear(2025, time.December), params.To)
			require.NotNil(t, params.UserID)
			assert.Equal(t, userID, *params.UserID)
			require.NotNil(t, params.ServiceName)
			assert.Equal(t, "Netflix", *params.ServiceName)
			return 2400, nil
		},
	})

	resp := do(t, app, http.MethodGet,
		"/subscriptions/cost?from=07-2025&to=12-2025&user_id="+userID.String()+"&service_name=Netflix", nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body := decode[costResponse](t, resp)
	assert.Equal(t, int64(2400), body.Total)
	assert.Equal(t, "RUB", body.Currency)
	assert.Equal(t, "07-2025", body.From)
	assert.Equal(t, "12-2025", body.To)
}

func TestCostHandler_MissingOrInvalidPeriod(t *testing.T) {
	app := setupApp(&mockService{})

	for name, query := range map[string]string{
		"no_params":  "",
		"no_to":      "?from=07-2025",
		"no_from":    "?to=12-2025",
		"bad_format": "?from=2025-07&to=12-2025",
		"bad_month":  "?from=00-2025&to=12-2025",
	} {
		t.Run(name, func(t *testing.T) {
			resp := do(t, app, http.MethodGet, "/subscriptions/cost"+query, nil)
			assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
			assert.NotEmpty(t, decode[errorResponse](t, resp).Error)
		})
	}
}

func TestCostHandler_ToBeforeFrom(t *testing.T) {
	app := setupApp(&mockService{
		costFunc: func(_ context.Context, _ domain.CostParams) (int64, error) {
			return 0, model.ErrInvalidPeriod
		},
	})

	resp := do(t, app, http.MethodGet, "/subscriptions/cost?from=12-2025&to=07-2025", nil)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, model.ErrInvalidPeriod.Error(), decode[errorResponse](t, resp).Error)
}

// Маршрут /subscriptions/cost не должен перехватываться /subscriptions/:id.
func TestCostHandler_NotShadowedByGetByID(t *testing.T) {
	app := setupApp(&mockService{
		costFunc: func(_ context.Context, _ domain.CostParams) (int64, error) {
			return 100, nil
		},
		getFunc: func(_ context.Context, _ uuid.UUID) (domain.Subscription, error) {
			t.Fatal("cost request must not be routed to Get")
			return domain.Subscription{}, nil
		},
	})

	resp := do(t, app, http.MethodGet, "/subscriptions/cost?from=07-2025&to=07-2025", nil)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestHealthHandler_OK(t *testing.T) {
	app := setupApp(&mockService{})

	resp := do(t, app, http.MethodGet, "/health", nil)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", decode[healthResponse](t, resp).Status)
}

func TestHealthHandler_Unavailable(t *testing.T) {
	app := setupApp(&mockService{
		pingFunc: func(_ context.Context) error {
			return errors.New("db down")
		},
	})

	resp := do(t, app, http.MethodGet, "/health", nil)
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
}

func TestRequestLogger_SetsRequestID(t *testing.T) {
	app := setupApp(&mockService{})

	resp := do(t, app, http.MethodGet, "/health", nil)
	assert.NotEmpty(t, resp.Header.Get(requestIDHeader))
}
