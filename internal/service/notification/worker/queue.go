package worker

import (
	"sync"
	"sync/atomic"

	"myapp/internal/service/notification/model"
)

// InMemoryQueue represents an in-memory queue for notifications
type InMemoryQueue struct {
	queue chan *model.NotificationTask
	mu    sync.RWMutex
	size  int
	stats QueueStats
}

// QueueStats holds queue statistics
type QueueStats struct {
	Length    int64
	Enqueued  int64
	Dequeued  int64
	FullCount int64
}

// NewInMemoryQueue creates a new in-memory queue
func NewInMemoryQueue(size int) *InMemoryQueue {
	return &InMemoryQueue{
		queue: make(chan *model.NotificationTask, size),
		size:  size,
		stats: QueueStats{},
	}
}

// Enqueue adds a task to the queue
// Returns true if successfully enqueued, false if queue is full
func (q *InMemoryQueue) Enqueue(task *model.NotificationTask) bool {
	select {
	case q.queue <- task:
		atomic.AddInt64(&q.stats.Enqueued, 1)
		atomic.AddInt64(&q.stats.Length, 1)
		return true
	default:
		atomic.AddInt64(&q.stats.FullCount, 1)
		return false
	}
}

// Dequeue removes and returns a task from the queue
// Returns nil if queue is empty
func (q *InMemoryQueue) Dequeue() *model.NotificationTask {
	select {
	case task := <-q.queue:
		atomic.AddInt64(&q.stats.Dequeued, 1)
		atomic.AddInt64(&q.stats.Length, -1)
		return task
	default:
		return nil
	}
}

// GetChannel returns the underlying channel for direct access
func (q *InMemoryQueue) GetChannel() <-chan *model.NotificationTask {
	return q.queue
}

// GetChannelForWrite returns the underlying channel for writing
func (q *InMemoryQueue) GetChannelForWrite() chan<- *model.NotificationTask {
	return q.queue
}

// Length returns the current queue length
func (q *InMemoryQueue) Length() int {
	return len(q.queue)
}

// Capacity returns the queue capacity
func (q *InMemoryQueue) Capacity() int {
	return q.size
}

// IsFull returns true if queue is full
func (q *InMemoryQueue) IsFull() bool {
	return len(q.queue) >= q.size
}

// Stats returns queue statistics
func (q *InMemoryQueue) Stats() QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return QueueStats{
		Length:    atomic.LoadInt64(&q.stats.Length),
		Enqueued:  atomic.LoadInt64(&q.stats.Enqueued),
		Dequeued:  atomic.LoadInt64(&q.stats.Dequeued),
		FullCount: atomic.LoadInt64(&q.stats.FullCount),
	}
}

// Close closes the queue channel
func (q *InMemoryQueue) Close() {
	close(q.queue)
}

