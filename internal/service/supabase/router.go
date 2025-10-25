package supabase

import (
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers Supabase login routes
func RegisterRoutes(e *echo.Echo, handler *Handler) {
	group := e.Group("/api/v1/supabase")
	group.POST("/login", handler.LoginWithSupabase)

	protected := e.Group("/api/v1/supabase")
	protected.Use(JWTMiddleware(handler))
	protected.GET("/profile", handler.GetProfile)
}
