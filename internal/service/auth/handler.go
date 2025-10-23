package auth

import (
	"net/http"

	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/server"

	"github.com/labstack/echo/v4"
)

// AuthHandler handles authentication HTTP requests
type AuthHandler struct {
	service *AuthService
	logger  *logger.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(service *AuthService, log *logger.Logger) *AuthHandler {
	return &AuthHandler{
		service: service,
		logger:  log,
	}
}

// Register handles user registration
func (h *AuthHandler) Register(c echo.Context) error {
	var dto RegisterDTO
	if err := c.Bind(&dto); err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid request body")
	}

	// Basic validation
	if dto.Email == "" || dto.Password == "" || dto.Name == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "Email, password, and name are required")
	}

	user, err := h.service.Register(dto)
	if err != nil {
		h.logger.Error("Registration failed")
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Registration failed")
	}

	return server.SuccessResponse(c, http.StatusCreated, user.ToUserResponse(), "User registered successfully")
}

// Login handles user login
func (h *AuthHandler) Login(c echo.Context) error {
	var dto LoginDTO
	if err := c.Bind(&dto); err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid request body")
	}

	// Basic validation
	if dto.Email == "" || dto.Password == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "Email and password are required")
	}

	tokenResponse, err := h.service.Login(dto)
	if err != nil {
		h.logger.Error("Login failed")
		return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Login failed")
	}

	return server.SuccessResponse(c, http.StatusOK, tokenResponse, "Login successful")
}

// GetProfile returns the authenticated user's profile
func (h *AuthHandler) GetProfile(c echo.Context) error {
	userCtx, err := GetUserFromContext(c)
	if err != nil {
		return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Unauthorized")
	}

	user, err := h.service.GetUserByID(userCtx.UserID)
	if err != nil {
		h.logger.Error("Failed to get user profile")
		return server.ErrorResponse(c, http.StatusNotFound, err.Error(), "User not found")
	}

	return server.SuccessResponse(c, http.StatusOK, user.ToUserResponse(), "Profile retrieved successfully")
}
