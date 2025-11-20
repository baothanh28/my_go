| Mục tiêu                             | Mô tả                                                                                                                       |
| ------------------------------------ | --------------------------------------------------------------------------------------------------------------------------- |
| **1. Lắng nghe sự kiện từ Supabase** | Service Golang dùng **pgx** để `LISTEN` khi có bản ghi mới trong bảng `notification`.                                       |
| **2. Gửi sự kiện qua Redis**         | Dùng **Redis Streams** làm **relay layer** trung gian – lưu trữ, điều phối, retry và phân phối notification đến các worker. |
| **3. Xử lý gửi tin nhắn**            | Các **worker Golang** đọc từ Redis stream, gọi các sender (Expo, FCM, APNs...) để gửi notification đến thiết bị người dùng. |
| **4. Đảm bảo độ tin cậy cao**        | Có cơ chế retry, delayed queue, DLQ (dead-letter queue), idempotency key, và tự khôi phục sau crash.                        |
| **5. Loại bỏ Prometheus**            | Thay vì metrics Prometheus, hệ thống dùng **structured logging + Redis counters** để theo dõi, giúp giảm phức tạp vận hành. |
| **6. Hỗ trợ luồng pull**             | Người dùng có thể **pull** lại các notification thất bại (`notification_failed`) qua API Golang.                            |
| **7. Khả năng mở rộng**              | Thiết kế **stateless** và **distributed** — có thể mở rộng bằng cách tăng số lượng worker hoặc Redis shard.                 |



### Tasks (line-by-line — implement bằng Go, dùng Redis là relay, loại bỏ Prometheus)

1. **Tạo repo mới**: `git init notifications-go-redis` + `go mod init github.com/<you>/notifications-go-redis`.
2. **Tạo folder cấu trúc**: `cmd/`, `internal/`, `pkg/`, `configs/`, `deploy/`, `scripts/`, `docs/`.
3. **Cài đặt dependencies cơ bản**: pgx, redis, zerolog, viper, backoff, uuid, migrate.
4. **Viết file config** `configs/config.yaml` chứa DSN, Redis addr, consumer group, concurrency, batch sizes, retry policy.
5. **Tạo package config loader** dùng viper load config.
6. **Tạo logger** dùng zerolog, include trace id.
7. **DB migration skeleton**: tạo bảng `notification`, `notification_failed`, indexes, created_at, status, payload JSONB, attempt_count, last_error, delivered_at.
8. **Tạo module DB** `internal/db/db.go` wrapper pgx pool + expose `SubscribeToNotificationChanges()`.
9. **Implement listener (pgx)**: Connect, LISTEN notification_insert, parse payload, push Redis stream `XADD stream:notifications * id <uuid> payload <json>`.
10. **Schema payload policy**: id, user_id, type, data JSON, priority, created_at, trace_id.
11. **Thiết kế Redis relay**: Redis Streams + Consumer Groups (stream:notifications, cg:notifications, DLQ stream:notifications:dlq).
12. **Tạo module Redis client** wrapper XAdd, XReadGroup, XAck, XClaim, XTrim.
13. **Implement worker dispatcher**: XREADGROUP LOOP, parse, validate idempotency, call sender function.
14. **Idempotency logic**: Redis key delivered:<id> SETNX + TTL 7d, update attempt_count trong Postgres.
15. **Sender module**: expo, fcm, apns, email (trả về ok, err, retryable).
16. **Sender expo example**: HTTP client retry/backoff, handle rate limits 429.
17. **Failure handling**: retryable -> requeue, non-retryable -> mark Postgres + push DLQ.
18. **Delayed retry**: Redis Sorted Set zadd delayed:<ts> + scheduler move due items -> stream.
19. **Delayed Scheduler service**: poll ZRANGEBYSCORE, ZREM, XADD back to stream.
20. **Pending/Claim handling**: worker XAUTOCLAIM stale msg.
21. **Batch Ack**: XACK sau khi send success, optional XDEL trim.
22. **Redis Stream trimming**: XTRIM MAXLEN ~100k.
23. **Records GC**: Trim DLQ + main stream, archive old items.
24. **Postgres store APIs**: MarkDelivered, IncrementAttempt, InsertFailed, GetPendingFailedForUser.
25. **User pull API**: GET /users/{id}/notifications/failed, POST /notifications/{id}/retry.
26. **Healthcheck & readiness**: GET /healthz (DB, Redis), GET /readyz (consumer group exists).
27. **Logging (no Prometheus)**: structured logs events: ENQUEUE, DEQUEUE, SEND_SUCCESS, SEND_FAIL, RETRY_SCHEDULED, DLQ_PUSHED.
28. **Optional metrics**: store counters Redis keys metrics:notifications:sent:YYYYMMDD.
29. **Tracing minimal**: trace_id trong payload/log.
30. **Configurable concurrency**: env WORKER_CONCURRENCY.
31. **Graceful shutdown**: SIGINT/SIGTERM stop read, wait, XACK inflight.
32. **Testing**: unit (miniredis, test DB), integration (docker-compose Postgres+Redis).
33. **Load testing**: script simulate >10M/month.
34. **Redis sizing & config**: Redis cluster, memory, eviction policy.
35. **Rate limiting**: token bucket per recipient/provider.
36. **Idempotency guarantee**: Redis SETNX + Postgres check.
37. **Admin tools**: requeue_failed.go, inspect_dlq.go.
38. **Observability**: central logs ELK/Graylog/Loki.
39. **K8s manifests**: listener, worker, delay-scheduler, api, ConfigMap/Secrets.
40. **CI/CD**: build/test, push image, apply manifests.
41. **Security**: TLS Redis/DB, rotate creds, RBAC.
42. **Monitoring health**: alert via logs, monitor DLQ len.
43. **Cleanup Prometheus**: remove prom client, ServiceMonitors, CI/CD configs.
44. **Documentation**: README with diagram, env vars, redis key convention.
45. **Rollout plan**: deploy staging, smoke test, scale gradually.
46. **Operational runbook**: claim stuck msg, requeue DLQ, reset consumer group.
47. **Performance tuning**: adjust XREADGROUP COUNT, goroutines, Redis maxmemory-policy.
48. **Data retention policy**: TTL delivered keys, trim streams, archive failed.
49. **Auto-recovery**: XAUTOCLAIM pending messages.
50. **Final checklist**: tests pass, endpoints green, DLQ works, no Prometheus remains.
