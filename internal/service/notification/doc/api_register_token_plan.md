# Kế hoạch thêm API Register Token (chạy độc lập với Worker)

## Tổng quan
Thêm API endpoint để đăng ký push notification tokens (Expo, FCM, APNS, Native) và cho phép chạy API server độc lập với worker.

## Các loại Token Type

- **expo**: Expo Push Token (ExponentPushToken[...])
- **fcm**: Firebase Cloud Messaging token
- **apns**: Apple Push Notification Service token
- **native**: Native push token (tùy chỉnh)

## Cấu trúc thay đổi

### 1. Tạo Model cho Device Token
**File**: `internal/service/notification/model.go`

Thêm struct mới:
```go
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
```

### 2. Thêm Repository Methods
**File**: `internal/service/notification/repository.go`

Thêm các methods:
```go
// RegisterDeviceToken registers or updates a device token
func (r *NotificationRepository) RegisterDeviceToken(dto RegisterTokenDTO) (*DeviceToken, error) {
    var token DeviceToken
    
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
        token = DeviceToken{
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
func (r *NotificationRepository) GetDeviceTokensByUserID(userID string) ([]*DeviceToken, error) {
    var tokens []*DeviceToken
    if err := r.db.Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
        return nil, fmt.Errorf("failed to get device tokens: %w", err)
    }
    return tokens, nil
}

// DeleteDeviceToken deletes a device token
func (r *NotificationRepository) DeleteDeviceToken(userID, deviceID string) error {
    result := r.db.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&DeviceToken{})
    if result.Error != nil {
        return fmt.Errorf("failed to delete device token: %w", result.Error)
    }
    if result.RowsAffected == 0 {
        return gorm.ErrRecordNotFound
    }
    return nil
}
```

### 3. Thêm Service Methods
**File**: `internal/service/notification/service.go`

Thêm method:
```go
// RegisterDeviceToken registers or updates a device token
func (s *NotificationService) RegisterDeviceToken(dto RegisterTokenDTO) (*RegisterTokenResponse, error) {
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
    
    return &RegisterTokenResponse{
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
```

### 4. Thêm Handler Methods
**File**: `internal/service/notification/handler.go`

Thêm handler:
```go
// RegisterToken handles device token registration
func (h *NotificationHandler) RegisterToken(c echo.Context) error {
    var dto RegisterTokenDTO
    if err := c.Bind(&dto); err != nil {
        return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid request body")
    }
    
    // Validation
    if dto.UserID == "" {
        return server.ErrorResponse(c, http.StatusBadRequest, nil, "user_id is required")
    }
    if dto.DeviceID == "" {
        return server.ErrorResponse(c, http.StatusBadRequest, nil, "device_id is required")
    }
    if dto.PushToken == "" {
        return server.ErrorResponse(c, http.StatusBadRequest, nil, "push_token is required")
    }
    if dto.Type == "" {
        return server.ErrorResponse(c, http.StatusBadRequest, nil, "type is required (expo, fcm, apns, native)")
    }
    if dto.Platform == "" {
        return server.ErrorResponse(c, http.StatusBadRequest, nil, "platform is required (ios, android, web)")
    }
    
    // Optional: Verify user from auth context
    if userCtx, err := auth.GetUserFromContext(c); err == nil {
        if dto.UserID != strconv.FormatUint(uint64(userCtx.UserID), 10) && userCtx.Role != "admin" {
            return server.ErrorResponse(c, http.StatusForbidden, nil, "Forbidden")
        }
    }
    
    result, err := h.service.RegisterDeviceToken(dto)
    if err != nil {
        h.logger.Error("Failed to register device token", zap.Error(err))
        return server.ErrorResponse(c, http.StatusInternalServerError, err.Error(), "Failed to register device token")
    }
    
    return server.SuccessResponse(c, http.StatusOK, result, "Device token registered successfully")
}
```

### 5. Cập nhật Routes
**File**: `internal/service/notification/app.go`

Cập nhật function `registerNotificationRoutes`:
```go
func registerNotificationRoutes(params NotificationRoutesParams) {
    e := params.Server.GetEcho()
    
    protectedGroup := e.Group("/api/v1/notifications")
    
    // Existing routes
    protectedGroup.POST("", params.Handler.CreateNotification)
    protectedGroup.GET("/users/:user_id/failed", params.Handler.GetFailedNotifications)
    protectedGroup.GET("/failed", params.Handler.GetFailedNotifications)
    protectedGroup.POST("/:id/retry", params.Handler.RetryNotification)
    
    // New token registration route
    protectedGroup.POST("/tokens/register", params.Handler.RegisterToken)
}
```

### 6. Tạo Migration cho bảng device_tokens
**File**: Tạo migration mới hoặc thêm vào migration hiện có

