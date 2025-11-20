package supabase

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

// JWTMiddleware creates a JWT authentication middleware for app tokens
func JWTMiddleware(handler *Handler) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return server.ErrorResponse(c, http.StatusUnauthorized, nil, "Missing authorization header")
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return server.ErrorResponse(c, http.StatusUnauthorized, nil, "Invalid authorization header format")
			}
			tokenString := parts[1]

			// First, try validating as our application JWT
			userContext, err := handler.service.ValidateToken(tokenString)
			if err != nil {
				// Fallback: try validating as a Supabase access token and map claims
				claims, sErr := handler.service.validateSupabaseJWT(tokenString)
				if sErr != nil {
					return server.ErrorResponse(c, http.StatusUnauthorized, sErr.Error(), "Invalid or expired token")
				}

				var (
					email string
					name  string
					role  string
					sub   string
					aud   string
					appMd map[string]any
					usrMd map[string]any
				)

				if v, ok := claims["email"].(string); ok {
					email = v
				}
				if v, ok := claims["name"].(string); ok {
					name = v
				}
				if v, ok := claims["role"].(string); ok {
					role = v
				}
				if v, ok := claims["sub"].(string); ok {
					sub = v
				}
				if v, ok := claims["aud"].(string); ok {
					aud = v
				}
				if v, ok := claims["app_metadata"].(map[string]any); ok {
					appMd = v
					// fallback role from app_metadata.role if not set
					if role == "" {
						if rv, rok := v["role"].(string); rok {
							role = rv
						}
					}
				}
				if v, ok := claims["user_metadata"].(map[string]any); ok {
					usrMd = v
					// fallback name/email from user_metadata
					if name == "" {
						if nv, nok := v["name"].(string); nok {
							name = nv
						} else if fn, fnok := v["full_name"].(string); fnok {
							name = fn
						}
					}
					if email == "" {
						if ev, eok := v["email"].(string); eok {
							email = ev
						}
					}
				}

				uc := &UserContext{
					UserID:       0,
					Email:        email,
					Role:         role,
					Name:         name,
					Sub:          sub,
					Aud:          aud,
					AppMetadata:  appMd,
					UserMetadata: usrMd,
				}
				c.Set(userContextKey, uc)
				return next(c)
			}
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
