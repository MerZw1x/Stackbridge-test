package handler

import (
	"backend/internal/domain"
	"backend/internal/model"
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	fiberSwagger "github.com/swaggo/fiber-swagger"
)

const maxServiceNameLen = 255

type SubscriptionsService interface {
	Create(ctx context.Context, in domain.SubscriptionInput) (domain.Subscription, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Subscription, error)
	Update(ctx context.Context, id uuid.UUID, in domain.SubscriptionInput) (domain.Subscription, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, params domain.ListParams) ([]domain.Subscription, error)
	Cost(ctx context.Context, params domain.CostParams) (int64, error)
	Ping(ctx context.Context) error
}

type SubscriptionsHandler struct {
	service SubscriptionsService
	log     *slog.Logger
}

func NewSubscriptionsHandler(service SubscriptionsService, log *slog.Logger) *SubscriptionsHandler {
	return &SubscriptionsHandler{service: service, log: log}
}

func (h *SubscriptionsHandler) Register(app *fiber.App) {
	app.Use(RequestLogger(h.log))

	app.Get("/health", h.Health)
	app.Get("/swagger/*", fiberSwagger.WrapHandler)

	subs := app.Group("/subscriptions")
	subs.Post("/", h.Create)
	subs.Get("/", h.List)
	// Статический /cost регистрируется до /:id, иначе "cost" попадёт в параметр.
	subs.Get("/cost", h.Cost)
	subs.Get("/:id", h.Get)
	subs.Put("/:id", h.Update)
	subs.Delete("/:id", h.Delete)
}

// subscriptionRequest — тело запроса на создание и обновление подписки.
type subscriptionRequest struct {
	ServiceName string  `json:"service_name" example:"Yandex Plus"`
	Price       *int    `json:"price" example:"400"`
	UserID      string  `json:"user_id" example:"60601fee-2bf1-4721-ae6f-7636e79a0cba"`
	StartDate   string  `json:"start_date" example:"07-2025"`
	EndDate     *string `json:"end_date,omitempty" example:"12-2025"`
} // @name SubscriptionRequest

