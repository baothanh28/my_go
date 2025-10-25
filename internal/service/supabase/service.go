package supabase

import (
	"errors"
	"fmt"
	"time"

	"myapp/internal/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
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

// ExchangeSupabaseToken validates Supabase access token and issues app JWT
func (s *Service) ExchangeSupabaseToken(dto SupabaseLoginDTO) (*TokenResponse, error) {
	claims, err := s.validateSupabaseJWT(dto.SupabaseAccessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid Supabase token: %w", err)
	}

	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if email == "" {
		return nil, errors.New("email missing in token")
	}

	user, err := s.repo.GetByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = &User{Email: email, Name: name, Role: "user"}
			if err := s.repo.Create(user); err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to fetch user: %w", err)
		}
	}

	token, exp, err := s.generateAppJWT(user)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{Token: token, ExpiresAt: exp, User: user.ToUserResponse()}, nil
}

// GetOrCreateUser returns a user by context or creates one if needed
func (s *Service) GetOrCreateUser(ctx *UserContext) (*User, error) {
	user, err := s.repo.GetByEmail(ctx.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = &User{Email: ctx.Email, Name: ctx.Name, Role: ctx.Role}
			if err := s.repo.Create(user); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return user, nil
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
