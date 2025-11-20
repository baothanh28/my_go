package supabase

import (
	"net/http"

	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/server"

	"github.com/labstack/echo/v4"
)

// Handler handles Supabase login HTTP requests
type Handler struct {
	service *Service
	logger  *logger.Logger
}

// NewHandler creates a new handler
func NewHandler(service *Service, log *logger.Logger) *Handler {
	return &Handler{service: service, logger: log}
}

// GetUserPermissions returns role and permissions for the authenticated Supabase user
func (h *Handler) GetUserPermissions(c echo.Context) error {
	userCtx, err := GetUserFromContext(c)
	if err != nil {
		return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Unauthorized")
	}

	// Expect Supabase flow to populate Sub as the Supabase user id
	if userCtx.Sub == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "Supabase user id (sub) is required")
	}

	resp, err := h.service.GetUserPermissions(userCtx.Sub)
	if err != nil {
		h.logger.Error("Failed to get user permissions")
		return server.ErrorResponse(c, http.StatusInternalServerError, err.Error(), "Failed to fetch permissions")
	}
	return server.SuccessResponse(c, http.StatusOK, resp, "Permissions retrieved successfully")
}

// Health returns a simple health status for the Supabase service
func (h *Handler) Health(c echo.Context) error {
	return server.SuccessResponse(c, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "supabase",
	}, "Supabase service healthy")
}
