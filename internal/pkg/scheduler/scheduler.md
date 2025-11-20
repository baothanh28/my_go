# 📦 Common Scheduler Package — Design Document

This document defines a **generic, reusable Scheduler package** written in **Golang**. The goal is to build a robust scheduling system for running recurring or delayed tasks across different services.

---

# Step 1: Understand the Problem

## Functional Requirements

* Provide a unified scheduler API usable across multiple modules/services.
* Support recurring jobs (cron-like), delayed jobs, and one-time scheduled jobs.
* Allow job registration with metadata (name, schedule, timeout, retry policy, etc.).
* Allow job cancellation, pausing, and resuming.
* Support distributed scheduling (multiple instances — only one executes a job).
* Expose metrics and logs for observability.
* Allow pluggable backends (e.g., Redis, DB, in-memory).
* Provide safe shutdown ensuring in-flight job completion.
* Allow custom job handler functions injected from user services.

## Non-Functional Requirements

* High availability when multiple instances of the scheduler service are running.
* Horizontal scalability under high scheduling load.
* Minimal jitter — jobs should fire as close as possible to schedule time.
* Fault tolerance: nodes can crash and new nodes recover job execution safely.
* Exactly-once or at-least-once execution guarantee based on user configuration.
* Low-latency lock acquisition for distributed scheduling.
* Low CPU and memory overhead.
* Interface-based design to maximize testability.

## Back-of-the-Envelope Estimations

Assume the following constraints:

* ~ 500–20,000 scheduled jobs per service.
* ~ 100–500 jobs executed per minute.
* Distributed environment with ~ 3–10 instances.
* Redis backend latency ~1–5 ms per read/write.
* Job handler execution varies: 10 ms – 1 minute.
* TTL-based lock expiration usually 10–30 seconds.

These assumptions help define lock strategy, refresh intervals, and queue depth.

---

# Step 2: High-Level Design

## Scheduler Core

Responsible for orchestrating job registration, execution, tracking, and lifecycle.

* Loads jobs from storage during startup.
* Starts tickers/timers for job due time.
* Dispatches job execution to worker pool.
* Ensures job execution safety during shutdown.

## Job Executor

* Responsible for executing the job's handler.
* Applies timeout, retry logic, panic recovery.
* Reports metrics and logs.

## Backend Provider

Pluggable persistence + locking system.

* Redis-based backend (primary use-case).
* Local-only in-memory backend for unit tests.
* Interface includes: AcquireLock, ReleaseLock, SaveJob, LoadJobs, UpdateStatus.

## Distributed Lock Provider

For distributed execution guarantee.

* Ensures only one node executes a job at a time.
* Uses Redis SET NX with TTL.
* Auto-renews lock while job is running.

## Job Definition

A unified struct describing any scheduled task.

* Name
* Schedule (cron expr or interval or timestamp)
* Retry policy
* Timeout
* Handler (callback)

---

# Step 3: Design Deep Dive

## Backend Integration

Describe how backends (Redis, Postgres, in-memory) implement:

* Job storage (state, next run time)
* Distributed locking (SET NX)
* Heartbeat mechanism
* Failure detection

## Reconciliation

After crashes or restarts:

* Nodes scan job list
* Identify orphan jobs (locked but TTL expired)
* Recalculate next execution
* Clean inconsistent state

## Handling Processing Delays

Jobs may lag due to:

* Node overload
* Backend slow response
* Time drift

Solutions:

* Drift compensation window (± few seconds)
* Job enqueue window scanning
* Adaptive backoff for busy nodes

## Handling Failed Jobs

* Retry N times before marking as permanently failed.
* Exponential backoff.
* Custom retry strategies.
* Dead-letter queue (optional).

## Exactly-Once Delivery

Strategies:

* Idempotent handlers (recommended)
* Use Redis Lua script to guarantee atomic lock + update
* Keep job execution log to prevent duplicates

## Consistency Model

* **At-least-once execution** by default.
* **At-most-once** can be enabled by skipping retries.
* Strong consistency backed by single backend provider.

## Security Considerations

* Protect backend credentials
* Prevent arbitrary handler injection via external API
* Use TLS for Redis/DB connections
* Enforce sandboxing if remote code execution applicable

---

# Golang Interface Definitions (Draft)

```go
// Scheduler manages registration and execution of scheduled jobs.
type Scheduler interface {
    Register(job Job) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Pause(jobName string) error
    Resume(jobName string) error
    Remove(jobName string) error
}

// Job defines a scheduled task.
type Job struct {
    Name        string
    Schedule    Schedule
    RetryPolicy RetryPolicy
    Timeout     time.Duration
    Handler     JobHandler
}

// JobHandler is the user-defined function executed by the scheduler.
type JobHandler func(ctx context.Context) error

// Schedule supports cron, interval, and exact timestamp.
type Schedule interface {
    NextRun(from time.Time) time.Time
}

// BackendProvider handles persistence and distributed locking.
type BackendProvider interface {
    SaveJob(ctx context.Context, job Job) error
    LoadJobs(ctx context.Context) ([]Job, error)
    UpdateStatus(ctx context.Context, jobName string, status JobStatus) error

    AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error)
    ReleaseLock(ctx context.Context, lockKey string) error
}

// JobExecutor processes execution with retry, timeout, panic recovery.
type JobExecutor interface {
    Execute(ctx context.Context, job Job) error
}
```

---