package auth

import (
	"fmt"

	"myapp/internal/pkg/database"
)

// AuthRepository handles database operations for users in auth domain
type AuthRepository struct {
	db *database.Database
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(db *database.Database) *AuthRepository {
	return &AuthRepository{db: db}
}

// GetByEmail retrieves a user by email
func (r *AuthRepository) GetByEmail(email string) (*User, error) {
	var user User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// GetByID retrieves a user by ID
func (r *AuthRepository) GetByID(id uint) (*User, error) {
	var user User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &user, nil
}

// Create inserts a new user
func (r *AuthRepository) Create(user *User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// ExistsByEmail checks if a user exists with the given email
func (r *AuthRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	if err := r.db.Model(&User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	return count > 0, nil
}
