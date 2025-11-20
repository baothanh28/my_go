package supabase

import (
	"errors"
	"fmt"
	"time"

	"myapp/internal/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
)

// Service encapsulates Supabase login business logic
type Service struct {
	repo   *Repository
	config *ServiceConfig
	logger *logger.Logger
}

// NewService constructs Service
func NewService(repo *Repository, cfg *ServiceConfig, log *logger.Logger) *Service {
	return &Service{repo: repo, config: cfg, logger: log}
}

// validateSupabaseJWT validates a Supabase JWT with the anon key
func (s *Service) validateSupabaseJWT(tokenString string) (jwt.MapClaims, error) {
	key := []byte(s.config.Supabase.AnonKey)
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return key, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

// generateAppJWT issues the app's JWT using shared JWT config
func (s *Service) generateAppJWT(user *User) (string, time.Time, error) {
	exp := time.Now().Add(time.Duration(s.config.JWT.ExpireHour) * time.Hour)
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"name":    user.Name,
		"exp":     exp.Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.config.JWT.Secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}
	return signed, exp, nil
}

// ValidateToken validates app JWT and returns context
func (s *Service) ValidateToken(tokenString string) (*UserContext, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.config.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	ctx := &UserContext{
		UserID: uint(claims["user_id"].(float64)),
		Email:  claims["email"].(string),
		Role:   claims["role"].(string),
		Name:   claims["name"].(string),
	}
	return ctx, nil
}

// GetUserPermissions retrieves the role and merged permissions for a given Supabase user id
func (s *Service) GetUserPermissions(supabaseUserID string) (*PermissionsResponse, error) {
	role, perms, err := s.repo.GetUserRoleAndPermissions(supabaseUserID)
	if err != nil {
		return nil, err
	}
	return &PermissionsResponse{Role: role, Permissions: perms}, nil
}
