package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"myapp/internal/pkg/database"
	"myapp/internal/service/notification/model"

	"gorm.io/gorm"
)

// NotificationRepository handles database operations for notifications
type NotificationRepository struct {
	db *database.Database
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *database.Database) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// CreateNotification creates a new notification with targets
func (r *NotificationRepository) CreateNotification(notif *model.Notification, targets []*model.NotificationTarget) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Create notification
		if err := tx.Create(notif).Error; err != nil {
			return fmt.Errorf("failed to create notification: %w", err)
		}

		// Create targets
		for _, target := range targets {
			target.NotificationID = notif.ID
			if err := tx.Create(target).Error; err != nil {
				return fmt.Errorf("failed to create notification target: %w", err)
			}

			// Create delivery record
			delivery := &model.NotificationDelivery{
				TargetID:     target.ID,
				Status:       "pending",
				AttemptCount: 0,
				RetryCount:   0,
			}
			if err := tx.Create(delivery).Error; err != nil {
				return fmt.Errorf("failed to create delivery record: %w", err)
			}
		}

		return nil
	})
}

// GetNotificationByID retrieves a notification by ID
func (r *NotificationRepository) GetNotificationByID(id int64) (*model.Notification, error) {
	var notif model.Notification
	if err := r.db.First(&notif, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	return &notif, nil
}

// GetTargetsByNotificationID retrieves all targets for a notification
func (r *NotificationRepository) GetTargetsByNotificationID(notificationID int64) ([]*model.NotificationTarget, error) {
	var targets []*model.NotificationTarget
	if err := r.db.Where("notification_id = ?", notificationID).Find(&targets).Error; err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}
	return targets, nil
}

// GetDeliveryByTargetID retrieves delivery record by target ID
func (r *NotificationRepository) GetDeliveryByTargetID(targetID int64) (*model.NotificationDelivery, error) {
	var delivery model.NotificationDelivery
	if err := r.db.Where("target_id = ?", targetID).First(&delivery).Error; err != nil {
		return nil, fmt.Errorf("failed to get delivery: %w", err)
	}
	return &delivery, nil
}

// MarkDelivered marks a delivery as delivered
func (r *NotificationRepository) MarkDelivered(targetID int64) error {
	now := time.Now()
	return r.db.Model(&model.NotificationDelivery{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"status":       "delivered",
			"delivered_at": now,
			"updated_at":   now,
		}).Error
}

// IncrementAttempt increments the attempt count for a delivery
func (r *NotificationRepository) IncrementAttempt(targetID int64, errorMsg string) error {
	updates := map[string]interface{}{
		"attempt_count": gorm.Expr("attempt_count + 1"),
		"updated_at":    time.Now(),
	}
	if errorMsg != "" {
		updates["last_error"] = errorMsg
		updates["failed_at"] = time.Now()
		updates["status"] = "failed"
	} else {
		updates["status"] = "processing"
	}

	return r.db.Model(&model.NotificationDelivery{}).
		Where("target_id = ?", targetID).
		Updates(updates).Error
}

// IncrementRetry increments the retry count for a delivery
func (r *NotificationRepository) IncrementRetry(targetID int64) error {
	return r.db.Model(&model.NotificationDelivery{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"retry_count": gorm.Expr("retry_count + 1"),
			"status":      "pending",
			"updated_at":  time.Now(),
		}).Error
}

// GetPendingFailedForUser retrieves failed notifications for a user
func (r *NotificationRepository) GetPendingFailedForUser(userID string, limit, offset int) ([]*model.FailedNotificationResponse, error) {
	var results []*model.FailedNotificationResponse

	query := `
		SELECT 
			nd.id,
			nt.notification_id,
			nt.user_id,
			n.type,
			nt.payload,
			nd.status,
			nd.attempt_count,
			nd.retry_count,
			nd.last_error,
			nd.created_at,
			nd.failed_at
		FROM notification_delivery nd
		INNER JOIN notification_target nt ON nd.target_id = nt.id
		INNER JOIN notification n ON nt.notification_id = n.id
		WHERE nt.user_id = ? AND nd.status = 'failed'
		ORDER BY nd.failed_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Raw(query, userID, limit, offset).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to query failed notifications: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var result model.FailedNotificationResponse
		var payload model.JSONB
		if err := rows.Scan(
			&result.ID,
			&result.NotificationID,
			&result.UserID,
			&result.Type,
			&payload,
			&result.Status,
			&result.AttemptCount,
			&result.RetryCount,
			&result.LastError,
			&result.CreatedAt,
			&result.FailedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result.Payload = payload
		results = append(results, &result)
	}

	return results, nil
}

// GetTargetByID retrieves a target by ID
func (r *NotificationRepository) GetTargetByID(id int64) (*model.NotificationTarget, error) {
	var target model.NotificationTarget
	if err := r.db.First(&target, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}
	return &target, nil
}

// ResetDeliveryStatus resets delivery status to pending for retry
func (r *NotificationRepository) ResetDeliveryStatus(targetID int64) error {
	return r.db.Model(&model.NotificationDelivery{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"status":     "pending",
			"updated_at": time.Now(),
			"failed_at":  nil,
		}).Error
}

// GetPendingDeliveries fetches pending deliveries from notification_delivery table
// Query bắt đầu từ notification_delivery, join với notification_target và notification
func (r *NotificationRepository) GetPendingDeliveries(limit int) ([]*model.PendingNotification, error) {
	var results []*model.PendingNotification

	query := `
		SELECT 
			nd.id as delivery_id,
			nd.target_id,
			nd.status,
			nd.attempt_count,
			nd.retry_count,
			nd.last_error,
			nd.created_at as delivery_created_at,
			nd.updated_at as delivery_updated_at,
			nd.delivered_at,
			nd.failed_at,
			nt.id as target_id,
			nt.notification_id,
			nt.user_id,
			nt.payload as target_payload,
			nt.created_at as target_created_at,
			nt.updated_at as target_updated_at,
			n.id as notification_id,
			n.type,
			n.target_type,
			n.priority,
			n.trace_id,
			n.created_at as notification_created_at,
			n.updated_at as notification_updated_at
		FROM notification_delivery nd
		INNER JOIN notification_target nt ON nd.target_id = nt.id
		INNER JOIN notification n ON nt.notification_id = n.id
		WHERE nd.status = 'pending'
		ORDER BY n.priority DESC, nd.created_at ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.Raw(query, limit).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to query pending deliveries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pn model.PendingNotification
		var delivery model.NotificationDelivery
		var target model.NotificationTarget
		var notif model.Notification
		var targetPayload model.JSONB
		var lastError sql.NullString
		var traceID sql.NullString

		err := rows.Scan(
			&delivery.ID,
			&delivery.TargetID,
			&delivery.Status,
			&delivery.AttemptCount,
			&delivery.RetryCount,
			&lastError, // Use sql.NullString for nullable field
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
			&delivery.DeliveredAt,
			&delivery.FailedAt,
			&target.ID,
			&target.NotificationID,
			&target.UserID,
			&targetPayload,
			&target.CreatedAt,
			&target.UpdatedAt,
			&notif.ID,
			&notif.Type,
			&notif.TargetType,
			&notif.Priority,
			&traceID, // Use sql.NullString for nullable field
			&notif.CreatedAt,
			&notif.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert sql.NullString to string
		if lastError.Valid {
			delivery.LastError = lastError.String
		} else {
			delivery.LastError = ""
		}

		if traceID.Valid {
			notif.TraceID = traceID.String
		} else {
			notif.TraceID = ""
		}

		target.Payload = targetPayload
		pn.DeliveryID = delivery.ID
		pn.Delivery = &delivery
		pn.TargetID = target.ID
		pn.Target = &target
		pn.NotificationID = notif.ID
		pn.Notification = &notif

		results = append(results, &pn)
	}

	return results, nil
}

