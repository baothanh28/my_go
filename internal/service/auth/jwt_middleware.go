package auth

import (
	"errors"
	"net/http"
	"strings"

	"myapp/internal/pkg/server"

	"github.com/labstack/echo/v4"
)

const (
	userContextKey = "user"
)

// JWTMiddleware creates a JWT authentication middleware
func JWTMiddleware(authService *AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Extract token from Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return server.ErrorResponse(c, http.StatusUnauthorized, nil, "Missing authorization header")
			}

			// Check if it starts with "Bearer "
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return server.ErrorResponse(c, http.StatusUnauthorized, nil, "Invalid authorization header format")
			}

			tokenString := parts[1]

			// Validate token
			userContext, err := authService.ValidateToken(tokenString)
			if err != nil {
				return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Invalid or expired token")
			}

			// Inject user context into Echo context
			c.Set(userContextKey, userContext)

			return next(c)
		}
	}
}

// GetUserFromContext extracts user information from Echo context
func GetUserFromContext(c echo.Context) (*UserContext, error) {
	user := c.Get(userContextKey)
	if user == nil {
		return nil, errors.New("user not found in context")
	}

	userContext, ok := user.(*UserContext)
	if !ok {
		return nil, errors.New("invalid user context type")
	}

	return userContext, nil
}
