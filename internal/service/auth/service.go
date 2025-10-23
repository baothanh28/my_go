package auth

import (
	"errors"
	"fmt"
	"time"

	"myapp/internal/pkg/config"
	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService handles authentication business logic
type AuthService struct {
	db     *database.Database
	config *config.Config
	logger *logger.Logger
}

// NewAuthService creates a new auth service
func NewAuthService(db *database.Database, cfg *config.Config, log *logger.Logger) *AuthService {
	return &AuthService{
		db:     db,
		config: cfg,
		logger: log,
	}
}

// Register creates a new user account
func (s *AuthService) Register(dto RegisterDTO) (*User, error) {
	// Check if user already exists
	var existingUser User
	err := s.db.Where("email = ?", dto.Email).First(&existingUser).Error
	if err == nil {
		return nil, errors.New("user with this email already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(dto.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := User{
		Email:    dto.Email,
		Password: string(hashedPassword),
		Name:     dto.Name,
		Role:     "user",
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// Login authenticates a user and returns a JWT token
func (s *AuthService) Login(dto LoginDTO) (*TokenResponse, error) {
	// Find user by email
	var user User
	err := s.db.Where("email = ?", dto.Email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(dto.Password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Generate JWT token
	token, expiresAt, err := s.GenerateToken(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user.ToUserResponse(),
	}, nil
}

// GenerateToken generates a JWT token for a user
func (s *AuthService) GenerateToken(user *User) (string, time.Time, error) {
	expiresAt := time.Now().Add(time.Duration(s.config.JWT.ExpireHour) * time.Hour)

	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"name":    user.Name,
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.JWT.Secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *AuthService) ValidateToken(tokenString string) (*UserContext, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Extract user context from claims
	userContext := &UserContext{
		UserID: uint(claims["user_id"].(float64)),
		Email:  claims["email"].(string),
		Role:   claims["role"].(string),
		Name:   claims["name"].(string),
	}

	return userContext, nil
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(id uint) (*User, error) {
	var user User
	err := s.db.First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}
