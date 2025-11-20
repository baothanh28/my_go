# Notification Service

Service xử lý thông báo với in-memory queue, database poller, và hỗ trợ nhiều loại channel (Expo, FCM, APNS, Email).

## 📁 Cấu trúc Project

```
internal/service/notification/
├── app.go                    # FX module setup - Dependency injection
├── cmd/                      # CLI commands
│   ├── main.go              # Main entry point
│   ├── serve.go             # Serve command
│   ├── migrate.go           # Migration command
│   ├── api.go               # API-only mode
│   ├── lifecycle.go         # Lifecycle management
│   └── version.go           # Version command
│
├── config/                   # Configuration
│   ├── config.go            # Service configuration types
│   └── config.yaml          # Configuration file
│
├── model/                    # Domain models & DTOs
│   └── model.go             # Notification models, DTOs, responses
│
├── handler/                  # HTTP handlers
│   ├── handler.go           # HTTP request handlers
│   └── router.go            # Route registration
│
├── service/                  # Business logic layer
│   └── service.go           # Notification business logic
│
├── repository/               # Data access layer
│   ├── repository.go        # Database operations
│   └── migration.go         # Migration runner
│
├── worker/                   # Background workers
│   ├── worker.go            # Notification worker
│   ├── poller.go            # Database poller
│   ├── provider.go          # Worker provider (in-memory queue)
│   └── queue.go             # In-memory queue implementation
│
├── channel/                  # Notification channels
│   └── channel.go           # Expo, FCM, APNS, Email channels
│
├── migration/                # SQL migration files
│   ├── 000001_create_notification_table.up.sql
│   ├── 000001_create_notification_table.down.sql
│   ├── 000002_create_notification_target_table.up.sql
│   ├── 000002_create_notification_target_table.down.sql
│   ├── 000003_create_notification_delivery_table.up.sql
│   ├── 000003_create_notification_delivery_table.down.sql
│   ├── 000004_create_device_tokens_table.up.sql
│   └── 000004_create_device_tokens_table.down.sql
│
└── doc/                      # Documentation
    ├── README.md            # This file
    ├── task.md              # Task requirements
    └── ...
```

## 🏗️ Kiến trúc

### Layers

1. **Handler Layer** (`handler/`)
   - Xử lý HTTP requests/responses
   - Validation input
   - Gọi service layer

2. **Service Layer** (`service/`)
   - Business logic
   - Orchestration
   - Gọi repository layer

3. **Repository Layer** (`repository/`)
   - Database operations
   - Data access abstraction
   - Query optimization

4. **Worker Layer** (`worker/`)
   - Background processing
   - Queue management
   - Polling database

5. **Channel Layer** (`channel/`)
   - Notification delivery
   - Multiple channel support (Expo, FCM, APNS, Email)
   - Channel registry

### Components

- **Poller**: Polls database for pending notifications
- **Queue**: In-memory queue for notification tasks
- **Worker**: Processes notifications from queue
- **Channels**: Send notifications via different channels (Expo, FCM, etc.)

## 🚀 Hướng dẫn chạy

### Yêu cầu

- Go 1.25+
- PostgreSQL 15+
- Docker (tùy chọn, để chạy PostgreSQL)

### 1. Khởi động PostgreSQL

```bash
# Sử dụng Docker
docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:15

# Hoặc sử dụng Docker Compose
docker-compose -f deployment/docker-compose.yml up -d postgres
```

### 2. Cấu hình

Chỉnh sửa file `internal/service/notification/config/config.yaml`:

```yaml
server:
  port: 8082  # Port của notification service

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  dbname: "myapp"

notification:
  poller:
    enabled: true
    poll_interval_sec: 5
    batch_size: 1000
    max_queue_size: 2000
    backoff_on_empty_sec: 30
    processing_timeout_minutes: 5
  worker_concurrency: 10
  max_retries: 3
  retry_backoff_sec: 60
  senders:
    expo:
      enabled: true
      api_url: "https://exp.host/--/api/v2/push/send"
      access_token: ""  # Optional
      timeout_sec: 30
      max_retries: 3
    fcm:
      enabled: false
    apns:
      enabled: false
    email:
      enabled: false
```

### 3. Chạy Migrations

```bash
# Cách 1: Sử dụng go run
go run ./internal/service/notification/cmd/main.go migrate

# Cách 2: Build rồi chạy
go build -o notification-service.exe ./internal/service/notification/cmd
./notification-service.exe migrate
```

