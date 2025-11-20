pgnotify.md
Step 1: Understand the Problem
Functional Requirements

Common pgnotify package cần cung cấp một abstraction đơn giản, ổn định, có thể tái sử dụng cho nhiều tác vụ khác nhau dựa trên PostgreSQL LISTEN/NOTIFY.

Các chức năng chính:

Publish event

Cho phép gửi NOTIFY đến channel với payload.

Hỗ trợ text và byte payload.

Subscribe

Lắng nghe nhiều channel khác nhau.

Callback khi có message.

Cho phép hủy đăng ký.

Connection management

Kết nối PostgreSQL ổn định.

Tự động reconnect khi connection bị mất.

Tự động re-LISTEN sau khi reconnect.

Observability

Hooks cho logging.

Hooks cho metrics (message count, error count, reconnect count).

Graceful shutdown

Dừng lắng nghe, đóng connection an toàn, không leak goroutine.

Non-functional Requirements

Reliability

Không crash khi Postgres mất kết nối.

Reconnect với backoff.

Performance

Độ trễ từ notify đến callback thấp.

Scalability

Hỗ trợ nhiều channel đồng thời.

Callback xử lý đồng thời, không block global listener.

Concurrency Safety

Thread-safe khi publish và manage subscriptions.

Extensibility

Không ràng buộc domain.

Cho phép override logger, metrics, error handler.

Portability

Dùng được với pgx, database/sql, hoặc implement backend khác.

Back-of-the-envelope Estimations

Số lượng event: thường 10k–100k/hour tùy use case.

Payload tối đa 8 KB theo giới hạn PostgreSQL.

Mỗi listener có thể hoạt động bằng 1 goroutine.

Reconnect trung bình 100–200 ms.

Step 2: High-Level Design
Core Concepts

Notifier

Component chính

Publish/Subscribe

Quản lý kết nối

Subscription

Tương ứng 1 LISTEN channel

Callback được gọi khi có notify

Dispatcher

Nhận notify từ Postgres

Trigger callback

Bắt lỗi, ghi metrics

Connection Supervisor

Detect connection drop

Reconnect

Re-register LISTEN channel

Configuration

DSN

Reconnect interval

Max reconnect

Logger / Metrics / Hooks

High-level Flow
Publishing
Client → Notifier → PostgreSQL NOTIFY

Subscribing
Client → Register subscription
Notifier → LISTEN channel
Postgres → NOTIFY → Notifier → Dispatcher → Callback

Reconnect Flow
Connection lost → Supervisor → Reconnect → Re-LISTEN → Resume receiving events

Step 3: Design Deep Dive
Subscription lifecycle

User gọi Subscribe("channel", callback)

Notifier:

Tạo subscription object

Gửi lệnh LISTEN

Lưu subscription vào map

Khi callback destroy → gỡ khỏi map → gửi UNLISTEN

Message dispatching

Notifier nhận message thông qua Postgres connection listener

Dispatcher:

Resolve subscription theo channel

Gọi callback trong goroutine riêng

Handle panic

Ghi metrics

Handling connection drop

Sử dụng goroutine giám sát connection

Nếu mất kết nối:

Log + metrics

Reconnect với backoff

Gửi lại toàn bộ LISTEN

Continue receive

Error handling

Callback error không được propagate về Postgres

Notifier tự xử lý:

Log

Tăng error metrics

Delivery semantics

PostgreSQL LISTEN/NOTIFY = at-least-once

Không đảm bảo exactly-once

Nếu muốn, user phải tự implement:

idempotent callback

deduplication

sequence number

Security Considerations

Không gửi thông tin nhạy cảm qua payload

Payload nên encode theo một format ổn định (JSON, protobuf…)

Cho phép whitelist channel để tránh subscribe nhầm

Thêm MaxPayloadSize để tránh misuse

Use cases examples (generic, non-domain)

Cache invalidation

Realtime notification

Distributed lock coordination

Trigger background job

Event propagation giữa microservices

Hot-reload configuration

System health reporting