// MarkDeliveriesAsProcessing marks deliveries as processing by delivery IDs
func (r *NotificationRepository) MarkDeliveriesAsProcessing(deliveryIDs []int64) error {
	if len(deliveryIDs) == 0 {
		return nil
	}

	now := time.Now()
	return r.db.Model(&model.NotificationDelivery{}).
		Where("id IN ?", deliveryIDs).
		Updates(map[string]interface{}{
			"status":     "processing",
			"updated_at": now,
		}).Error
}

// GetPendingDeliveryCount returns count of pending deliveries
func (r *NotificationRepository) GetPendingDeliveryCount() (int64, error) {
	var count int64
	err := r.db.Model(&model.NotificationDelivery{}).
		Where("status = ?", "pending").
		Count(&count).Error
	return count, err
}

// CheckIdempotency checks if a delivery has already been processed (database-based)
func (r *NotificationRepository) CheckIdempotency(deliveryID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.NotificationDelivery{}).
		Where("id = ? AND status = ? AND delivered_at IS NOT NULL", deliveryID, "delivered").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ResetProcessingToPending resets stale processing deliveries back to pending
func (r *NotificationRepository) ResetProcessingToPending(timeoutMinutes int) error {
	timeout := time.Now().Add(-time.Duration(timeoutMinutes) * time.Minute)
	now := time.Now()

	return r.db.Model(&model.NotificationDelivery{}).
		Where("status = ? AND updated_at < ?", "processing", timeout).
		Updates(map[string]interface{}{
			"status":     "pending",
			"updated_at": now,
		}).Error
}

// RegisterDeviceToken registers or updates a device token
func (r *NotificationRepository) RegisterDeviceToken(dto model.RegisterTokenDTO) (*model.DeviceToken, error) {
	var token model.DeviceToken

	// Tìm token theo user_id và device_id
	err := r.db.Where("user_id = ? AND device_id = ?", dto.UserID, dto.DeviceID).First(&token).Error

	now := time.Now()

	if err == nil {
		// Update existing token
		token.PushToken = dto.PushToken
		token.Type = dto.Type
		token.Platform = dto.Platform
		token.LastSeenAt = now
		token.UpdatedAt = now

		if err := r.db.Save(&token).Error; err != nil {
			return nil, fmt.Errorf("failed to update device token: %w", err)
		}
		return &token, nil
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new token
		token = model.DeviceToken{
			UserID:     dto.UserID,
			DeviceID:   dto.DeviceID,
			PushToken:  dto.PushToken,
			Type:       dto.Type,
			Platform:   dto.Platform,
			LastSeenAt: now,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		if err := r.db.Create(&token).Error; err != nil {
			return nil, fmt.Errorf("failed to create device token: %w", err)
		}
		return &token, nil
	}

	return nil, fmt.Errorf("failed to query device token: %w", err)
}

// GetDeviceTokensByUserID retrieves all device tokens for a user
func (r *NotificationRepository) GetDeviceTokensByUserID(userID string) ([]*model.DeviceToken, error) {
	var tokens []*model.DeviceToken
	if err := r.db.Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("failed to get device tokens: %w", err)
	}
	return tokens, nil
}

// DeleteDeviceToken deletes a device token
func (r *NotificationRepository) DeleteDeviceToken(userID, deviceID string) error {
	result := r.db.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&model.DeviceToken{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete device token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
