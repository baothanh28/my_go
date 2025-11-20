# 🧱 Common Worker Package — Design Document

## Step 1: Understand the Problem

### 🎯 Functional Requirements

* Worker phải xử lý được nhiều loại tác vụ khác nhau (generic task).
* Cho phép đóng gói logic xử lý trong các module riêng, inject vào worker.
* Hỗ trợ cách chạy:

  * **Pull-based** (worker tự lấy job từ queue)
  * **Push-based** (work dispatcher gửi thẳng job vào worker)
* Worker phải hỗ trợ retry logic:

  * Retry theo số lần tối đa
  * Delay giữa các lần retry
* Worker phải hỗ trợ:

  * Timeout cho mỗi task
  * Cancellation (context)
* Collect được metrics:

  * Task processed
  * Task failed
  * Task duration
* Log có cấu trúc (tracing ID theo request/job ID)
* Worker phải **graceful shutdown**
* Worker phải cho phép cấu hình:

  * Số lượng goroutine worker (concurrency)
  * Backoff strategy (linear/exponential)
* Worker phải hỗ trợ middleware cho:

  * Logging
  * Metrics
  * Panic recovery

---

### 🛡️ Non-Functional Requirements

* High-throughput: vài nghìn task/giây
* Low-latency processing
* Mở rộng dễ dàng (stateless + scale horizontally)
* Không phụ thuộc mạnh vào một queue cụ thể (Redis, SQS, NATS...)
* Thread-safe
* Code clean, dễ maintain
* Backward-compatible cho các hệ thống về sau
* Thử nghiệm đơn vị dễ viết (low coupling)

---

### 🧮 Back-of-the-Envelope Estimations

* Task size trung bình: ~1KB
* Số tác vụ/ngày: 10M → ~115 task/sec
* Concurrency tối ưu mỗi instance: 50–200 goroutine worker
* CPU-bound: Worker ít tốn CPU (IO mainly)
* Memory: mỗi task context ~50KB → 100 goroutine ~5MB overhead

---

## Step 2: High-Level Design

### 🧩 Worker Service

Đây là service chính, quản lý:

* Task dispatcher
* Worker pool
* Retry mechanism
* Middleware pipeline

Nó sẽ expose:

* Hàm đăng ký handler
* Hàm start/stop worker
* Config

---

### ⚙️ Task Executor

Component chịu trách nhiệm:

* Thực thi từng task cụ thể
* Áp dụng middleware: logging, metrics, tracing
* Panic recovery
* Timeout

---

### 📮 Task Provider

Đây là nơi worker lấy job:

* Redis Stream
* Redis List
* Kafka
* NATS
* SQS

Tất cả task provider tuân theo 1 interface chung để dễ mở rộng.

---

### 🧾 Task Definition

Đây là define chuẩn cho job:

* JobID
* Payload
* Metadata
* RetryCount
* MaxRetry
* Deadline / Timeout

---

### 📚 Worker Registry

Giữ danh sách các task handler đã đăng ký.

---

### 💼 Worker Context (Execution Context)

Mỗi task có context riêng

* CorrelationID
* Deadline
* Logger scoped
* Tracing

---

### 🔁 Retry / Backoff Engine

Giống “Double-entry ledger system” — xử lý logic retry & đảm bảo consistency:

* Retry strategy
* Exponential backoff
* Dead-letter queue

---

## Step 3: Design Deep Dive

### 🔌 Task Provider Integration

* Task provider phải độc lập với core worker
* Interface Provider:

```go
type Provider interface {
    Fetch(ctx context.Context) (*Task, error)
    Ack(ctx context.Context, task *Task) error
    Nack(ctx context.Context, task *Task, requeue bool) error
}
```

---

### ✔️ Reconciliation

Khi worker crash:

* Nhiệm vụ chưa ack phải được trả về queue
* Nếu provider không hỗ trợ auto-rollback → phải implement event sourcing
* Worker phải đảm bảo không drop job

---

### 🕒 Handling Processing Delays

* Task timeout
* Slow handler detection
* Metrics time histogram
* Optional: circuit breaker

---

### ❌ Handling Failed Tasks

* Retry
* Dead letter queue
* Persist retry count

Flow khi fail:

```
process task → error → retry engine → requeue / DLQ
```

---

### 🔄 Exactly-Once Delivery

* Trong worker cấp application là **"At-least-once"**
* Chống duplicate bằng:

  * Idempotent task handler
  * Dedup store (Redis SETEX, Bloom Filter)
  * Provider supporting explicit ACK

---

### 🔗 Consistency

* Không xử lý 1 job 2 lần
* Không mất job khi worker crash
* Context xuyên suốt pipeline
* Transaction-like flow:

  ```
  Fetch → Execute → Ack
  ```
* Logs + metrics là một phần của consistency để debug

---

### 🔐 Worker Security

* Không thực thi eval code
* Validate input payload
* Chỉ load handler đã đăng ký trước
* Apply timeout để tránh task treo
* Sandbox queue input nếu cần (limit size)

---

# 🎯 Golang Interfaces (Output cuối cùng bạn dùng để implement)

## `Task`

```go
type Task struct {
    ID        string
    Payload   []byte
    Metadata  map[string]string
    Retry     int
    MaxRetry  int
    Timeout   time.Duration
    CreatedAt time.Time
}
```

## `Handler`

```go
type Handler interface {
    Process(ctx context.Context, task *Task) error
}
```

## `Provider`

```go
type Provider interface {
    Fetch(ctx context.Context) (*Task, error)
    Ack(ctx context.Context, task *Task) error
    Nack(ctx context.Context, task *Task, requeue bool) error
}
```

## `Middleware`

```go
type Middleware func(Handler) Handler
```

## `Worker`

```go
type Worker interface {
    Register(name string, handler Handler)
    Use(mw Middleware)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

## `WorkerConfig`

```go
type WorkerConfig struct {
    Concurrency int
    Backoff     BackoffStrategy
}
```

---

## Next steps (gợi ý)

* Tạo skeleton code theo interfaces trên
* Implement 1 Provider (ví dụ: Redis Stream)
* Implement 1 Handler ví dụ
* Unit test cho middleware + retry
* Ví dụ deploy config và load test
