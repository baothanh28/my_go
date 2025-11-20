package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/worker"
	"myapp/internal/service/notification/model"
	"myapp/internal/service/notification/repository"

	"go.uber.org/zap"
)

// InMemoryProvider implements worker.Provider interface for in-memory queue
type InMemoryProvider struct {
	queue  *InMemoryQueue
	repo   *repository.NotificationRepository
	logger *logger.Logger
}

// NewInMemoryProvider creates a new in-memory provider
func NewInMemoryProvider(
	queue *InMemoryQueue,
	repo *repository.NotificationRepository,
	log *logger.Logger,
) *InMemoryProvider {
	return &InMemoryProvider{
		queue:  queue,
		repo:   repo,
		logger: log,
	}
}

// Fetch retrieves the next task from the queue
func (p *InMemoryProvider) Fetch(ctx context.Context) (*worker.Task, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case task, ok := <-p.queue.GetChannel():
		if !ok {
			// Channel closed
			return nil, fmt.Errorf("queue channel closed")
		}
		return p.convertToWorkerTask(task), nil
	}
}

// Ack acknowledges successful processing of a task
// For in-memory queue, this is a no-op as status is updated by worker
func (p *InMemoryProvider) Ack(ctx context.Context, task *worker.Task) error {
	// No-op for in-memory queue
	// Status update is handled by worker
	return nil
}

// Nack negatively acknowledges a task
func (p *InMemoryProvider) Nack(ctx context.Context, task *worker.Task, requeue bool) error {
	// Extract delivery ID from metadata
	deliveryIDStr := task.Metadata["delivery_id"]
	if deliveryIDStr == "" {
		return fmt.Errorf("missing delivery_id in task metadata")
	}

	var deliveryID int64
	if _, err := fmt.Sscanf(deliveryIDStr, "%d", &deliveryID); err != nil {
		return fmt.Errorf("invalid delivery_id: %w", err)
	}

	if requeue {
		// Reset status to pending for retry
		targetIDStr := task.Metadata["target_id"]
		if targetIDStr == "" {
			return fmt.Errorf("missing target_id in task metadata")
		}

		var targetID int64
		if _, err := fmt.Sscanf(targetIDStr, "%d", &targetID); err != nil {
			return fmt.Errorf("invalid target_id: %w", err)
		}

		if err := p.repo.ResetDeliveryStatus(targetID); err != nil {
			p.logger.Error("Failed to reset delivery status for retry",
				zap.Int64("delivery_id", deliveryID),
				zap.Int64("target_id", targetID),
				zap.Error(err),
			)
			return err
		}

		p.logger.Info("Delivery reset to pending for retry",
			zap.Int64("delivery_id", deliveryID),
			zap.Int64("target_id", targetID),
		)
	} else {
		// Mark as failed (already done by worker, but log it)
		p.logger.Info("Delivery marked as failed",
			zap.Int64("delivery_id", deliveryID),
		)
	}

	return nil
}

// Close cleans up provider resources
func (p *InMemoryProvider) Close() error {
	// Don't close the queue channel here as it's shared
	// Queue will be closed by poller or app shutdown
	return nil
}

// convertToWorkerTask converts NotificationTask to worker.Task
func (p *InMemoryProvider) convertToWorkerTask(nt *model.NotificationTask) *worker.Task {
	// Build payload
	payload := model.NotificationPayload{
		ID:        fmt.Sprintf("%d", nt.TargetID),
		UserID:    nt.Target.UserID,
		Type:      nt.Notification.Type,
		Data:      map[string]interface{}(nt.Target.Payload),
		Priority:  nt.Notification.Priority,
		CreatedAt: nt.Target.CreatedAt,
		TraceID:   nt.Notification.TraceID,
	}

	payloadBytes, _ := json.Marshal(payload)

	// Build metadata
	metadata := map[string]string{
		"delivery_id":       fmt.Sprintf("%d", nt.DeliveryID),
		"target_id":         fmt.Sprintf("%d", nt.TargetID),
		"notification_id":   fmt.Sprintf("%d", nt.NotificationID),
		"user_id":           nt.Target.UserID,
		"type":              "notification",
		"notification_type": nt.Notification.Type,
		"priority":          fmt.Sprintf("%d", nt.Notification.Priority),
		"trace_id":          nt.Notification.TraceID,
	}

	return &worker.Task{
		ID:        fmt.Sprintf("%d", nt.DeliveryID),
		Payload:   payloadBytes,
		Metadata:  metadata,
		Retry:     nt.Delivery.AttemptCount,
		MaxRetry:  3, // Will be set from config
		CreatedAt: nt.Delivery.CreatedAt,
	}
}
