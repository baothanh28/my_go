package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// Notification represents the notification metadata table
type Notification struct {
	ID         int64          `gorm:"primarykey" json:"id"`
	Type       string         `gorm:"type:varchar(100);not null" json:"type"`
	TargetType string         `gorm:"type:varchar(20);not null;default:'user'" json:"target_type"`
	Priority   int            `gorm:"not null;default:0" json:"priority"`
	TraceID    string         `gorm:"type:varchar(255)" json:"trace_id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name
func (Notification) TableName() string {
	return "notification"
}

// NotificationTarget represents the notification_target table
type NotificationTarget struct {
	ID             int64          `gorm:"primarykey" json:"id"`
	NotificationID int64          `gorm:"not null;index" json:"notification_id"`
	UserID         string         `gorm:"type:varchar(255);not null;index" json:"user_id"`
	Payload        JSONB          `gorm:"type:jsonb;not null;default:'{}'" json:"payload"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name
func (NotificationTarget) TableName() string {
	return "notification_target"
}

// NotificationDelivery represents the notification_delivery table
type NotificationDelivery struct {
	ID           int64      `gorm:"primarykey" json:"id"`
	TargetID     int64      `gorm:"not null;uniqueIndex" json:"target_id"`
	Status       string     `gorm:"type:varchar(50);not null;default:'pending';index" json:"status"`
	AttemptCount int        `gorm:"not null;default:0" json:"attempt_count"`
	RetryCount   int        `gorm:"not null;default:0" json:"retry_count"`
	LastError    string     `gorm:"type:text" json:"last_error"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeliveredAt  *time.Time `json:"delivered_at"`
	FailedAt     *time.Time `json:"failed_at"`
}

// TableName specifies the table name
func (NotificationDelivery) TableName() string {
	return "notification_delivery"
}

// JSONB is a custom type for JSONB fields
type JSONB map[string]interface{}

// Value implements driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// NotificationPayload represents the payload structure
type NotificationPayload struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Priority  int                    `json:"priority"`
	CreatedAt time.Time              `json:"created_at"`
	TraceID   string                 `json:"trace_id"`
}

// CreateNotificationDTO is the DTO for creating a notification
type CreateNotificationDTO struct {
	Type       string                  `json:"type" validate:"required"`
	TargetType string                  `json:"target_type" validate:"required,oneof=user group alluser"`
	Priority   int                     `json:"priority" validate:"gte=0,lte=2"`
	TraceID    string                  `json:"trace_id"`
	Targets    []NotificationTargetDTO `json:"targets" validate:"required,min=1"`
}

// NotificationTargetDTO represents a target for notification
type NotificationTargetDTO struct {
	UserID  string                 `json:"user_id" validate:"required"`
	Payload map[string]interface{} `json:"payload" validate:"required"`
}

// NotificationResponse represents a notification in API responses
type NotificationResponse struct {
	ID          int64                  `json:"id"`
	Type        string                 `json:"type"`
	TargetType  string                 `json:"target_type"`
	Priority    int                    `json:"priority"`
	TraceID     string                 `json:"trace_id"`
	UserID      string                 `json:"user_id"`
	Payload     map[string]interface{} `json:"payload"`
	Status      string                 `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	DeliveredAt *time.Time             `json:"delivered_at"`
}

// FailedNotificationResponse represents a failed notification
type FailedNotificationResponse struct {
	ID             int64      `json:"id"`
	NotificationID int64      `json:"notification_id"`
	UserID         string     `json:"user_id"`
	Type           string     `json:"type"`
	Payload        JSONB      `json:"payload"`
	Status         string     `json:"status"`
	AttemptCount   int        `json:"attempt_count"`
	RetryCount     int        `json:"retry_count"`
	LastError      string     `json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
	FailedAt       *time.Time `json:"failed_at"`
}

// PendingNotification represents a pending notification with all related data
type PendingNotification struct {
	// Delivery info (primary)
	DeliveryID int64
	Delivery   *NotificationDelivery

	// Target info
	TargetID int64
	Target   *NotificationTarget

	// Notification info
	NotificationID int64
	Notification   *Notification
}

// NotificationTask represents a task in the in-memory queue
type NotificationTask struct {
	DeliveryID     int64
	Delivery       *NotificationDelivery
	TargetID       int64
	Target         *NotificationTarget
	NotificationID int64
	Notification   *Notification
}

// DeviceToken represents a device push token
type DeviceToken struct {
	ID         int64          `gorm:"primarykey" json:"id"`
	UserID     string         `gorm:"type:varchar(255);not null;index" json:"user_id"`
	DeviceID   string         `gorm:"type:varchar(255);not null;index" json:"device_id"`
	PushToken  string         `gorm:"type:varchar(500);not null" json:"push_token"` // Lưu token (expo, fcm, apns, native)
	Type       string         `gorm:"type:varchar(50);not null;index" json:"type"`  // expo, fcm, apns, native
	Platform   string         `gorm:"type:varchar(50);not null" json:"platform"`    // ios, android, web
	LastSeenAt time.Time      `gorm:"not null" json:"last_seen_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name
func (DeviceToken) TableName() string {
	return "device_tokens"
}

// RegisterTokenDTO is the DTO for registering a device token
type RegisterTokenDTO struct {
	UserID    string `json:"user_id" validate:"required"`
	DeviceID  string `json:"device_id" validate:"required"`
	PushToken string `json:"push_token" validate:"required"`
	Type      string `json:"type" validate:"required,oneof=expo fcm apns native"` // Loại token
	Platform  string `json:"platform" validate:"required,oneof=ios android web"`  // Platform của device
}

// RegisterTokenResponse represents the response after token registration
type RegisterTokenResponse struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	DeviceID   string    `json:"device_id"`
	PushToken  string    `json:"push_token"`
	Type       string    `json:"type"`
	Platform   string    `json:"platform"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
