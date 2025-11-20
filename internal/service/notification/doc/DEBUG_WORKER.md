# Debug Guide: Worker không xử lý notification

## Vấn đề đã sửa ✅
- **Bug**: Field `type` trong Redis stream message không khớp với handler name
- **Fix**: Đã sửa `listener.go` để set `type: "notification"` thay vì `notif.Type`

## Checklist kiểm tra

### 1. Migration đã chạy chưa?
```bash
# Chạy migration để tạo trigger
cd internal/service/notification
go run cmd/main.go migrate
```

Kiểm tra trigger trong PostgreSQL:
```sql
-- Kiểm tra trigger function
SELECT * FROM pg_proc WHERE proname = 'notify_notification_insert';

-- Kiểm tra trigger
SELECT * FROM pg_trigger WHERE tgname = 'notification_insert_trigger';
```

### 2. Service đã start chưa?
```bash
# Start service
go run cmd/main.go serve
```

Kiểm tra logs:
- ✅ "Starting notification listener"
- ✅ "Notification listener started"
- ✅ "Starting notification worker"
- ✅ "Worker started" (cho mỗi worker goroutine)

### 3. Worker và Listener có được start không?

Kiểm tra trong `app.go` line 243-272:
- `startBackgroundServices` được gọi trong `fx.Invoke`
- Listener.Start() được gọi
- Worker.Start() được gọi trong goroutine

### 4. Redis Stream có message không?

Kiểm tra Redis:
```bash
redis-cli
> XINFO STREAM stream:notifications
> XRANGE stream:notifications - + COUNT 10
> XINFO GROUPS stream:notifications
> XPENDING stream:notifications notifications
```

### 5. Consumer Group đã được tạo chưa?

Worker tự động tạo consumer group khi khởi động. Kiểm tra:
```bash
redis-cli
> XINFO GROUPS stream:notifications
```

Nếu không có group "notifications", có thể:
- Worker chưa start
- Redis connection lỗi
- Config sai

### 6. Message format đúng chưa?

Message trong Redis stream phải có:
- `type: "notification"` (để match handler)
- `id: "<target_id>"`
- `payload: "<json>"`

Kiểm tra:
```bash
redis-cli
> XREAD STREAMS stream:notifications 0-0 COUNT 1
```

### 7. Handler có được register không?

Trong `worker.go` line 64:
```go
w.worker.Register("notification", w)
```

Handler name phải là `"notification"` và match với `task.Metadata["type"]`.

### 8. PostgreSQL NOTIFY có hoạt động không?

Test thủ công:
```sql
-- Insert notification
INSERT INTO notification (type, target_type, priority, trace_id)
VALUES ('test', 'user', 0, 'test-trace');

-- Kiểm tra xem có NOTIFY được gửi không
-- (Listener sẽ log "Received notification from PostgreSQL")
```

### 9. Kiểm tra logs chi tiết

Bật debug logging:
```yaml
# config/config.yaml
logger:
  level: "debug"  # Thay vì "info"
```

Các log quan trọng:
- `"Starting notification listener"`
- `"Received notification from PostgreSQL"`
- `"Pushed notification to Redis stream"`
- `"Starting notification worker"`
- `"Worker started"` (cho mỗi worker)
- `"Task processed successfully"` hoặc `"Task processing failed"`

### 10. Kiểm tra Redis connection

Đảm bảo Redis đang chạy và config đúng:
```yaml
redis:
  addr: "localhost:6379"
  password: ""
  db: 0
```

Test connection:
```bash
redis-cli ping
# Phải trả về: PONG
```

### 11. Kiểm tra PostgreSQL connection

Đảm bảo PostgreSQL đang chạy và config đúng:
```yaml
database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  dbname: "myapp"
```

### 12. Kiểm tra pending messages

Nếu có message pending nhưng không được xử lý:
```bash
redis-cli
> XPENDING stream:notifications notifications
```

Nếu có pending messages, có thể:
- Worker đang xử lý nhưng bị lỗi
- Message bị claim bởi consumer khác
- Consumer group bị lỗi

Reset consumer group (cẩn thận - sẽ mất pending messages):
```bash
redis-cli
> XGROUP DESTROY stream:notifications notifications
# Worker sẽ tự tạo lại khi restart
```

## Các lỗi thường gặp

### Lỗi 1: "No handler registered for task type"
- **Nguyên nhân**: Field `type` trong message không khớp với handler name
- **Fix**: Đã sửa trong `listener.go` line 135

### Lỗi 2: "Failed to fetch task"
- **Nguyên nhân**: Redis connection lỗi hoặc stream không tồn tại
- **Fix**: Kiểm tra Redis connection và config

### Lỗi 3: "Task missing type metadata"
- **Nguyên nhân**: Message trong Redis stream thiếu field `type`
- **Fix**: Đảm bảo listener push đúng format

### Lỗi 4: Worker không start
- **Nguyên nhân**: `startBackgroundServices` không được gọi hoặc lỗi khi start
- **Fix**: Kiểm tra logs và đảm bảo fx lifecycle hooks được gọi

## Test thủ công

1. **Test end-to-end**:
```bash
# 1. Start service
go run cmd/main.go serve

# 2. Trong terminal khác, tạo notification
curl -X POST http://localhost:8082/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "test",
    "target_type": "user",
    "targets": [{
      "user_id": "user123",
      "payload": {"message": "test"}
    }]
  }'

# 3. Kiểm tra logs - phải thấy:
# - "Received notification from PostgreSQL"
# - "Pushed notification to Redis stream"
# - "Task processed successfully"
```

2. **Test trực tiếp Redis**:
```bash
# Push message trực tiếp vào Redis stream
redis-cli XADD stream:notifications * \
  type notification \
  id "999" \
  payload '{"id":"999","user_id":"test","type":"test"}' \
  created_at "2024-01-01T00:00:00Z"

# Worker phải xử lý message này
```

## Debug commands

```bash
# Xem tất cả messages trong stream
redis-cli XRANGE stream:notifications - + COUNT 100

# Xem consumer groups
redis-cli XINFO GROUPS stream:notifications

# Xem pending messages
redis-cli XPENDING stream:notifications notifications

# Xem stream info
redis-cli XINFO STREAM stream:notifications

# Xem consumers trong group
redis-cli XINFO CONSUMERS stream:notifications notifications
```

