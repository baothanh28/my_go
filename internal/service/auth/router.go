package auth

import (
	"github.com/labstack/echo/v4"
)

// RegisterAuthRoutes registers authentication routes
func RegisterAuthRoutes(e *echo.Echo, handler *AuthHandler, authService *AuthService) {
	// Public routes (no authentication required)
	authGroup := e.Group("/api/v1/auth")
	authGroup.POST("/register", handler.Register)
	authGroup.POST("/login", handler.Login)

	// Protected routes (authentication required)
	protectedGroup := e.Group("/api/v1/auth")
	protectedGroup.Use(JWTMiddleware(authService))
	protectedGroup.GET("/profile", handler.GetProfile)
}