Migrations sẽ tạo các bảng:
- `notification` - Metadata của notification
- `notification_target` - Target và payload cho mỗi user
- `notification_delivery` - Lịch sử delivery và retry
- `device_tokens` - Device push tokens

### 4. Chạy Service

```bash
# Cách 1: Sử dụng go run (development)
go run ./internal/service/notification/cmd/main.go serve

# Cách 2: Build rồi chạy
go build -o notification-service.exe ./internal/service/notification/cmd
./notification-service.exe serve

# Cách 3: Chạy API-only mode (không có worker)
go run ./internal/service/notification/cmd/main.go api
```

Service sẽ chạy trên: **http://localhost:8082**

### 5. Kiểm tra Service

```bash
# Health check
curl http://localhost:8082/health

# Kiểm tra version
go run ./internal/service/notification/cmd/main.go version
```

## 📡 API Endpoints

### Tạo Notification

```bash
curl -X POST http://localhost:8082/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "order_created",
    "target_type": "user",
    "priority": 1,
    "trace_id": "trace-123",
    "targets": [
      {
        "user_id": "user-123",
        "payload": {
          "title": "Đơn hàng mới",
          "body": "Bạn có đơn hàng mới #12345",
          "channel_type": "expo"
        }
      }
    ]
  }'
```

### Đăng ký Device Token

```bash
curl -X POST http://localhost:8082/api/v1/notifications/tokens/register \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "device_id": "device-abc",
    "push_token": "ExponentPushToken[xxxxx]",
    "type": "expo",
    "platform": "ios"
  }'
```

### Lấy Failed Notifications

```bash
# Lấy failed notifications của user
curl http://localhost:8082/api/v1/notifications/users/user-123/failed?limit=10&offset=0

# Lấy failed notifications của user hiện tại (cần JWT token)
curl http://localhost:8082/api/v1/notifications/failed \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Retry Failed Notification

```bash
curl -X POST http://localhost:8082/api/v1/notifications/123/retry \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## 🔄 Luồng hoạt động

1. **Tạo Notification**: API tạo notification trong database với status `pending`
2. **Poller**: Poller định kỳ query database để lấy pending notifications
3. **Queue**: Poller đẩy notifications vào in-memory queue
4. **Worker**: Worker lấy notifications từ queue và xử lý
5. **Channel**: Worker gửi notification qua channel phù hợp (Expo, FCM, etc.)
6. **Delivery Tracking**: Cập nhật trạng thái delivery trong database
7. **Retry**: Tự động retry nếu gửi thất bại

### Flow Diagram

```
API Request
    ↓
Handler (handler/)
    ↓
Service (service/)
    ↓
Repository (repository/)
    ↓
Database (pending status)
    ↓
Poller (worker/poller.go) - Polls database
    ↓
Queue (worker/queue.go) - In-memory queue
    ↓
Worker (worker/worker.go) - Processes tasks
    ↓
Channel (channel/channel.go) - Sends notification
    ↓
External Service (Expo, FCM, etc.)
```

## 🛠️ Development

### Import Paths

```go
// Models
import "myapp/internal/service/notification/model"

// Config
import "myapp/internal/service/notification/config"

// Handler
import "myapp/internal/service/notification/handler"

// Service
import "myapp/internal/service/notification/service"

// Repository
import "myapp/internal/service/notification/repository"

// Worker
import "myapp/internal/service/notification/worker"

// Channel
import "myapp/internal/service/notification/channel"
```

### Chạy với hot reload (nếu có air)

```bash
air -c .air.toml
```

### Xem logs

Service sử dụng structured logging (JSON format). Logs bao gồm:
- `Notification created` - Notification được tạo
- `Poll completed` - Poller hoàn thành một lần poll
- `Notification sent successfully` - Gửi thành công
- `Notification send failed` - Gửi thất bại
- `Delivery reset to pending for retry` - Retry được lên lịch

### Debug

```bash
# Kiểm tra pending notifications trong database
psql -h localhost -U postgres -d myapp -c "SELECT COUNT(*) FROM notification_delivery WHERE status = 'pending';"

# Kiểm tra failed notifications
psql -h localhost -U postgres -d myapp -c "SELECT COUNT(*) FROM notification_delivery WHERE status = 'failed';"

# Xem device tokens
psql -h localhost -U postgres -d myapp -c "SELECT * FROM device_tokens LIMIT 10;"
```

