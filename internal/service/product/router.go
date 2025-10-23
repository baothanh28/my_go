package product

import (
	"myapp/internal/service/auth"

	"github.com/labstack/echo/v4"
)

// RegisterProductRoutes registers product routes
func RegisterProductRoutes(e *echo.Echo, handler *ProductHandler, authService *auth.AuthService) {
	// Product routes (all require authentication)
	productGroup := e.Group("/api/v1/products")
	productGroup.Use(auth.JWTMiddleware(authService))

	productGroup.POST("", handler.Create)
	productGroup.GET("", handler.List)
	productGroup.GET("/search", handler.Search)
	productGroup.GET("/:id", handler.Get)
	productGroup.PUT("/:id", handler.Update)
	productGroup.DELETE("/:id", handler.Delete)
}
