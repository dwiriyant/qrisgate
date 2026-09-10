package admin

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/qrisgate/qrisgate/internal/domain"
	"github.com/qrisgate/qrisgate/internal/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) CreateApp(c echo.Context) error {
	var in CreateAppInput
	if err := c.Bind(&in); err != nil {
		return response.BadRequest(c, "invalid json")
	}
	out, err := h.svc.CreateApp(c.Request().Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			return response.BadRequest(c, "name and merchant_qris are required")
		case errors.Is(err, domain.ErrInvalidQRIS):
			return response.BadRequest(c, "invalid merchant_qris")
		default:
			return response.Internal(c)
		}
	}
	return response.Created(c, out)
}

func (h *Handler) CreateWebhook(c echo.Context) error {
	var in CreateWebhookInput
	if err := c.Bind(&in); err != nil {
		return response.BadRequest(c, "invalid json")
	}
	out, err := h.svc.CreateWebhook(c.Request().Context(), c.Param("id"), in)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			return response.BadRequest(c, "url must be a valid https URL")
		case errors.Is(err, domain.ErrNotFound):
			return response.NotFound(c)
		default:
			return response.Internal(c)
		}
	}
	return response.Created(c, out)
}

func (h *Handler) UpdateApp(c echo.Context) error {
	var in UpdateAppInput
	if err := c.Bind(&in); err != nil {
		return response.BadRequest(c, "invalid json")
	}
	out, err := h.svc.UpdateApp(c.Request().Context(), c.Param("id"), in)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			return response.BadRequest(c, "merchant_qris is required")
		case errors.Is(err, domain.ErrInvalidQRIS):
			return response.BadRequest(c, "invalid merchant_qris")
		case errors.Is(err, domain.ErrNotFound):
			return response.NotFound(c)
		default:
			return response.Internal(c)
		}
	}
	return response.OK(c, out)
}

func AdminAuth(token string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if token == "" {
				return c.JSON(http.StatusServiceUnavailable, response.ErrorBody{Error: "admin not configured"})
			}
			auth := c.Request().Header.Get("Authorization")
			const prefix = "Bearer "
			if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix || auth[len(prefix):] != token {
				return response.Unauthorized(c)
			}
			return next(c)
		}
	}
}
