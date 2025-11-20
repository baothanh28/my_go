package worker

import (
	"context"
	"sync"
	"time"

	"myapp/internal/pkg/logger"

	"go.uber.org/zap"
)

// MetricsCollector holds basic metrics for worker tasks
type MetricsCollector struct {
	mu            sync.RWMutex
	taskProcessed map[string]map[string]int64 // taskType -> status -> count
	taskDurations map[string][]time.Duration  // taskType -> durations
	taskRetries   map[string]int64            // taskType -> retry count
	logger        *logger.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(log *logger.Logger) *MetricsCollector {
	return &MetricsCollector{
		taskProcessed: make(map[string]map[string]int64),
		taskDurations: make(map[string][]time.Duration),
		taskRetries:   make(map[string]int64),
		logger:        log,
	}
}

// RecordTask records task processing metrics
func (mc *MetricsCollector) RecordTask(taskType, status string, duration time.Duration, retry int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Record processed count
	if mc.taskProcessed[taskType] == nil {
		mc.taskProcessed[taskType] = make(map[string]int64)
	}
	mc.taskProcessed[taskType][status]++

	// Record duration (keep last 1000 for each type)
	mc.taskDurations[taskType] = append(mc.taskDurations[taskType], duration)
	if len(mc.taskDurations[taskType]) > 1000 {
		mc.taskDurations[taskType] = mc.taskDurations[taskType][1:]
	}

	// Record retries
	if retry > 0 {
		mc.taskRetries[taskType]++
	}
}

// LogMetrics logs current metrics
func (mc *MetricsCollector) LogMetrics() {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	for taskType, statuses := range mc.taskProcessed {
		for status, count := range statuses {
			mc.logger.Info("Task metrics",
				zap.String("task_type", taskType),
				zap.String("status", status),
				zap.Int64("count", count),
			)
		}
	}
}

// MetricsMiddleware creates a middleware that collects basic metrics
// For production use, consider integrating with Prometheus or other metrics systems
func MetricsMiddleware(collector *MetricsCollector) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, task *Task) error {
			taskType := task.Metadata["type"]
			if taskType == "" {
				taskType = "unknown"
			}

			// Track processing time
			start := time.Now()
			err := next.Process(ctx, task)
			duration := time.Since(start)

			// Record metrics
			status := "success"
			if err != nil {
				status = "error"
			}

			collector.RecordTask(taskType, status, duration, task.Retry)

			return err
		})
	}
}