## 📊 Monitoring

### Health Check

```bash
curl http://localhost:8082/health
```

### Database Metrics

```sql
-- Số lượng notifications theo status
SELECT status, COUNT(*) 
FROM notification_delivery 
GROUP BY status;

-- Số lượng notifications theo ngày
SELECT DATE(created_at) as date, COUNT(*) 
FROM notification 
GROUP BY DATE(created_at) 
ORDER BY date DESC;

-- Top users nhận nhiều notifications nhất
SELECT user_id, COUNT(*) as count 
FROM notification_target 
GROUP BY user_id 
ORDER BY count DESC 
LIMIT 10;
```

## 🔧 Configuration

### Environment Variables

Có thể override config bằng environment variables với prefix `APP_`:

```bash
export APP_SERVER_PORT=8082
export APP_DATABASE_HOST=localhost
export APP_NOTIFICATION_WORKER_CONCURRENCY=20
export APP_NOTIFICATION_POLLER_ENABLED=true
export APP_NOTIFICATION_POLLER_BATCH_SIZE=2000
```

### Channel Configuration

#### Expo Push Notifications

```yaml
notification:
  senders:
    expo:
      enabled: true
      api_url: "https://exp.host/--/api/v2/push/send"
      access_token: ""  # Optional, for authenticated requests
      timeout_sec: 30
      max_retries: 3
```

#### Firebase Cloud Messaging (FCM)

```yaml
notification:
  senders:
    fcm:
      enabled: true
      project_id: "your-project-id"
      credentials_file: "/path/to/credentials.json"
      timeout_sec: 30
      max_retries: 3
```

#### Apple Push Notification Service (APNS)

```yaml
notification:
  senders:
    apns:
      enabled: true
      key_id: "your-key-id"
      team_id: "your-team-id"
      bundle_id: "com.yourapp.bundle"
      key_file: "/path/to/key.p8"
      production: false
      timeout_sec: 30
      max_retries: 3
```

#### Email

```yaml
notification:
  senders:
    email:
      enabled: true
      smtp_host: "smtp.gmail.com"
      smtp_port: 587
      username: "your-email@gmail.com"
      password: "your-password"
      from_email: "noreply@yourapp.com"
      timeout_sec: 30
      max_retries: 3
```

## 🐛 Troubleshooting

### Service không kết nối được PostgreSQL

```bash
# Kiểm tra PostgreSQL đang chạy
docker ps | grep postgres

# Kiểm tra connection
psql -h localhost -U postgres -d myapp
```

### Notification không được xử lý

1. Kiểm tra poller có đang chạy:
   - Xem logs của service
   - Kiểm tra config `notification.poller.enabled = true`

2. Kiểm tra queue có đầy không:
   - Xem logs: "Queue is full, skipping poll"
   - Tăng `max_queue_size` trong config

3. Kiểm tra pending notifications trong database:
   ```sql
   SELECT COUNT(*) FROM notification_delivery WHERE status = 'pending';
   ```

4. Kiểm tra worker có đang xử lý:
   - Xem logs: "Notification sent successfully" hoặc "Notification send failed"
   - Kiểm tra `worker_concurrency` trong config

### Worker không xử lý messages

1. Kiểm tra worker concurrency trong config
2. Kiểm tra queue có messages không (xem logs)
3. Xem logs để tìm lỗi
4. Kiểm tra channel có enabled không

### Notification gửi thất bại

1. Kiểm tra device token có hợp lệ không
2. Kiểm tra channel configuration (Expo, FCM, etc.)
3. Xem logs để tìm lỗi cụ thể
4. Kiểm tra network connectivity đến external services

## 📝 Notes

- Service sử dụng in-memory queue để xử lý notifications
- Poller sử dụng `FOR UPDATE SKIP LOCKED` để tránh race condition
- Idempotency được đảm bảo bằng database checks
- Failed notifications có thể retry thủ công qua API
- Service hỗ trợ graceful shutdown
- Processing timeout: Nếu notification ở trạng thái `processing` quá lâu, sẽ được reset về `pending`

## 🔗 Related Documentation

- [Task Requirements](./task.md)
- [API Examples](../../../docs/API_EXAMPLES.md)
- [Deployment Guide](../../../docs/DEPLOYMENT.md)
- [Device Token Registration](./notification_register.md)
- [Worker Debug Guide](./DEBUG_WORKER.md)
