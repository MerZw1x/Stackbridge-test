package handler

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

// RequestLogger пишет структурированный лог по каждому запросу и проставляет
// идентификатор запроса, по которому лог связывается с ответом клиенту.
func RequestLogger(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := c.Get(requestIDHeader)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set(requestIDHeader, requestID)

		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()

		attrs := []any{
			"request_id", requestID,
			"method", c.Method(),
			"path", c.Path(),
			"query", c.Context().QueryArgs().String(),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
		}

		switch {
		case err != nil:
			log.Error("request failed", append(attrs, "error", err)...)
		case status >= fiber.StatusInternalServerError:
			log.Error("request completed with server error", attrs...)
		case status >= fiber.StatusBadRequest:
			log.Warn("request completed with client error", attrs...)
		default:
			log.Info("request completed", attrs...)
		}

		return err
	}
}
