package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/service/notification/config"
	"myapp/internal/service/notification/model"
	"myapp/internal/service/notification/repository"

	"go.uber.org/zap"
)

// NotificationPoller polls the database for pending notifications
type NotificationPoller struct {
	db              *database.Database
	repo            *repository.NotificationRepository
	queue           *InMemoryQueue
	config          *config.ServiceConfig
	logger          *logger.Logger
	pollInterval    time.Duration
	batchSize       int
	backoffInterval time.Duration
	stopCh          chan struct{}
	wg              sync.WaitGroup
	mu              sync.RWMutex
	running         bool
}

// NewNotificationPoller creates a new notification poller
func NewNotificationPoller(
	db *database.Database,
	repo *repository.NotificationRepository,
	queue *InMemoryQueue,
	config *config.ServiceConfig,
	log *logger.Logger,
) *NotificationPoller {
	pollerConfig := config.Notification.Poller
	return &NotificationPoller{
		db:              db,
		repo:            repo,
		queue:           queue,
		config:          config,
		logger:          log,
		pollInterval:    time.Duration(pollerConfig.PollIntervalSec) * time.Second,
		batchSize:       pollerConfig.BatchSize,
		backoffInterval: time.Duration(pollerConfig.BackoffOnEmptySec) * time.Second,
		stopCh:          make(chan struct{}),
		running:         false,
	}
}

// Start starts the poller
func (p *NotificationPoller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("poller is already running")
	}
	p.running = true
	p.mu.Unlock()

	p.logger.Info("Starting notification poller",
		zap.Duration("poll_interval", p.pollInterval),
		zap.Int("batch_size", p.batchSize),
	)

	p.wg.Add(1)
	go p.pollLoop(ctx)

	return nil
}

// Stop stops the poller gracefully
func (p *NotificationPoller) Stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = false
	p.mu.Unlock()

	p.logger.Info("Stopping notification poller")

	close(p.stopCh)

	// Wait for poll loop to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Notification poller stopped")
		return nil
	case <-ctx.Done():
		p.logger.Warn("Notification poller stop timeout")
		return ctx.Err()
	}
}

// pollLoop is the main polling loop
func (p *NotificationPoller) pollLoop(ctx context.Context) {
	defer p.wg.Done()

	currentInterval := p.pollInterval
	emptyCount := 0

	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Poller context cancelled")
			return
		case <-p.stopCh:
			p.logger.Info("Poller stop signal received")
			return
		case <-ticker.C:
			p.performPoll(ctx, &emptyCount, &currentInterval, ticker)
		}
	}
}

// performPoll performs a single poll operation
func (p *NotificationPoller) performPoll(ctx context.Context, emptyCount *int, currentInterval *time.Duration, ticker *time.Ticker) {
	startTime := time.Now()

	// Check if queue is full
	if p.queue.IsFull() {
		p.logger.Warn("Queue is full, skipping poll",
			zap.Int("queue_length", p.queue.Length()),
			zap.Int("queue_capacity", p.queue.Capacity()),
		)
		return
	}

	// Fetch pending deliveries
	pending, err := p.repo.GetPendingDeliveries(p.batchSize)
	if err != nil {
		p.logger.Error("Failed to fetch pending deliveries", zap.Error(err))
		return
	}

	duration := time.Since(startTime)

	if len(pending) == 0 {
		*emptyCount++
		// Adaptive backoff: increase interval if no data
		if *emptyCount > 3 {
			*currentInterval = p.backoffInterval
			ticker.Reset(*currentInterval)
			p.logger.Debug("No pending deliveries, using backoff interval",
				zap.Duration("interval", *currentInterval),
			)
		}
		return
	}

	// Reset empty count and interval
	*emptyCount = 0
	if *currentInterval != p.pollInterval {
		*currentInterval = p.pollInterval
		ticker.Reset(*currentInterval)
	}

	// Extract delivery IDs
	deliveryIDs := make([]int64, 0, len(pending))
	for _, pn := range pending {
		deliveryIDs = append(deliveryIDs, pn.DeliveryID)
	}

	// Mark as processing
	if err := p.repo.MarkDeliveriesAsProcessing(deliveryIDs); err != nil {
		p.logger.Error("Failed to mark deliveries as processing", zap.Error(err))
		return
	}

	// Enqueue tasks
	enqueued := 0
	for _, pn := range pending {
		task := &model.NotificationTask{
			DeliveryID:     pn.DeliveryID,
			Delivery:       pn.Delivery,
			TargetID:       pn.TargetID,
			Target:         pn.Target,
			NotificationID: pn.NotificationID,
			Notification:   pn.Notification,
		}

		if p.queue.Enqueue(task) {
			enqueued++
		} else {
			// Queue is full, reset status back to pending
			p.logger.Warn("Queue full, resetting delivery to pending",
				zap.Int64("delivery_id", pn.DeliveryID),
			)
			// Reset this delivery back to pending
			_ = p.repo.ResetDeliveryStatus(pn.TargetID)
		}
	}

	p.logger.Info("Poll completed",
		zap.Int("fetched", len(pending)),
		zap.Int("enqueued", enqueued),
		zap.Duration("duration_ms", duration),
		zap.Int("queue_length", p.queue.Length()),
	)
}

// IsRunning returns true if poller is running
func (p *NotificationPoller) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}