```sql
CREATE TABLE IF NOT EXISTS device_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    push_token VARCHAR(500) NOT NULL,
    type VARCHAR(50) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT unique_user_device UNIQUE (user_id, device_id)
);

CREATE INDEX idx_device_tokens_user_id ON device_tokens(user_id);
CREATE INDEX idx_device_tokens_device_id ON device_tokens(device_id);
CREATE INDEX idx_device_tokens_type ON device_tokens(type);
CREATE INDEX idx_device_tokens_deleted_at ON device_tokens(deleted_at);
```

### 7. Thêm Command mới trong cmd/main.go
**File**: `internal/service/notification/cmd/main.go`

Thêm command mới `api` để chạy chỉ API server (không có worker):

```go
var apiCmd = &cobra.Command{
    Use:   "api",
    Short: "Start the notification API server only (without worker)",
    Run: func(cmd *cobra.Command, args []string) {
        runAPI()
    },
}

func init() {
    rootCmd.AddCommand(serveCmd)
    rootCmd.AddCommand(migrateCmd)
    rootCmd.AddCommand(versionCmd)
    rootCmd.AddCommand(apiCmd)  // Thêm command mới
}

func runAPI() {
    app := fx.New(
        notification.NotificationAppAPIOnly,  // Sử dụng module mới không có worker
        fx.NopLogger,
    )

    startCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
    defer cancel()

    if err := app.Start(startCtx); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to start notification API server: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("Notification API server started successfully on http://localhost:8082")
    fmt.Println("Worker is NOT running - only API endpoints are available")

    <-app.Done()

    stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
    defer cancel()

    if err := app.Stop(stopCtx); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to stop notification API server: %v\n", err)
        os.Exit(1)
    }
}
```

### 8. Tạo Module mới cho API Only
**File**: `internal/service/notification/app.go`

Thêm module mới `NotificationAppAPIOnly` không khởi động worker:

```go
// NotificationAppAPIOnly provides notification service dependencies without worker
var NotificationAppAPIOnly = fx.Options(
    // Infrastructure modules
    config.WithServiceDir("internal/service/notification"),
    config.Module,
    logger.Module,
    database.Module,
    server.Module,

    // Notification service components (không có worker)
    fx.Provide(
        NewServiceConfig,
        NewNotificationRepository,
        NewNotificationService,
        NewNotificationHandler,
        // Không include: NewSenderRegistry, provideInMemoryQueue, provideInMemoryProvider,
        // provideNotificationPoller, provideNotificationWorker
    ),

    // Register routes
    fx.Invoke(registerNotificationRoutes),

    // KHÔNG invoke startBackgroundServices - chỉ chạy API server
)
```

## Cách sử dụng

### Chạy API server độc lập (không có worker):
```bash
go run ./internal/service/notification/cmd/main.go api
```

### Chạy full service (có cả API và worker):
```bash
go run ./internal/service/notification/cmd/main.go serve
```

### Test API register token:

**Ví dụ với Expo token:**
```bash
curl -X POST http://localhost:8082/api/v1/notifications/tokens/register \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123",
    "device_id": "device-abc-xyz-123",
    "push_token": "ExponentPushToken[xxxxxx]",
    "type": "expo",
    "platform": "ios"
  }'
```

**Ví dụ với FCM token:**
```bash
curl -X POST http://localhost:8082/api/v1/notifications/tokens/register \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123",
    "device_id": "device-abc-xyz-123",
    "push_token": "fcm-token-xxxxx",
    "type": "fcm",
    "platform": "android"
  }'
```

**Ví dụ với APNS token:**
```bash
curl -X POST http://localhost:8082/api/v1/notifications/tokens/register \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123",
    "device_id": "device-abc-xyz-123",
    "push_token": "apns-token-xxxxx",
    "type": "apns",
    "platform": "ios"
  }'
```

**Ví dụ với Native token:**
```bash
curl -X POST http://localhost:8082/api/v1/notifications/tokens/register \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123",
    "device_id": "device-abc-xyz-123",
    "push_token": "native-token-xxxxx",
    "type": "native",
    "platform": "ios"
  }'
```

## Tóm tắt các file cần thay đổi

1. ✅ `model.go` - Thêm DeviceToken model và DTOs
2. ✅ `repository.go` - Thêm methods để quản lý device tokens
3. ✅ `service.go` - Thêm RegisterDeviceToken method
4. ✅ `handler.go` - Thêm RegisterToken handler
5. ✅ `app.go` - Thêm NotificationAppAPIOnly module và cập nhật routes
6. ✅ `cmd/main.go` - Thêm `api` command
7. ✅ Migration file - Tạo bảng device_tokens

## Lưu ý

- API server độc lập sẽ không xử lý notifications (không có worker)
- Chỉ dùng để đăng ký tokens và các API endpoints khác
- Worker vẫn cần chạy riêng bằng command `serve` để xử lý notifications
- Có thể chạy nhiều instance API server và worker riêng biệt để scale

