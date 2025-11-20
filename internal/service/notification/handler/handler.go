package handler

import (
	"net/http"
	"strconv"

	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/server"
	"myapp/internal/service/auth"
	"myapp/internal/service/notification/model"
	"myapp/internal/service/notification/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// NotificationHandler handles notification HTTP requests
type NotificationHandler struct {
	service *service.NotificationService
	logger  *logger.Logger
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(service *service.NotificationService, log *logger.Logger) *NotificationHandler {
	return &NotificationHandler{
		service: service,
		logger:  log,
	}
}

// CreateNotification handles notification creation
func (h *NotificationHandler) CreateNotification(c echo.Context) error {
	var dto model.CreateNotificationDTO
	if err := c.Bind(&dto); err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid request body")
	}

	// Basic validation
	if dto.Type == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "Type is required")
	}
	if len(dto.Targets) == 0 {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "At least one target is required")
	}

	notif, err := h.service.CreateNotification(dto)
	if err != nil {
		h.logger.Error("Failed to create notification", zap.Error(err))
		return server.ErrorResponse(c, http.StatusInternalServerError, err.Error(), "Failed to create notification")
	}

	return server.SuccessResponse(c, http.StatusCreated, notif, "Notification created successfully")
}

// GetFailedNotifications handles retrieval of failed notifications for a user
func (h *NotificationHandler) GetFailedNotifications(c echo.Context) error {
	// Get query parameters
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 20
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get user ID from path or context
	userID := c.Param("user_id")
	if userID == "" {
		// Try to get from auth context if available
		userCtx, err := auth.GetUserFromContext(c)
		if err == nil {
			userID = strconv.FormatUint(uint64(userCtx.UserID), 10)
		} else {
			return server.ErrorResponse(c, http.StatusBadRequest, nil, "user_id is required")
		}
	}

	// Verify user can only access their own notifications (if auth is available)
	if userCtx, err := auth.GetUserFromContext(c); err == nil {
		if userID != strconv.FormatUint(uint64(userCtx.UserID), 10) && userCtx.Role != "admin" {
			return server.ErrorResponse(c, http.StatusForbidden, nil, "Forbidden")
		}
	}

	notifications, err := h.service.GetFailedNotifications(userID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get failed notifications", zap.Error(err))
		return server.ErrorResponse(c, http.StatusInternalServerError, err.Error(), "Failed to get failed notifications")
	}

	return server.SuccessResponse(c, http.StatusOK, notifications, "Failed notifications retrieved successfully")
}

// RetryNotification handles retry of a failed notification
func (h *NotificationHandler) RetryNotification(c echo.Context) error {
	// Get notification ID from path
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid notification ID")
	}

	if err := h.service.RetryNotification(id); err != nil {
		userID := "unknown"
		if userCtx, err := auth.GetUserFromContext(c); err == nil {
			userID = strconv.FormatUint(uint64(userCtx.UserID), 10)
		}
		h.logger.Error("Failed to retry notification",
			zap.Error(err),
			zap.Int64("notification_id", id),
			zap.String("user_id", userID),
		)
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Failed to retry notification")
	}

	return server.SuccessResponse(c, http.StatusOK, map[string]interface{}{
		"notification_id": id,
		"status":          "retry_requested",
	}, "Notification retry requested successfully")
}

// RegisterToken handles device token registration
func (h *NotificationHandler) RegisterToken(c echo.Context) error {
	var dto model.RegisterTokenDTO
	if err := c.Bind(&dto); err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid request body")
	}

	// Validation
	if dto.UserID == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "user_id is required")
	}
	if dto.DeviceID == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "device_id is required")
	}
	if dto.PushToken == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "push_token is required")
	}
	if dto.Type == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "type is required (expo, fcm, apns, native)")
	}
	if dto.Platform == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "platform is required (ios, android, web)")
	}

	// Optional: Verify user from auth context
	if userCtx, err := auth.GetUserFromContext(c); err == nil {
		if dto.UserID != strconv.FormatUint(uint64(userCtx.UserID), 10) && userCtx.Role != "admin" {
			return server.ErrorResponse(c, http.StatusForbidden, nil, "Forbidden")
		}
	}

	result, err := h.service.RegisterDeviceToken(dto)
	if err != nil {
		h.logger.Error("Failed to register device token", zap.Error(err))
		return server.ErrorResponse(c, http.StatusInternalServerError, err.Error(), "Failed to register device token")
	}

	return server.SuccessResponse(c, http.StatusOK, result, "Device token registered successfully")
}

