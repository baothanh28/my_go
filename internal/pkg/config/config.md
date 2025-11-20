config.md – Common Config Package Design (Golang)
Step 1: Understand the problem
Problem Overview

Trong hệ thống microservice, mỗi service đều cần tải và quản lý cấu hình:

Database config

Cache config

Message broker config

External API keys

Feature flags

Runtime overrides

Nếu mỗi service tự implement logic config thì:

Không đồng nhất

Khó test

Thừa code

Khó mở rộng

Dễ lỗi khi multi-environment

Do đó cần một common config package, tiêu chuẩn hóa cách load, validate, override và expose config.

Functional Requirements

Load configuration từ các nguồn:

File .env, .yaml, .json

Environment variables

Inline defaults

Remote config provider (tùy chọn)

Merge logic:

Thứ tự ưu tiên: env > file > default

Support nested config struct

Validation:

Validate tự động sau khi load

Support custom validator (ex: email, url, duration)

Hot Reload (optional):

Watch file (if enabled)

Callback on change

Access API:

Lấy cấu hình theo type-safe struct

Lấy giá trị kiểu đơn: GetInt, GetString, GetDuration

Support multi-environment:

dev / staging / prod

config override theo env

Support secrets separation:

Non-secret file config

Secret từ ENV hoặc vault

Non-Functional Requirements

Thread-safe

Zero-memory overhead sau load

Không có dependency nặng (ưu tiên stdlib + một số gói nhẹ như Viper, envconfig)

Fast startup (dưới 5ms)

Easy plug-and-play

Testable: mockable config provider

Back-of-the-envelope Estimations

Số lượng config keys: ~50–200

Kích thước file config: 1–5 KB

Số lần đọc config: 1 lần (startup), hot reload tuỳ option

Overhead CPU: rất nhỏ (chủ yếu parse file)

Overhead RAM: vài KB

Step 2: High-level Design
┌────────────────────────────┐
│        Config API          │
│   - Load()                 │
│   - Reload()               │
│   - Get() / Unmarshal()    │
└───────────────┬────────────┘
                │
┌───────────────▼────────────────┐
│       Config Manager           │
│  - merge providers             │
│  - priority resolution         │
│  - validation                  │
└───────────────┬────────────────┘
                │
 ┌──────────────▼────────────────┐
 │      Providers Layer          │
 │ file provider | env provider  │
 │ default provider | remote     │
 └───────────────────────────────┘

Step 3: Design Deep Dive
1. Config Provider Interface
type Provider interface {
    Load() (map[string]interface{}, error)
    Name() string
}

Providers bao gồm:

FileProvider (.env, yaml, json)

EnvProvider

DefaultProvider

RemoteProvider (Consul, Vault, SSM, etc.)

2. Config Manager
type ConfigManager interface {
    Load() error
    Get(key string) interface{}
    Unmarshal(target interface{}) error
    Watch(onChange func()) error
}

Chức năng:

Load tất cả provider theo thứ tự ưu tiên

Merge config

Validate

Expose API cho business code

3. Struct-based Configuration

Golang service dùng:

type AppConfig struct {
    AppName string `validate:"required"`
    LogLevel string `validate:"oneof=debug info error"`

    Database struct {
        DSN string `validate:"required,url"`
        MaxConn int `validate:"gte=1,lte=100"`
    }

    Cache struct {
        RedisURL string `validate:"required"`
        Timeout  time.Duration
    }
}

4. Validation Layer

Sử dụng go-playground/validator:

Validate sau khi merge

Custom validators (port range, duration, url)

5. Hot Reload (Optional)

File watcher → re-parse config → validate → trigger callback

Shutdown-safe, thread-safe reload

6. Config Security

Secrets KHÔNG lưu file

Chỉ load qua environment / secret provider

Tự động mask khi log (****)

Final Interfaces
Provider Interface
type Provider interface {
    Load() (map[string]any, error)
    Name() string
}

Config Manager Interface
type Config interface {
    Load() error
    Reload() error
    Get(key string) any
    Unmarshal(target any) error
    Watch(callback func()) error
}

Helper Functions
func GetString(key string) string
func GetInt(key string) int
func GetDuration(key string) time.Duration

Ready-to-Use Example
cfg := config.New(
    config.WithProvider(config.NewDefaultProvider()),
    config.WithProvider(config.NewFileProvider("config.yaml")),
    config.WithProvider(config.NewEnvProvider()),
)

if err := cfg.Load(); err != nil {
    panic(err)
}

var appCfg AppConfig
cfg.Unmarshal(&appCfg)