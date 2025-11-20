package handler

import (
	"myapp/internal/service/auth"

	"github.com/labstack/echo/v4"
)

// RegisterNotificationRoutes registers notification routes
func RegisterNotificationRoutes(e *echo.Echo, handler *NotificationHandler, authHandler *auth.AuthHandler) {
	// Protected routes (authentication required)
	protectedGroup := e.Group("/api/v1/notifications")
	if authHandler != nil {
		protectedGroup.Use(auth.JWTMiddleware(authHandler))
	}

	// Create notification (admin only in production)
	protectedGroup.POST("", handler.CreateNotification)

	// Get failed notifications for a user
	protectedGroup.GET("/users/:user_id/failed", handler.GetFailedNotifications)
	protectedGroup.GET("/failed", handler.GetFailedNotifications) // Current user's failed notifications

	// Retry a failed notification
	protectedGroup.POST("/:id/retry", handler.RetryNotification)
}

