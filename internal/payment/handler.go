package payment

import (
	"errors"

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

func (h *Handler) Create(c echo.Context) error {
	apiKey := c.Request().Header.Get("X-API-Key")
	var in CreateInput
	if err := c.Bind(&in); err != nil {
		return response.BadRequest(c, "invalid json")
	}
	out, err := h.svc.Create(c.Request().Context(), apiKey, in)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			return response.BadRequest(c, "order_id and positive amount required")
		case errors.Is(err, domain.ErrUnauthorized):
			return response.Unauthorized(c)
		case errors.Is(err, domain.ErrInvalidQRIS):
			return response.BadRequest(c, "merchant qris misconfigured")
		default:
			return response.Internal(c)
		}
	}
	return response.Created(c, out)
}

func (h *Handler) Get(c echo.Context) error {
	apiKey := c.Request().Header.Get("X-API-Key")
	out, err := h.svc.Get(c.Request().Context(), apiKey, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUnauthorized):
			return response.Unauthorized(c)
		case errors.Is(err, domain.ErrNotFound):
			return response.NotFound(c)
		default:
			return response.Internal(c)
		}
	}
	return response.OK(c, out)
}

func (h *Handler) MarkPaid(c echo.Context) error {
	out, err := h.svc.MarkPaid(c.Request().Context(), c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			return response.NotFound(c)
		case errors.Is(err, domain.ErrConflict):
			return response.Conflict(c, "payment cannot be marked paid")
		default:
			return response.Internal(c)
		}
	}
	return response.OK(c, out)
}

func (h *Handler) Claim(c echo.Context) error {
	var in ClaimInput
	if err := c.Bind(&in); err != nil {
		return response.BadRequest(c, "invalid json")
	}
	out, err := h.svc.Claim(c.Request().Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			return response.BadRequest(c, "app_id, amount, provider, and external_id required")
		case errors.Is(err, domain.ErrNotFound):
			return response.NotFound(c)
		case errors.Is(err, domain.ErrConflict):
			return response.Conflict(c, "payment cannot be claimed")
		default:
			return response.Internal(c)
		}
	}
	return response.OK(c, out)
}
