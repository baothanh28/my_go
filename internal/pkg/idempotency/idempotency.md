Step 1: Understand the Problem
1. Functional Requirements

Common idempotency package phải cung cấp khả năng:

Đảm bảo tính idempotent cho bất kỳ tác vụ nào có nguy cơ được gọi nhiều lần (retry, network issue, job re-run…).

Cung cấp cơ chế:

Generate Idempotency Key nếu caller không cung cấp.

Check (Load) trạng thái của một key: not-exist, processing, completed, failed.

Mark processing để đảm bảo một tác vụ chỉ được xử lý duy nhất.

Store result sau khi xử lý xong.

Return cached output nếu lần gọi lại cùng một key.

Có thể tích hợp vào nhiều domain khác nhau:

API HTTP

Background worker

Scheduler

Distributed job executor

Hỗ trợ custom TTL, custom serialization, custom storage backend.

Có thể mở rộng sang Redis, SQL, DynamoDB, in-memory.

2. Non-functional Requirements

High performance: xử lý được request với latency thấp.

Thread-safe / goroutine-safe.

Distributed-safe: nhiều instance chạy song song không gây race condition.

Pluggable backend: không phụ thuộc 1 storage cố định.

Observability: hỗ trợ expose:

metrics: success, hit-cache, write-failure

logs: duplication detected, conflicting keys

Timeout/TTL hợp lý để dọn dữ liệu cũ.

Extensibility: cho phép override serializer, key generator, storage impl.

3. Back-of-the-envelope Estimations

(Bạn có thể tùy chỉnh theo hệ thống)

Số request/s: 5.000 – 20.000 req/s.

Size mỗi record: ~1–5 KB.

TTL: từ 5 phút đến 24 giờ tùy use case.

Nếu dùng Redis:

Pipeline + SET NX là bắt buộc.

Memory cần: ~200MB/1M records (tùy payload).

Expected duplication rate: ~10% do retry / client-side timeout.

Step 2: High-level Design
1. Idempotency Flow
Client → Handler → IdempotencyService → Storage → (task execution)


Flow hoạt động:

Handler nhận request.

Generate hoặc lấy idempotency key.

Load(key) từ storage:

Nếu status = COMPLETED → trả về cached result.

Nếu status = PROCESSING → trả về error hoặc wait.

MarkProcessing(key)

Xử lý tác vụ (business logic).

SaveResult(key, result)

Return result.

Step 3: Define Interfaces (Quan trọng nhất)

Dưới đây là interface chuẩn cho package idempotency.

1. Storage Interface
type Status string

const (
    StatusNone       Status = "none"
    StatusProcessing Status = "processing"
    StatusCompleted  Status = "completed"
    StatusFailed     Status = "failed"
)

type Record struct {
    Key       string
    Status    Status
    Result    []byte
    ErrorMsg  string
    CreatedAt time.Time
    UpdatedAt time.Time
    TTL       time.Duration
}

type Storage interface {
    // Load trả về record nếu tồn tại, nếu không trả về nil
    Load(ctx context.Context, key string) (*Record, error)

    // TryMarkProcessing dùng cơ chế atomic (Redis SETNX, SQL INSERT) để đảm bảo một thread/process duy nhất được xử lý
    TryMarkProcessing(ctx context.Context, key string, ttl time.Duration) (bool, error)

    // SaveResult lưu kết quả thành completed
    SaveResult(ctx context.Context, key string, result []byte, ttl time.Duration) error

    // SaveError lưu trạng thái failed
    SaveError(ctx context.Context, key string, errMsg string, ttl time.Duration) error
}

2. Serializer Interface (optional)
type Serializer interface {
    Marshal(v any) ([]byte, error)
    Unmarshal(data []byte, v any) error
}

3. Key Generator Interface
type KeyGenerator interface {
    Generate(input any) (string, error)
}

4. Idempotency Service Interface
type Service interface {
    // Execute đảm bảo tính idempotent cho hàm f
    Execute[T any](
        ctx context.Context,
        key string,
        ttl time.Duration,
        fn func(ctx context.Context) (T, error),
    ) (T, error)
}

5. Default Implementation Structure
type service struct {
    storage     Storage
    serializer  Serializer
}

func NewService(storage Storage, serializer Serializer) Service {
    return &service{
        storage:    storage,
        serializer: serializer,
    }
}

Step 4: High-level Implementation Strategy

Redis backend

Dùng SET NX PX ttl để MarkProcessing.

Dùng JSON hoặc msgpack để serialize result.

SQL backend

Dùng table idempotency_records.

Dùng INSERT ... ON CONFLICT DO NOTHING.

Memory backend

Dùng sync.Map + mutex.

Chỉ phù hợp test/local.

Step 5: Examples
1. Usage in HTTP Handler
result, err := idemService.Execute(ctx, idKey, 5*time.Minute, func(ctx context.Context) (MyResponse, error) {
    return processPayment(ctx, req)
})

2. Usage in Worker
jobID := fmt.Sprintf("job-%d", msg.ID)

idemService.Execute(ctx, jobID, time.Hour, func(ctx context.Context) (string, error) {
    return runJob(ctx, msg)
})
Step 6: Future Enhancements
Token-based idempotency
Outbox pattern integration
Add metrics: duplication_hit, storage_errors, process_duration
Distributed lock fallback
Pluggable encryption layer
Multi-tenancy support