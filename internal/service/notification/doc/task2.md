# Notification Service - Pull-Based Architecture với In-Memory Queue

## 📋 Tổng quan

Notification Service là một service xử lý thông báo được thiết kế với kiến trúc **pull-based**, sử dụng database polling và in-memory queue để xử lý notifications một cách hiệu quả và đáng tin cậy.

### Kiến trúc tổng quan

Service sử dụng mô hình **Database Polling → In-Memory Queue → Worker Processing** để đảm bảo:
- **Độ tin cậy cao**: Tất cả data được lưu trong PostgreSQL
- **Hiệu suất tốt**: In-memory queue giảm latency
- **Dễ mở rộng**: Có thể chạy nhiều instances với SKIP LOCKED
- **Đơn giản**: Chỉ cần PostgreSQL, không cần Redis hay message broker

## 🏗️ Kiến trúc hệ thống

```
┌─────────────────────┐
│ PostgreSQL          │
│ notification_delivery│
│ (status='pending')  │
└──────┬──────────────┘
       │ SELECT FROM notification_delivery
       │ JOIN notification_target, notification
       │ WHERE status = 'pending'
       │ ORDER BY priority DESC, created_at ASC
       │ LIMIT 1000
       │ FOR UPDATE SKIP LOCKED
       │ (Polling định kỳ: 5 giây)
       ▼
┌──────────────────┐
│ Notification     │
│ Poller           │
│ (Periodic Query) │
└──────┬───────────┘
       │ Fetch batch
       │ Mark status = 'processing'
       │ Enqueue to memory
       ▼
┌──────────────────┐
│ In-Memory Queue  │
│ (Buffered Channel)│
│ Max: 2000 items  │
└──────┬───────────┘
       │ Dequeue
       ▼
┌──────────────────┐
│ Notification     │
│ Worker Pool      │
│ (Concurrency: 10)│
└──────┬───────────┘
       │ Process notification
       │ Call sender (Expo/FCM/APNS)
       │ Update status in DB
       ▼
┌──────────────────┐
│ Sender Services  │
│ (Expo, FCM, etc.)│
└──────────────────┘
```

## 📊 Database Schema

### Bảng notification_delivery (Primary Table)

Bảng này là nguồn dữ liệu chính cho polling:

```sql
CREATE TABLE notification_delivery (
    id BIGSERIAL PRIMARY KEY,
    target_id BIGINT NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- pending, processing, delivered, failed
    attempt_count INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP,
    failed_at TIMESTAMP
);
```

**Status Flow:**
- `pending` → `processing` → `delivered` (success)
- `pending` → `processing` → `failed` (error, có thể retry)
- `processing` → `pending` (nếu timeout hoặc crash)

### Bảng notification_target

Chứa thông tin target và payload:

