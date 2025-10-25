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

// LoginWithSupabase exchanges a Supabase session JWT for an app JWT
func (h *Handler) LoginWithSupabase(c echo.Context) error {
	var dto SupabaseLoginDTO
	if err := c.Bind(&dto); err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid request body")
	}
	if dto.SupabaseAccessToken == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "supabase_access_token is required")
	}

	tokenResponse, err := h.service.ExchangeSupabaseToken(dto)
	if err != nil {
		h.logger.Error("Supabase login failed")
		return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Login failed")
	}
	return server.SuccessResponse(c, http.StatusOK, tokenResponse, "Login successful")
}

// GetProfile returns the authenticated user's profile
func (h *Handler) GetProfile(c echo.Context) error {
	userCtx, err := GetUserFromContext(c)
	if err != nil {
		return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Unauthorized")
	}
	user, err := h.service.GetOrCreateUser(userCtx)
	if err != nil {
		h.logger.Error("Failed to get user profile")
		return server.ErrorResponse(c, http.StatusNotFound, err.Error(), "User not found")
	}
	return server.SuccessResponse(c, http.StatusOK, user.ToUserResponse(), "Profile retrieved successfully")
}