// subscriptionResponse — представление подписки в ответах API.
type subscriptionResponse struct {
	ID          uuid.UUID `json:"id" example:"9b7c2f5e-1c1a-4c2b-9d3e-5f6a7b8c9d0e"`
	ServiceName string    `json:"service_name" example:"Yandex Plus"`
	Price       int       `json:"price" example:"400"`
	UserID      uuid.UUID `json:"user_id" example:"60601fee-2bf1-4721-ae6f-7636e79a0cba"`
	StartDate   string    `json:"start_date" example:"07-2025"`
	EndDate     *string   `json:"end_date,omitempty" example:"12-2025"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
} // @name Subscription

// listResponse — страница списка подписок.
type listResponse struct {
	Items  []subscriptionResponse `json:"items"`
	Count  int                    `json:"count" example:"2"`
	Limit  int                    `json:"limit" example:"50"`
	Offset int                    `json:"offset" example:"0"`
} // @name SubscriptionList

// costResponse — суммарная стоимость подписок за период.
type costResponse struct {
	Total    int64  `json:"total" example:"2400"`
	Currency string `json:"currency" example:"RUB"`
	From     string `json:"from" example:"07-2025"`
	To       string `json:"to" example:"12-2025"`
} // @name SubscriptionsCost

// errorResponse — тело ответа при ошибке.
type errorResponse struct {
	Error string `json:"error" example:"invalid request body"`
} // @name Error

// healthResponse — состояние сервиса.
type healthResponse struct {
	Status string `json:"status" example:"ok"`
} // @name Health

// Health godoc
//
//	@Summary		Проверка живости сервиса
//	@Description	Проверяет доступность хранилища. Для postgres выполняется ping пула соединений.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	healthResponse
//	@Failure		503	{object}	errorResponse
//	@Router			/health [get]
func (h *SubscriptionsHandler) Health(c *fiber.Ctx) error {
	if err := h.service.Ping(c.Context()); err != nil {
		h.log.Error("health check failed", "error", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(errorResponse{Error: "unavailable"})
	}
	return c.JSON(healthResponse{Status: "ok"})
}

// Create godoc
//
//	@Summary		Создать подписку
//	@Description	Создаёт запись о подписке пользователя. Даты передаются в формате MM-YYYY, end_date опционален.
//	@Tags			subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			request	body		subscriptionRequest	true	"Данные подписки"
//	@Success		201		{object}	subscriptionResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Router			/subscriptions [post]
func (h *SubscriptionsHandler) Create(c *fiber.Ctx) error {
	in, err := parseInput(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: err.Error()})
	}

	sub, err := h.service.Create(c.Context(), in)
	if err != nil {
		return h.fail(c, err, "create subscription failed", "failed to create subscription")
	}

	return c.Status(fiber.StatusCreated).JSON(toResponse(sub))
}

// Get godoc
//
//	@Summary		Получить подписку по ID
//	@Tags			subscriptions
//	@Produce		json
//	@Param			id	path		string	true	"ID подписки"	format(uuid)
//	@Success		200	{object}	subscriptionResponse
//	@Failure		400	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Failure		500	{object}	errorResponse
//	@Router			/subscriptions/{id} [get]
func (h *SubscriptionsHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: "invalid subscription id"})
	}

	sub, err := h.service.Get(c.Context(), id)
	if err != nil {
		return h.fail(c, err, "get subscription failed", "failed to get subscription")
	}

	return c.JSON(toResponse(sub))
}

// Update godoc
//
//	@Summary		Обновить подписку
//	@Description	Полностью заменяет запись о подписке.
//	@Tags			subscriptions
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"ID подписки"	format(uuid)
//	@Param			request	body		subscriptionRequest	true	"Новые данные подписки"
//	@Success		200		{object}	subscriptionResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Router			/subscriptions/{id} [put]
func (h *SubscriptionsHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: "invalid subscription id"})
	}

	in, err := parseInput(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: err.Error()})
	}

	sub, err := h.service.Update(c.Context(), id, in)
	if err != nil {
		return h.fail(c, err, "update subscription failed", "failed to update subscription")
	}

	return c.JSON(toResponse(sub))
}

// Delete godoc
//
//	@Summary		Удалить подписку
//	@Tags			subscriptions
//	@Produce		json
//	@Param			id	path	string	true	"ID подписки"	format(uuid)
//	@Success		204	"Подписка удалена"
//	@Failure		400	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Failure		500	{object}	errorResponse
//	@Router			/subscriptions/{id} [delete]
func (h *SubscriptionsHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: "invalid subscription id"})
	}

	if err := h.service.Delete(c.Context(), id); err != nil {
		return h.fail(c, err, "delete subscription failed", "failed to delete subscription")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// List godoc
//
//	@Summary		Список подписок
//	@Description	Возвращает подписки с опциональной фильтрацией по пользователю и названию сервиса.
//	@Tags			subscriptions
//	@Produce		json
//	@Param			user_id			query		string	false	"ID пользователя"	format(uuid)
//	@Param			service_name	query		string	false	"Название сервиса (без учёта регистра)"
//	@Param			limit			query		int		false	"Размер страницы (1..500, по умолчанию 50)"
//	@Param			offset			query		int		false	"Смещение (по умолчанию 0)"
//	@Success		200				{object}	listResponse
//	@Failure		400				{object}	errorResponse
//	@Failure		500				{object}	errorResponse
//	@Router			/subscriptions [get]
func (h *SubscriptionsHandler) List(c *fiber.Ctx) error {
	userID, err := parseOptionalUUID(c.Query("user_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: "invalid user_id"})
	}

	params := domain.ListParams{
		UserID:      userID,
		ServiceName: parseOptionalString(c.Query("service_name")),
		Limit:       c.QueryInt("limit"),
		Offset:      c.QueryInt("offset"),
	}

	subs, err := h.service.List(c.Context(), params)
	if err != nil {
		return h.fail(c, err, "list subscriptions failed", "failed to list subscriptions")
	}

	items := make([]subscriptionResponse, 0, len(subs))
	for _, sub := range subs {
		items = append(items, toResponse(sub))
	}

	return c.JSON(listResponse{
		Items:  items,
		Count:  len(items),
		Limit:  params.Limit,
		Offset: params.Offset,
	})
}

// Cost godoc
//
//	@Summary		Суммарная стоимость подписок за период
//	@Description	Считает сумму всех подписок, попадающих в период [from, to] включительно.
//	@Description	Стоимость подписки учитывается за каждый месяц пересечения с периодом.
//	@Description	Подписка без даты окончания считается активной до конца периода.
//	@Tags			subscriptions
//	@Produce		json
//	@Param			from			query		string	true	"Начало периода, MM-YYYY"	example(07-2025)
//	@Param			to				query		string	true	"Конец периода, MM-YYYY"	example(12-2025)
//	@Param			user_id			query		string	false	"ID пользователя"			format(uuid)
//	@Param			service_name	query		string	false	"Название сервиса (без учёта регистра)"
//	@Success		200				{object}	costResponse
//	@Failure		400				{object}	errorResponse
//	@Failure		500				{object}	errorResponse
//	@Router			/subscriptions/cost [get]
func (h *SubscriptionsHandler) Cost(c *fiber.Ctx) error {
	from, err := domain.ParseMonthYear(c.Query("from"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: `invalid "from": expected format MM-YYYY`})
	}

	to, err := domain.ParseMonthYear(c.Query("to"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: `invalid "to": expected format MM-YYYY`})
	}

	userID, err := parseOptionalUUID(c.Query("user_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: "invalid user_id"})
	}

	total, err := h.service.Cost(c.Context(), domain.CostParams{
		UserID:      userID,
		ServiceName: parseOptionalString(c.Query("service_name")),
		From:        from,
		To:          to,
	})
	if err != nil {
		return h.fail(c, err, "calculate cost failed", "failed to calculate cost")
	}

	return c.JSON(costResponse{
		Total:    total,
		Currency: "RUB",
		From:     from.String(),
		To:       to.String(),
	})
}

// parseInput разбирает и валидирует тело запроса. Любая ошибка здесь — это 400.
func parseInput(c *fiber.Ctx) (domain.SubscriptionInput, error) {
	var req subscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.SubscriptionInput{}, errors.New("invalid request body")
	}

	if err := validateSubscriptionReq(req); err != nil {
		return domain.SubscriptionInput{}, err
	}

	return req.toInput()
}

// fail переводит доменную ошибку в HTTP-ответ и пишет лог для неожиданных случаев.
func (h *SubscriptionsHandler) fail(c *fiber.Ctx, err error, logMsg, userMsg string) error {
	switch {
	case errors.Is(err, model.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(errorResponse{Error: model.ErrNotFound.Error()})
	case errors.Is(err, model.ErrInvalidPeriod):
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse{Error: model.ErrInvalidPeriod.Error()})
	default:
		h.log.Error(logMsg, "path", c.Path(), "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(errorResponse{Error: userMsg})
	}
}

func validateSubscriptionReq(req subscriptionRequest) error {
	return validation.ValidateStruct(&req,
		validation.Field(&req.ServiceName, validation.Required, validation.Length(1, maxServiceNameLen)),
		validation.Field(&req.Price, validation.NotNil, validation.Min(0)),
		validation.Field(&req.UserID, validation.Required, is.UUIDv4),
		validation.Field(&req.StartDate, validation.Required, validation.By(isMonthYear)),
		validation.Field(&req.EndDate, validation.By(isMonthYear)),
	)
}

func isMonthYear(value any) error {
	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case *string:
		if v == nil {
			return nil
		}
		raw = *v
	default:
		return domain.ErrInvalidMonthYear
	}

	if raw == "" {
		return nil
	}

	_, err := domain.ParseMonthYear(raw)
	return err
}

func (req subscriptionRequest) toInput() (domain.SubscriptionInput, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return domain.SubscriptionInput{}, errors.New("invalid user_id")
	}

	startDate, err := domain.ParseMonthYear(req.StartDate)
	if err != nil {
		return domain.SubscriptionInput{}, err
	}

	in := domain.SubscriptionInput{
		ServiceName: strings.TrimSpace(req.ServiceName),
		Price:       *req.Price,
		UserID:      userID,
		StartDate:   startDate,
	}

	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		endDate, err := domain.ParseMonthYear(*req.EndDate)
		if err != nil {
			return domain.SubscriptionInput{}, err
		}
		in.EndDate = &endDate
	}

	return in, nil
}

func toResponse(sub domain.Subscription) subscriptionResponse {
	resp := subscriptionResponse{
		ID:          sub.ID,
		ServiceName: sub.ServiceName,
		Price:       sub.Price,
		UserID:      sub.UserID,
		StartDate:   sub.StartDate.String(),
		CreatedAt:   sub.CreatedAt,
		UpdatedAt:   sub.UpdatedAt,
	}

	if sub.EndDate != nil {
		end := sub.EndDate.String()
		resp.EndDate = &end
	}

	return resp
}

func parseOptionalUUID(raw string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseOptionalString(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return &raw
}