```sql
CREATE TABLE notification_target (
    id BIGSERIAL PRIMARY KEY,
    notification_id BIGINT NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Bảng notification

Chứa metadata của notification:

```sql
CREATE TABLE notification (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    target_type VARCHAR(20) NOT NULL DEFAULT 'user',
    priority INTEGER NOT NULL DEFAULT 0,  -- 0=normal, 1=high, 2=urgent
    trace_id VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

## 🔧 Core Components

### 1. NotificationPoller

**Chức năng**: Polling database định kỳ để lấy pending notifications.

**Struct**:
```go
type NotificationPoller struct {
    db          *database.Database
    repo        *NotificationRepository
    queue       chan *NotificationTask
    config      *ServiceConfig
    logger      *logger.Logger
    pollInterval time.Duration
    batchSize    int
    stopCh       chan struct{}
    wg           sync.WaitGroup
}
```

**Luồng hoạt động**:

1. **Polling Loop**: Chạy định kỳ (mặc định 5 giây)
2. **Query Database**: 
   ```sql
   SELECT 
       nd.id as delivery_id,
       nd.target_id,
       nd.status,
       nd.attempt_count,
       nd.retry_count,
       nd.last_error,
       nd.created_at as delivery_created_at,
       nt.id as target_id,
       nt.notification_id,
       nt.user_id,
       nt.payload as target_payload,
       nt.created_at as target_created_at,
       n.id as notification_id,
       n.type,
       n.priority,
       n.trace_id,
       n.created_at as notification_created_at
   FROM notification_delivery nd
   INNER JOIN notification_target nt ON nd.target_id = nt.id
   INNER JOIN notification n ON nt.notification_id = n.id
   WHERE nd.status = 'pending'
   ORDER BY n.priority DESC, nd.created_at ASC
   LIMIT ?
   FOR UPDATE SKIP LOCKED
   ```
3. **Mark as Processing**: Cập nhật status từ 'pending' → 'processing'
4. **Enqueue**: Đẩy vào in-memory queue
5. **Backoff**: Nếu không có records, tăng poll interval

**Tính năng**:
- Batch processing: Lấy nhiều records một lần (configurable, mặc định 1000)
- SKIP LOCKED: Cho phép nhiều instance chạy song song
- Priority ordering: Ưu tiên notifications có priority cao
- Adaptive backoff: Tăng poll interval khi không có data
- Graceful shutdown: Đợi queue rỗng trước khi shutdown

### 2. In-Memory Queue

**Chức năng**: Lưu trữ notifications trong memory để xử lý.

**Struct**:
```go
type InMemoryQueue struct {
    queue chan *NotificationTask
    mu    sync.RWMutex
    size  int
    stats QueueStats
}

type NotificationTask struct {
    DeliveryID     int64
    Delivery       *NotificationDelivery
    TargetID       int64
    Target         *NotificationTarget
    NotificationID int64
    Notification   *Notification
}

type QueueStats struct {
    Length      int64
    Enqueued    int64
    Dequeued    int64
    FullCount   int64
}
```

**Đặc điểm**:
- **Buffered Channel**: Kích thước buffer = batchSize * 2 (để tránh blocking)
- **Thread-Safe**: Sử dụng channel (đã thread-safe)
- **Non-Blocking Enqueue**: Nếu queue đầy, log warning và retry sau
- **Metrics**: Track queue length, enqueue/dequeue rate
- **Backpressure**: Nếu queue đầy, poller sẽ tạm dừng polling
- **Drain on Shutdown**: Đợi tất cả messages được xử lý

### 3. InMemoryProvider

**Chức năng**: Implement `worker.Provider` interface để cung cấp tasks cho worker.

**Struct**:
```go
type InMemoryProvider struct {
    queue  chan *NotificationTask
    logger *logger.Logger
}

func (p *InMemoryProvider) Fetch(ctx context.Context) (*worker.Task, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case task := <-p.queue:
        return convertToWorkerTask(task), nil
    }
}

func (p *InMemoryProvider) Ack(ctx context.Context, task *worker.Task) error {
    // No-op for in-memory queue
    // Status update is handled by worker
    return nil
}

func (p *InMemoryProvider) Nack(ctx context.Context, task *worker.Task, requeue bool) error {
    // Handle retry logic
    if requeue {
        // Reset status to pending for retry
        return p.resetToPending(task)
    }
    // Mark as failed
    return p.markAsFailed(task)
}

func (p *InMemoryProvider) Close() error {
    close(p.queue)
    return nil
}
```

### 4. NotificationWorker

**Chức năng**: Xử lý notifications từ queue với concurrency.

**Luồng xử lý**:

1. **Fetch Task**: Lấy task từ InMemoryProvider
2. **Check Idempotency**: Kiểm tra xem đã được xử lý chưa (database-based)
3. **Get Target**: Lấy thông tin target từ database
4. **Determine Sender**: Xác định sender type (Expo, FCM, APNS, Email)
5. **Send Notification**: Gọi sender để gửi
6. **Update Status**: 
   - Success → `status='delivered'`, set `delivered_at`
   - Failure → `status='failed'`, increment `attempt_count`, set `last_error`
7. **Retry Logic**: Nếu retryable và chưa vượt max_retries, reset về `pending`

**Idempotency**:
- Check bằng cách query `notification_delivery` với `delivered_at IS NOT NULL`
- Hoặc check `status='delivered'` cho delivery_id

**Retry Logic**:
- Dựa trên `attempt_count` và `max_retries` config
- Exponential backoff: `backoff = base_backoff * (2 ^ attempt_count)`
- Non-retryable errors → mark as failed, không retry

### 5. NotificationRepository

**Methods mới**:

```go
// GetPendingDeliveries fetches pending deliveries from notification_delivery table
// Query bắt đầu từ notification_delivery, join với notification_target và notification
func (r *NotificationRepository) GetPendingDeliveries(limit int) ([]*PendingNotification, error)

// MarkDeliveriesAsProcessing marks deliveries as processing by delivery IDs
func (r *NotificationRepository) MarkDeliveriesAsProcessing(deliveryIDs []int64) error

// GetPendingDeliveryCount returns count of pending deliveries
func (r *NotificationRepository) GetPendingDeliveryCount() (int64, error)

// CheckIdempotency checks if a delivery has already been processed (database-based)
func (r *NotificationRepository) CheckIdempotency(deliveryID int64) (bool, error)

// ResetProcessingToPending resets stale processing deliveries back to pending
func (r *NotificationRepository) ResetProcessingToPending(timeoutMinutes int) error
```

**PendingNotification struct**:
```go
type PendingNotification struct {
    // Delivery info (primary)
    DeliveryID   int64
    Delivery     *NotificationDelivery
    
    // Target info
    TargetID     int64
    Target       *NotificationTarget
    
    // Notification info
    NotificationID int64
    Notification   *Notification
}
```

## ⚙️ Configuration

### Config Structure

```yaml
notification:
  # Poller configuration
  poller:
    enabled: true
    poll_interval_sec: 5          # Thời gian giữa các lần poll (giây)
    batch_size: 1000               # Số lượng notifications lấy mỗi lần poll
    max_queue_size: 2000           # Kích thước tối đa của in-memory queue
    backoff_on_empty_sec: 30       # Tăng poll interval khi không có data
    processing_timeout_minutes: 5  # Timeout để reset processing -> pending
  
  # Worker configuration
  worker_concurrency: 10           # Số worker goroutines chạy song song
  max_retries: 3                   # Số lần retry tối đa
  retry_backoff_sec: 60            # Base backoff time (giây)
  
  # Sender configuration
  senders:
    expo:
      enabled: true
      api_url: "https://exp.host/--/api/v2/push/send"
      access_token: ""
      timeout_sec: 30
      max_retries: 3
    fcm:
      enabled: false
      project_id: ""
      credentials_file: ""
      timeout_sec: 30
      max_retries: 3
    # ... other senders
```

### Environment Variables

```bash
APP_NOTIFICATION_POLLER_ENABLED=true
APP_NOTIFICATION_POLLER_INTERVAL_SEC=5
APP_NOTIFICATION_POLLER_BATCH_SIZE=1000
APP_NOTIFICATION_POLLER_MAX_QUEUE_SIZE=2000
APP_NOTIFICATION_POLLER_BACKOFF_ON_EMPTY_SEC=30
APP_NOTIFICATION_WORKER_CONCURRENCY=10
APP_NOTIFICATION_MAX_RETRIES=3
APP_NOTIFICATION_RETRY_BACKOFF_SEC=60
```

## 🗄️ Database Optimization

### Indexes

```sql
-- Index chính cho query pending deliveries (bắt đầu từ notification_delivery)
-- Partial index cho performance tốt hơn
CREATE INDEX idx_notification_delivery_status_created 
ON notification_delivery(status, created_at) 
WHERE status = 'pending';

-- Index cho priority ordering (cần join với notification)
CREATE INDEX idx_notification_priority_created 
ON notification(priority DESC, created_at ASC);

-- Index cho foreign key join từ notification_delivery -> notification_target
CREATE INDEX idx_notification_delivery_target_id 
ON notification_delivery(target_id);

-- Index cho foreign key join từ notification_target -> notification
CREATE INDEX idx_notification_target_notification_id 
ON notification_target(notification_id);

-- Index cho idempotency check (nếu cần query delivered_at)
CREATE INDEX idx_notification_delivery_delivered_at 
ON notification_delivery(delivered_at) 
WHERE delivered_at IS NOT NULL;

-- Index cho reset processing timeout
CREATE INDEX idx_notification_delivery_status_updated 
ON notification_delivery(status, updated_at) 
WHERE status = 'processing';
```

### Query Performance

- **FOR UPDATE SKIP LOCKED**: Cho phép nhiều instance chạy song song, tránh lock contention
- **LIMIT**: Giới hạn số records mỗi lần query để tránh memory overflow
- **ORDER BY**: Ưu tiên priority và created_at để xử lý notifications quan trọng trước
- **JOIN optimization**: Sử dụng indexes hiệu quả cho các foreign key joins
- **Partial indexes**: Chỉ index các rows có status='pending' để giảm index size

## 🔄 Error Handling & Recovery

### Poller Errors

- **Database connection lost**: Retry với exponential backoff, log error
- **Query timeout**: Log warning, retry sau poll interval
- **Queue full**: Tạm dừng polling, log alert, đợi queue có chỗ trống

### Worker Errors

- **Retry Logic**: 
  - Retryable errors → increment `attempt_count`, reset về `pending` nếu chưa vượt `max_retries`
  - Non-retryable errors → mark as `failed`, không retry
- **Idempotency**: Check `delivered_at IS NOT NULL` trước khi xử lý
- **Timeout Handling**: Nếu processing quá lâu, reset về `pending` để instance khác xử lý

### Graceful Shutdown

1. **Stop Poller**: Ngừng polling mới, đợi poll hiện tại hoàn thành
2. **Drain Queue**: Đợi queue rỗng hoặc timeout (30 giây)
3. **Stop Workers**: Đợi workers hoàn thành tasks đang xử lý
4. **Reset Stale Processing**: 
   - Query tất cả deliveries có `status='processing'` và `updated_at` > threshold (5 phút)
   - Reset về `status='pending'` để các instance khác hoặc restart có thể xử lý lại

### Crash Recovery

- **Processing Timeout**: Background job định kỳ reset các deliveries có `status='processing'` quá lâu về `pending`
- **Queue Loss**: Messages trong queue sẽ mất, nhưng đã được mark `processing` trong DB, sẽ được reset về `pending` sau timeout

## 📊 Monitoring & Observability

### Metrics

**Poller Metrics**:
- `notification_poller_polls_total`: Số lần poll
- `notification_poller_records_fetched`: Số records lấy được mỗi lần poll
- `notification_poller_errors_total`: Số lỗi khi poll
- `notification_poller_duration_seconds`: Thời gian mỗi lần poll

**Queue Metrics**:
- `notification_queue_length`: Độ dài queue hiện tại
- `notification_queue_enqueued_total`: Số messages đã enqueue
- `notification_queue_dequeued_total`: Số messages đã dequeue
- `notification_queue_full_total`: Số lần queue đầy

**Worker Metrics**:
- `notification_worker_processed_total`: Query COUNT(*) WHERE status='delivered'
- `notification_worker_errors_total`: Query COUNT(*) WHERE status='failed'
- `notification_worker_duration_seconds`: Tính từ `delivered_at - created_at`
- `notification_worker_retries_total`: SUM(retry_count) WHERE status='delivered' OR status='failed'

**Database Metrics**:
- `notification_pending_count`: COUNT(*) WHERE status='pending'
- `notification_processing_count`: COUNT(*) WHERE status='processing'
- `notification_delivered_count`: COUNT(*) WHERE status='delivered'
- `notification_failed_count`: COUNT(*) WHERE status='failed'

**Lưu ý**: Tất cả metrics đều query từ database. Có thể tạo bảng `notification_metrics` riêng hoặc materialized view nếu cần real-time metrics.

### Logging

**Structured Logs**:

```json
{
  "event": "POLL_START",
  "batch_size": 1000,
  "timestamp": "2024-01-01T00:00:00Z"
}

{
  "event": "POLL_SUCCESS",
  "records_fetched": 150,
  "duration_ms": 45,
  "timestamp": "2024-01-01T00:00:00Z"
}

{
  "event": "QUEUE_ENQUEUE",
  "delivery_id": 123,
  "target_id": 456,
  "queue_length": 150,
  "timestamp": "2024-01-01T00:00:00Z"
}

{
  "event": "NOTIFICATION_SEND_SUCCESS",
  "delivery_id": 123,
  "target_id": 456,
  "user_id": "user-123",
  "sender": "expo",
  "duration_ms": 120,
  "timestamp": "2024-01-01T00:00:00Z"
}

{
  "event": "NOTIFICATION_SEND_FAILED",
  "delivery_id": 123,
  "target_id": 456,
  "user_id": "user-123",
  "sender": "expo",
  "error": "connection timeout",
  "retryable": true,
  "attempt_count": 2,
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### Health Checks

**GET /health**:
- Check database connection
- Check queue length (alert nếu > 80% capacity)
- Check pending deliveries count

**GET /ready**:
- Check poller đang chạy
- Check workers đang chạy
- Check queue không đầy

## 🧪 Testing Strategy

### Unit Tests

**NotificationPoller**:
- Test polling logic với mock database
- Test batch fetching
- Test queue enqueue
- Test error handling
- Test graceful shutdown
- Test adaptive backoff

**InMemoryQueue**:
- Test enqueue/dequeue
- Test backpressure khi queue đầy
- Test thread-safety với concurrent access
- Test drain on shutdown

**InMemoryProvider**:
- Test Fetch() method
- Test context cancellation
- Test task conversion
- Test Ack/Nack behavior

**NotificationWorker**:
- Test notification processing
- Test idempotency check
- Test retry logic
- Test error handling
- Test sender integration

### Integration Tests

**Database Integration**:
- Test query với SKIP LOCKED
- Test concurrent polling từ nhiều instances
- Test status updates
- Test transaction handling

**End-to-End**:
- Test full flow: Poll → Queue → Worker → Send
- Test với nhiều notifications (1000+)
- Test error scenarios (sender failure, timeout)
- Test graceful shutdown
- Test crash recovery

### Load Tests

**Poller Performance**:
- Test với 10k+ pending notifications
- Test poll interval tuning
- Test batch size optimization
- Test với nhiều instances chạy song song

**Queue Performance**:
- Test với high throughput (1000+ notifications/second)
- Test queue size limits
- Test backpressure handling
- Test memory usage

**Worker Performance**:
- Test concurrency tuning
- Test với nhiều sender types
- Test retry overhead
- Test database connection pooling

## 🚀 Deployment

### Requirements

- **Go**: 1.25+
- **PostgreSQL**: 15+ (với SKIP LOCKED support)
- **No Redis**: Không cần Redis
- **No Message Broker**: Không cần RabbitMQ, Kafka, etc.

### Migration Steps

1. **Run Database Migrations**:
   ```bash
   go run ./internal/service/notification/cmd/main.go migrate
   ```
   - Tạo các bảng: notification, notification_target, notification_delivery
   - Tạo indexes
   - **Không tạo trigger** (pull-based model không cần)

2. **Configure Service**:
   - Update `config/config.yaml` với các thông số phù hợp
   - Set environment variables nếu cần

3. **Start Service**:
   ```bash
   go run ./internal/service/notification/cmd/main.go serve
   ```

4. **Monitor**:
   - Check logs để đảm bảo poller đang chạy
   - Check metrics để monitor performance
   - Check database để verify notifications được xử lý

### Scaling

**Horizontal Scaling**:
- Có thể chạy nhiều instances của service
- Mỗi instance sẽ poll riêng với SKIP LOCKED
- Tự động load balancing giữa các instances

**Vertical Scaling**:
- Tăng `worker_concurrency` để xử lý nhiều notifications cùng lúc
- Tăng `batch_size` để lấy nhiều notifications mỗi lần poll
- Tăng `max_queue_size` để buffer nhiều hơn

**Database Scaling**:
- Sử dụng read replicas cho polling queries
- Partition `notification_delivery` theo date nếu data lớn
- Tạo materialized views cho metrics

## 📈 Performance Tuning

### Poll Interval

- **Default**: 5 giây
- **High Load**: Giảm xuống 1-2 giây
- **Low Load**: Tăng lên 10-30 giây (với backoff)

### Batch Size

- **Default**: 1000
- **High Memory**: Giảm xuống 500
- **High Throughput**: Tăng lên 2000-5000

### Worker Concurrency

- **Default**: 10
- **CPU-bound**: Tăng lên số CPU cores
- **IO-bound**: Tăng lên 20-50

### Queue Size

- **Default**: 2000 (batch_size * 2)
- **High Throughput**: Tăng lên 5000-10000
- **Memory Constrained**: Giảm xuống 1000

## 🔐 Security Considerations

- **Database Connection**: Sử dụng connection pooling, credentials từ environment variables
- **API Authentication**: JWT tokens cho API endpoints
- **Sender Credentials**: Lưu trong config, không hardcode
- **SQL Injection**: Sử dụng parameterized queries
- **Rate Limiting**: Implement rate limiting cho senders (Expo, FCM, etc.)

## 📝 Best Practices

1. **Monitor Queue Length**: Alert nếu queue > 80% capacity
2. **Monitor Pending Count**: Alert nếu pending notifications tích tụ
3. **Monitor Processing Timeout**: Reset stale processing deliveries định kỳ
4. **Log Everything**: Structured logging cho tất cả events
5. **Graceful Shutdown**: Luôn đợi queue drain trước khi shutdown
6. **Idempotency**: Luôn check idempotency trước khi xử lý
7. **Error Handling**: Phân biệt retryable và non-retryable errors
8. **Database Indexes**: Đảm bảo tất cả queries đều sử dụng indexes

## 🎯 Advantages

- ✅ **Đơn giản**: Chỉ cần PostgreSQL, không cần Redis hay message broker
- ✅ **Đáng tin cậy**: Tất cả data trong database, không mất dữ liệu
- ✅ **Hiệu suất tốt**: In-memory queue giảm latency
- ✅ **Dễ debug**: Tất cả data trong database, dễ query và debug
- ✅ **Single source of truth**: Tất cả data (deliveries, metrics, idempotency) đều trong database
- ✅ **Flexible**: Dễ điều chỉnh poll interval và batch size
- ✅ **Scalable**: Có thể chạy nhiều instances với SKIP LOCKED
- ✅ **Cost effective**: Không cần Redis server

## ⚠️ Limitations & Mitigations

- ⚠️ **Memory usage**: Queue trong memory → Giới hạn queue size, monitor memory
- ⚠️ **Data loss risk**: Messages trong queue mất nếu crash → Mark processing trước khi enqueue, reset sau timeout
- ⚠️ **Polling overhead**: Query database định kỳ → Optimize query với indexes, tune poll interval
- ⚠️ **Database load**: Tăng load lên database → Monitor query performance, sử dụng connection pooling, tune batch size
- ⚠️ **Metrics performance**: Query metrics từ database có thể chậm → Cache metrics trong memory, hoặc tạo materialized view

## 🔮 Future Enhancements

- **Adaptive Polling**: Tự động điều chỉnh poll interval dựa trên load và queue length
- **Priority Queue**: Implement priority queue trong memory dựa trên notification.priority
- **Batch Processing**: Xử lý batch notifications cùng lúc để tăng throughput
- **Database Partitioning**: Partition `notification_delivery` theo status hoặc date nếu data lớn
- **Materialized Views**: Tạo materialized views cho metrics để query nhanh hơn
- **Read Replicas**: Sử dụng read replicas cho polling queries để giảm load lên primary database
- **Distributed Queue**: Nếu cần scale hơn nữa, có thể migrate sang Redis Queue hoặc RabbitMQ (nhưng hiện tại không cần)

---

**Tác giả**: Development Team  
**Ngày tạo**: 2024-01-01  
**Version**: 1.0  
**Status**: Design Document
