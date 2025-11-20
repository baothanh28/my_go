# Docker Compose Files Guide

Hướng dẫn sử dụng các file docker-compose cho Notification Service.

## 📁 Các file Docker Compose

### 1. `docker-compose.postgres.yml`
Chỉ chứa PostgreSQL service.

**Sử dụng:**
```bash
# Start PostgreSQL
docker-compose -f deployment/docker-compose.postgres.yml up -d

# Stop PostgreSQL
docker-compose -f deployment/docker-compose.postgres.yml down

# Xem logs
docker-compose -f deployment/docker-compose.postgres.yml logs -f
```

### 2. `docker-compose.redis.yml`
Chỉ chứa Redis service.

**Sử dụng:**
```bash
# Start Redis
docker-compose -f deployment/docker-compose.redis.yml up -d

# Stop Redis
docker-compose -f deployment/docker-compose.redis.yml down

# Xem logs
docker-compose -f deployment/docker-compose.redis.yml logs -f
```

### 3. `docker-compose.notification.yml`
Chỉ chứa Notification Service (không có PostgreSQL và Redis).

**Sử dụng:**
```bash
# Chạy cùng với dependencies (recommended)
docker-compose -f deployment/docker-compose.postgres.yml \
               -f deployment/docker-compose.redis.yml \
               -f deployment/docker-compose.notification.yml up -d

# Hoặc chạy standalone (nếu postgres và redis đã chạy)
docker-compose -f deployment/docker-compose.notification.yml up -d
```

### 4. `docker-compose.notification.full.yml`
Chứa tất cả: PostgreSQL, Redis, và Notification Service trong một file.

**Sử dụng:**
```bash
# Start tất cả services
docker-compose -f deployment/docker-compose.notification.full.yml up -d

# Stop tất cả services
docker-compose -f deployment/docker-compose.notification.full.yml down

# Xem logs
docker-compose -f deployment/docker-compose.notification.full.yml logs -f notification
```

## 🚀 Các cách chạy

### Cách 1: Chạy tất cả cùng lúc (Full Stack)

```bash
# Sử dụng file full
docker-compose -f deployment/docker-compose.notification.full.yml up -d

# Hoặc combine các file riêng
docker-compose -f deployment/docker-compose.postgres.yml \
               -f deployment/docker-compose.redis.yml \
               -f deployment/docker-compose.notification.yml up -d
```

### Cách 2: Chạy từng service riêng

```bash
# 1. Start PostgreSQL
docker-compose -f deployment/docker-compose.postgres.yml up -d

# 2. Start Redis
docker-compose -f deployment/docker-compose.redis.yml up -d

# 3. Start Notification Service
docker-compose -f deployment/docker-compose.notification.yml up -d
```

### Cách 3: Chạy với services đã có sẵn

Nếu bạn đã có PostgreSQL và Redis chạy sẵn (không phải từ docker-compose), bạn có thể:

```bash
# 1. Tạo network chung
docker network create notification_network

# 2. Connect existing containers vào network
docker network connect notification_network <postgres_container_name>
docker network connect notification_network <redis_container_name>

# 3. Start notification service
docker-compose -f deployment/docker-compose.notification.yml up -d
```

## 🔧 Quản lý Services

### Xem trạng thái

```bash
# Xem tất cả containers
docker ps | grep notification

# Xem logs của notification service
docker-compose -f deployment/docker-compose.notification.yml logs -f

# Xem logs của PostgreSQL
docker-compose -f deployment/docker-compose.postgres.yml logs -f

# Xem logs của Redis
docker-compose -f deployment/docker-compose.redis.yml logs -f
```

### Stop Services

```bash
# Stop notification service
docker-compose -f deployment/docker-compose.notification.yml down

# Stop PostgreSQL
docker-compose -f deployment/docker-compose.postgres.yml down

# Stop Redis
docker-compose -f deployment/docker-compose.redis.yml down

# Stop tất cả (nếu dùng full file)
docker-compose -f deployment/docker-compose.notification.full.yml down
```

### Restart Services

```bash
# Restart notification service
docker-compose -f deployment/docker-compose.notification.yml restart

# Restart tất cả
docker-compose -f deployment/docker-compose.notification.full.yml restart
```

## 📊 Network Management

Tất cả services sử dụng network `notification_network`. 

### Tạo network thủ công (nếu cần)

```bash
docker network create notification_network
```

### Xem network info

```bash
docker network inspect notification_network
```

### Xóa network (sau khi down tất cả containers)

```bash
docker network rm notification_network
```

## 💡 Best Practices

1. **Development**: Sử dụng `docker-compose.notification.full.yml` để dễ quản lý
2. **Production**: Tách riêng services để dễ scale và maintain
3. **Testing**: Chạy từng service riêng để test độc lập

## 🔍 Troubleshooting

### Service không kết nối được

```bash
# Kiểm tra network
docker network ls | grep notification

# Kiểm tra containers trong network
docker network inspect notification_network

# Kiểm tra logs
docker-compose -f deployment/docker-compose.notification.yml logs
```

### Port conflicts

Nếu port đã được sử dụng, sửa trong file docker-compose:

```yaml
ports:
  - "8083:8082"  # Thay đổi port host
```

### Volume conflicts

Nếu volume đã tồn tại, có thể xóa:

```bash
docker volume rm notification_postgres_data
docker volume rm notification_redis_data
```

