package service

import (
	"errors"
	"fmt"

	"myapp/internal/pkg/logger"
	"myapp/internal/service/notification/config"
	"myapp/internal/service/notification/model"
	"myapp/internal/service/notification/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// NotificationService handles notification business logic
type NotificationService struct {
	repo   *repository.NotificationRepository
	config *config.ServiceConfig
	logger *logger.Logger
}

// NewNotificationService creates a new notification service
func NewNotificationService(repo *repository.NotificationRepository, cfg *config.ServiceConfig, log *logger.Logger) *NotificationService {
	return &NotificationService{
		repo:   repo,
		config: cfg,
		logger: log,
	}
}

// CreateNotification creates a new notification with targets
func (s *NotificationService) CreateNotification(dto model.CreateNotificationDTO) (*model.Notification, error) {
	// Create notification
	notif := &model.Notification{
		Type:       dto.Type,
		TargetType: dto.TargetType,
		Priority:   dto.Priority,
		TraceID:    dto.TraceID,
	}

	// Create targets
	targets := make([]*model.NotificationTarget, 0, len(dto.Targets))
	for _, targetDTO := range dto.Targets {
		target := &model.NotificationTarget{
			UserID:  targetDTO.UserID,
			Payload: model.JSONB(targetDTO.Payload),
		}
		targets = append(targets, target)
	}

	// Save to database
	if err := s.repo.CreateNotification(notif, targets); err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	s.logger.Info("Notification created",
		zap.Int64("notification_id", notif.ID),
		zap.String("type", notif.Type),
		zap.Int("target_count", len(targets)),
		zap.String("trace_id", notif.TraceID),
	)

	return notif, nil
}

// GetFailedNotifications retrieves failed notifications for a user
func (s *NotificationService) GetFailedNotifications(userID string, limit, offset int) ([]*model.FailedNotificationResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	notifications, err := s.repo.GetPendingFailedForUser(userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed notifications: %w", err)
	}

	return notifications, nil
}

// RetryNotification retries a failed notification
func (s *NotificationService) RetryNotification(targetID int64) error {
	// Get target
	target, err := s.repo.GetTargetByID(targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("notification target not found")
		}
		return fmt.Errorf("failed to get target: %w", err)
	}

	// Get delivery record
	delivery, err := s.repo.GetDeliveryByTargetID(targetID)
	if err != nil {
		return fmt.Errorf("failed to get delivery record: %w", err)
	}

	if delivery.Status != "failed" {
		return errors.New("notification is not in failed status")
	}

	// Reset delivery status
	if err := s.repo.ResetDeliveryStatus(targetID); err != nil {
		return fmt.Errorf("failed to reset delivery status: %w", err)
	}

	s.logger.Info("Notification retry requested",
		zap.Int64("target_id", targetID),
		zap.String("user_id", target.UserID),
		zap.Int("previous_attempts", delivery.AttemptCount),
	)

	return nil
}

// GetNotificationByID retrieves a notification by ID
func (s *NotificationService) GetNotificationByID(id int64) (*model.Notification, error) {
	notif, err := s.repo.GetNotificationByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("notification not found")
		}
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	return notif, nil
}

// RegisterDeviceToken registers or updates a device token
func (s *NotificationService) RegisterDeviceToken(dto model.RegisterTokenDTO) (*model.RegisterTokenResponse, error) {
	token, err := s.repo.RegisterDeviceToken(dto)
	if err != nil {
		return nil, fmt.Errorf("failed to register device token: %w", err)
	}

	s.logger.Info("Device token registered",
		zap.Int64("token_id", token.ID),
		zap.String("user_id", token.UserID),
		zap.String("device_id", token.DeviceID),
		zap.String("type", token.Type),
		zap.String("platform", token.Platform),
	)

	return &model.RegisterTokenResponse{
		ID:         token.ID,
		UserID:     token.UserID,
		DeviceID:   token.DeviceID,
		PushToken:  token.PushToken,
		Type:       token.Type,
		Platform:   token.Platform,
		LastSeenAt: token.LastSeenAt,
		CreatedAt:  token.CreatedAt,
		UpdatedAt:  token.UpdatedAt,
	}, nil
}

