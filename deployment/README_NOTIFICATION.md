# Notification Service Deployment Guide

Hướng dẫn deploy Notification Service sử dụng Docker.

## 🚀 Quick Start

### Option 1: Deploy với tất cả services (Auth + Notification)

```bash
# Build và start tất cả services
docker-compose -f deployment/docker-compose.yml up -d

# Xem logs
docker-compose -f deployment/docker-compose.yml logs -f notification

# Stop services
docker-compose -f deployment/docker-compose.yml down
```

### Option 2: Deploy Notification Service với tất cả dependencies (Full Stack)

```bash
# Build và start notification service với dependencies
docker-compose -f deployment/docker-compose.notification.full.yml up -d

# Xem logs
docker-compose -f deployment/docker-compose.notification.full.yml logs -f notification

# Stop services
docker-compose -f deployment/docker-compose.notification.full.yml down
```

### Option 3: Deploy từng service riêng

```bash
# 1. Start PostgreSQL
docker-compose -f deployment/docker-compose.postgres.yml up -d

# 2. Start Redis
docker-compose -f deployment/docker-compose.redis.yml up -d

# 3. Start Notification Service
docker-compose -f deployment/docker-compose.notification.yml up -d

# Hoặc chạy tất cả cùng lúc
docker-compose -f deployment/docker-compose.postgres.yml \
               -f deployment/docker-compose.redis.yml \
               -f deployment/docker-compose.notification.yml up -d
```

Xem thêm chi tiết trong [README_DOCKER_COMPOSE.md](./README_DOCKER_COMPOSE.md)

## 📋 Prerequisites

- Docker 20.10+
- Docker Compose 2.0+

## 🔧 Build và Deploy

### 1. Build Docker Image

```bash
# Build notification service image
docker build -f deployment/Dockerfile.notification -t notification-service:latest ..

# Hoặc sử dụng docker-compose
docker-compose -f deployment/docker-compose.notification.yml build
```

### 2. Run Migrations

Trước khi start service, cần chạy migrations:

```bash
# Option 1: Chạy migration trong container
docker-compose -f deployment/docker-compose.notification.yml run --rm notification \
  ./notification-service migrate

# Option 2: Chạy migration từ host
docker exec -it notification_service ./notification-service migrate
```

### 3. Start Services

```bash
# Start với docker-compose
docker-compose -f deployment/docker-compose.notification.yml up -d

# Hoặc start từng service
docker-compose -f deployment/docker-compose.notification.yml up -d postgres redis
docker-compose -f deployment/docker-compose.notification.yml up -d notification
```

## 🔍 Monitoring và Debugging

### Xem Logs

```bash
# Xem logs của notification service
docker-compose -f deployment/docker-compose.notification.yml logs -f notification

# Xem logs của tất cả services
docker-compose -f deployment/docker-compose.notification.yml logs -f

# Xem logs của PostgreSQL
docker-compose -f deployment/docker-compose.notification.yml logs -f postgres

# Xem logs của Redis
docker-compose -f deployment/docker-compose.notification.yml logs -f redis
```

### Health Check

```bash
# Check service health
curl http://localhost:8082/health

# Check container health
docker ps | grep notification
```

### Kiểm tra Redis Stream

```bash
# Connect vào Redis container
docker exec -it notification_redis redis-cli

# Xem stream info
XINFO STREAM stream:notifications

# Xem consumer groups
XINFO GROUPS stream:notifications

# Xem pending messages
XPENDING stream:notifications notifications
```

### Kiểm tra Database

```bash
# Connect vào PostgreSQL
docker exec -it notification_postgres psql -U postgres -d myapp

# Kiểm tra tables
\dt

# Kiểm tra notifications
SELECT * FROM notification LIMIT 10;
SELECT * FROM notification_target LIMIT 10;
SELECT * FROM notification_delivery LIMIT 10;
```

## ⚙️ Configuration

### Environment Variables

Có thể override config qua environment variables trong `docker-compose.notification.yml`:

```yaml
environment:
  APP_SERVER_PORT: 8082
  APP_DATABASE_HOST: postgres
  APP_REDIS_ADDR: redis:6379
  APP_NOTIFICATION_WORKER_CONCURRENCY: 10
  APP_NOTIFICATION_STREAM_NAME: stream:notifications
  # ... more configs
```

### Volume Mounts

Để override config file, có thể mount volume:

```yaml
volumes:
  - ./config:/app/config:ro
```

## 🔄 Updates và Redeploy

### Update Service

```bash
# 1. Rebuild image
docker-compose -f deployment/docker-compose.notification.yml build notification

# 2. Stop và remove old container
docker-compose -f deployment/docker-compose.notification.yml stop notification
docker-compose -f deployment/docker-compose.notification.yml rm -f notification

# 3. Start new container
docker-compose -f deployment/docker-compose.notification.yml up -d notification
```

### Zero-downtime Update

```bash
# Scale up new version
docker-compose -f deployment/docker-compose.notification.yml up -d --scale notification=2

# Wait for new instance to be healthy
# Then scale down old version
docker-compose -f deployment/docker-compose.notification.yml up -d --scale notification=1
```

## 🧹 Cleanup

### Stop và Remove Containers

```bash
# Stop services
docker-compose -f deployment/docker-compose.notification.yml stop

# Stop và remove containers
docker-compose -f deployment/docker-compose.notification.yml down

# Remove containers và volumes (⚠️ sẽ xóa data)
docker-compose -f deployment/docker-compose.notification.yml down -v
```

### Remove Images

```bash
# Remove notification service image
docker rmi notification-service:latest

# Remove all unused images
docker image prune -a
```

## 📊 Production Considerations

### 1. Security

- Thay đổi default passwords
- Sử dụng secrets management (Docker Secrets, Vault, etc.)
- Enable TLS cho PostgreSQL và Redis
- Sử dụng non-root user (đã có trong Dockerfile)

### 2. Scaling

```yaml
# Scale notification workers
docker-compose -f deployment/docker-compose.notification.yml up -d --scale notification=3
```

### 3. Persistence

- PostgreSQL data: `notification_postgres_data` volume
- Redis data: `notification_redis_data` volume (AOF enabled)

### 4. Monitoring

- Health checks đã được cấu hình
- Logs có thể forward đến logging system (ELK, Loki, etc.)
- Metrics có thể expose qua endpoint (nếu implement)

### 5. Resource Limits

Thêm resource limits trong docker-compose:

```yaml
notification:
  deploy:
    resources:
      limits:
        cpus: '2'
        memory: 2G
      reservations:
        cpus: '1'
        memory: 1G
```

## 🐛 Troubleshooting

### Service không start

```bash
# Check logs
docker-compose -f deployment/docker-compose.notification.yml logs notification

# Check container status
docker ps -a | grep notification

# Check health
docker inspect notification_service | grep Health -A 10
```

### Database connection issues

```bash
# Test PostgreSQL connection
docker exec -it notification_postgres psql -U postgres -d myapp -c "SELECT 1;"

# Check network
docker network inspect notification_network
```

### Redis connection issues

```bash
# Test Redis connection
docker exec -it notification_redis redis-cli ping

# Check Redis logs
docker-compose -f deployment/docker-compose.notification.yml logs redis
```

### Migration issues

```bash
# Run migration manually
docker exec -it notification_service ./notification-service migrate

# Check migration status
docker exec -it notification_postgres psql -U postgres -d myapp -c "\dt"
```

## 📝 Notes

- Service sử dụng port 8082
- PostgreSQL sử dụng port 5432
- Redis sử dụng port 6379
- Health check endpoint: `/health`
- Service tự động restart nếu crash (restart: unless-stopped)

