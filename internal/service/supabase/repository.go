package supabase

import (
	"fmt"

	"myapp/internal/pkg/database"
)

// Repository handles database operations for users in supabase_login domain
type Repository struct {
	db *database.Database
}

// NewRepository creates a new repository
func NewRepository(db *database.Database) *Repository {
	return &Repository{db: db}
}

// GetByEmail retrieves a user by email
func (r *Repository) GetByEmail(email string) (*User, error) {
	var user User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// Create inserts a new user
func (r *Repository) Create(user *User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}
