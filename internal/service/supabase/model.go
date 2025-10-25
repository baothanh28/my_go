package supabase

import (
	"time"

	"gorm.io/gorm"
)

// User represents the user entity (shared shape with auth service)
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	Name      string         `gorm:"not null" json:"name"`
	Role      string         `gorm:"not null;default:'user'" json:"role"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for User model
func (User) TableName() string { return "users" }

// SupabaseLoginDTO contains the token from Supabase client
type SupabaseLoginDTO struct {
	SupabaseAccessToken string `json:"supabase_access_token"`
}

// TokenResponse mirrors auth service response
type TokenResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
}

// UserResponse is the user data in responses
type UserResponse struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// ToUserResponse converts User to UserResponse
func (u *User) ToUserResponse() UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role}
}

// UserContext holds user information in Echo context
type UserContext struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Name   string `json:"name"`
}
