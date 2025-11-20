# Hướng dẫn Build và Chạy Notification Service

## 🚀 Các cách Build và Chạy

### ⭐ Cách 1: Build Image ở Local, Sau đó Run với Docker Compose (Khuyến nghị)

Workflow này cho phép bạn build image một lần ở local, sau đó sử dụng docker-compose để chạy các services khác nhau (migration, API, worker) từ image đã build.

#### Bước 0: Tạo file .env (Tùy chọn nhưng khuyến nghị)

Tạo file `.env` trong thư mục `deployment/` để cấu hình các biến môi trường:

```bash
cd deployment
# Tạo file .env từ template (nếu có)
# Hoặc tạo file .env mới với nội dung sau:
```

**Nội dung file `.env` mẫu:**

```env
# Docker Image
NOTIFICATION_IMAGE=notification-service:latest

# PostgreSQL Configuration
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DBNAME=myapp
POSTGRES_SSLMODE=disable

# Redis Configuration
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_PORT=6379

# Service Ports
NOTIFICATION_API_PORT=8082
NOTIFICATION_WORKER_PORT=8083

# JWT Secret
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production

# Logger
LOGGER_LEVEL=info

# Notification Service Config
NOTIFICATION_WORKER_CONCURRENCY=10
NOTIFICATION_STREAM_NAME=stream:notifications
NOTIFICATION_CONSUMER_GROUP=notifications
NOTIFICATION_DLQ_STREAM_NAME=stream:notifications:dlq
NOTIFICATION_DELAYED_RETRY_ENABLED=true
NOTIFICATION_DELAYED_RETRY_KEY=delayed:notifications
NOTIFICATION_IDEMPOTENCY_TTL_DAYS=7
NOTIFICATION_STREAM_MAX_LEN=100000
```

**Lưu ý:**
- File `.env` sẽ tự động được Docker Compose đọc từ cùng thư mục với file docker-compose
- Nếu không có file `.env`, docker-compose sẽ sử dụng giá trị mặc định
- **Không commit file `.env` vào git** (nên thêm vào `.gitignore`)

#### Bước 1: Build Image ở Local

```bash
# Từ thư mục root của project
docker build -f deployment/Dockerfile.notification -t notification-service:latest .

# Kiểm tra image đã được build
docker images | grep notification-service
```

#### Bước 2: Chạy Migration

```bash
# Option A: Chạy migration với docker-compose (cần postgres đã chạy)
cd deployment
docker-compose -f docker-compose.notification.run.yml --profile migrate --profile infra up notification-migrate

# Option B: Chạy migration thủ công với docker run
docker run --rm --network notification_network \
  -e APP_DATABASE_HOST=postgres \
  -e APP_DATABASE_PORT=5432 \
  -e APP_DATABASE_USER=postgres \
  -e APP_DATABASE_PASSWORD=postgres \
  -e APP_DATABASE_DBNAME=myapp \
  -e APP_DATABASE_SSLMODE=disable \
  notification-service:latest ./notification-service migrate
```

#### Bước 3: Chạy Services với Docker Compose

Sau khi đã build image và chạy migration, bạn có thể chạy các services:

**Chạy API Server (không có worker):**
```bash
cd deployment
docker-compose -f docker-compose.notification.run.yml --profile api --profile infra up -d

# Xem logs
docker-compose -f docker-compose.notification.run.yml logs -f notification-api

# API sẽ chạy trên port 8082
curl http://localhost:8082/health
```

**Chạy Full Service (API + Worker):**
```bash
cd deployment
docker-compose -f docker-compose.notification.run.yml --profile worker --profile infra up -d

# Xem logs
docker-compose -f docker-compose.notification.run.yml logs -f notification-worker

# Service sẽ chạy trên port 8083
curl http://localhost:8083/health
```

**Chạy cả API và Worker cùng lúc:**
```bash
cd deployment
docker-compose -f docker-compose.notification.run.yml --profile api --profile worker --profile infra up -d
```

**Chạy tất cả (Migration + API + Worker + Infrastructure):**
```bash
cd deployment
# Chạy migration trước (một lần)
docker-compose -f docker-compose.notification.run.yml --profile migrate --profile infra up notification-migrate

# Sau đó chạy services
docker-compose -f docker-compose.notification.run.yml --profile api --profile worker --profile infra up -d
```

#### Các Profiles có sẵn trong docker-compose.notification.run.yml:

- `migrate`: Chạy migration service
- `api`: Chạy API server (không có worker)
- `worker`: Chạy full service (API + worker)
- `infra`: Chạy PostgreSQL và Redis

#### Stop Services

```bash
cd deployment
docker-compose -f docker-compose.notification.run.yml --profile api --profile worker down

# Hoặc stop tất cả
docker-compose -f docker-compose.notification.run.yml down
```

### Cách 2: Build và Chạy Tất cả với Docker Compose (Auto Build)

#### Option A: Chạy với tất cả dependencies (PostgreSQL + Redis + Notification)

```bash
# Từ thư mục root của project
cd deployment

# Build và start tất cả services cùng lúc
docker-compose -f docker-compose.notification.full.yml up -d --build

# Xem logs
docker-compose -f docker-compose.notification.full.yml logs -f notification

# Stop services
docker-compose -f docker-compose.notification.full.yml down
```

#### Option B: Build và chạy từng bước với docker run

```bash
# 1. Build image
docker build -f deployment/Dockerfile.notification -t notification-service:latest .

# 2. Chạy migrations (sau khi postgres đã chạy)
docker run --rm --network notification_network \
  -e APP_DATABASE_HOST=postgres \
  -e APP_DATABASE_USER=postgres \
  -e APP_DATABASE_PASSWORD=postgres \
  -e APP_DATABASE_DBNAME=myapp \
  notification-service:latest ./notification-service migrate

# 3. Chạy service
docker run -d --name notification_service \
  --network notification_network \
  -p 8082:8082 \
  -e APP_DATABASE_HOST=postgres \
  -e APP_DATABASE_USER=postgres \
  -e APP_DATABASE_PASSWORD=postgres \
  -e APP_DATABASE_DBNAME=myapp \
  -e APP_REDIS_ADDR=redis:6379 \
  notification-service:latest
```

### Cách 3: Build và Push lên Registry (Cho Production/CI/CD)

#### Push lên Docker Hub

```bash
# 1. Build image với tag
docker build -f deployment/Dockerfile.notification -t your-username/notification-service:latest .
docker build -f deployment/Dockerfile.notification -t your-username/notification-service:v1.0.0 .

# 2. Login vào Docker Hub
docker login

# 3. Push image
docker push your-username/notification-service:latest
docker push your-username/notification-service:v1.0.0
```

#### Push lên AWS ECR

```bash
# 1. Login vào ECR
aws ecr get-login-password --region ap-southeast-1 | docker login --username AWS --password-stdin <account-id>.dkr.ecr.ap-southeast-1.amazonaws.com

# 2. Create repository (nếu chưa có)
aws ecr create-repository --repository-name notification-service --region ap-southeast-1

# 3. Build và tag
docker build -f deployment/Dockerfile.notification -t notification-service:latest .
docker tag notification-service:latest <account-id>.dkr.ecr.ap-southeast-1.amazonaws.com/notification-service:latest

# 4. Push
docker push <account-id>.dkr.ecr.ap-southeast-1.amazonaws.com/notification-service:latest
```

#### Push lên Google Container Registry (GCR)

```bash
# 1. Build và push trực tiếp
gcloud builds submit --tag gcr.io/PROJECT_ID/notification-service:latest --file deployment/Dockerfile.notification

# Hoặc build local rồi push
docker build -f deployment/Dockerfile.notification -t gcr.io/PROJECT_ID/notification-service:latest .
docker push gcr.io/PROJECT_ID/notification-service:latest
```

#### Push lên Azure Container Registry (ACR)

```bash
# 1. Login
az acr login --name <registry-name>

# 2. Build và push
az acr build --registry <registry-name> --image notification-service:latest --file deployment/Dockerfile.notification .
```

### Cách 4: Sử dụng Makefile (Nhanh nhất cho Development - Không dùng Docker)

```bash
# Build notification service binary (không phải Docker)
make build-notification

# Chạy notification service (không phải Docker)
make run-notification

# Chạy với hot reload
make dev-notification
```

## 📋 Workflow Khuyến nghị

### Development (Local) - Build Image trước, Run với Docker Compose

```bash
# 1. Build image một lần
docker build -f deployment/Dockerfile.notification -t notification-service:latest .

# 2. Chạy infrastructure (PostgreSQL + Redis)
cd deployment
docker-compose -f docker-compose.notification.run.yml --profile infra up -d

# 3. Chạy migration (một lần)
docker-compose -f docker-compose.notification.run.yml --profile migrate up notification-migrate

# 4. Chạy API server (development)
docker-compose -f docker-compose.notification.run.yml --profile api up -d

# Hoặc chạy full service (API + Worker)
docker-compose -f docker-compose.notification.run.yml --profile worker up -d

# Xem logs
docker-compose -f docker-compose.notification.run.yml logs -f notification-api
# hoặc
docker-compose -f docker-compose.notification.run.yml logs -f notification-worker
```

**Lưu ý:** Nếu bạn đã có PostgreSQL và Redis chạy sẵn, có thể bỏ `--profile infra`:

```bash
# Chỉ chạy migration
docker-compose -f docker-compose.notification.run.yml --profile migrate up notification-migrate

# Chỉ chạy API
docker-compose -f docker-compose.notification.run.yml --profile api up -d

# Chỉ chạy Worker
docker-compose -f docker-compose.notification.run.yml --profile worker up -d
```

### Development (Local) - Quick Start với Auto Build

```bash
# Cách nhanh nhất: Sử dụng docker-compose với full stack (auto build)
cd deployment
docker-compose -f docker-compose.notification.full.yml up -d --build

# Hoặc chạy binary trực tiếp (nhanh hơn cho development, không dùng Docker)
make dev-notification
```

### Testing/Staging
```bash
# Build image với version tag
docker build -f deployment/Dockerfile.notification -t notification-service:v1.0.0 .

# Push lên registry
docker tag notification-service:v1.0.0 your-registry/notification-service:v1.0.0
docker push your-registry/notification-service:v1.0.0

# Deploy từ registry
docker pull your-registry/notification-service:v1.0.0
docker-compose -f deployment/docker-compose.notification.yml up -d
```

### Production
```bash
# Sử dụng CI/CD pipeline để:
# 1. Build image từ source code
# 2. Run tests
# 3. Push lên registry với version tag
# 4. Deploy lên production environment (ECS, Kubernetes, etc.)
```

## 🔍 Kiểm tra sau khi Build

```bash
# Kiểm tra image đã được build
docker images | grep notification-service

# Kiểm tra container đang chạy
docker ps | grep notification

# Kiểm tra logs
docker logs notification_service

# Kiểm tra health
curl http://localhost:8082/health
```

## ⚙️ Environment Variables

### Sử dụng file .env với Docker Compose

File `docker-compose.notification.run.yml` hỗ trợ đọc các biến môi trường từ file `.env` trong cùng thư mục.

**Các biến môi trường có sẵn:**

#### Docker Image
- `NOTIFICATION_IMAGE`: Tên image (mặc định: `notification-service:latest`)

#### PostgreSQL
- `POSTGRES_HOST`: Host PostgreSQL (mặc định: `postgres`)
- `POSTGRES_PORT`: Port PostgreSQL (mặc định: `5432`)
- `POSTGRES_USER`: Username PostgreSQL (mặc định: `postgres`)
- `POSTGRES_PASSWORD`: Password PostgreSQL (mặc định: `postgres`)
- `POSTGRES_DBNAME`: Database name (mặc định: `myapp`)
- `POSTGRES_SSLMODE`: SSL mode (mặc định: `disable`)
- `POSTGRES_IMAGE`: PostgreSQL image (mặc định: `postgres:15-alpine`)
- `POSTGRES_CONTAINER_NAME`: Container name (mặc định: `notification_postgres`)

#### Redis
- `REDIS_ADDR`: Redis address (mặc định: `redis:6379`)
- `REDIS_PASSWORD`: Redis password (mặc định: empty)
- `REDIS_DB`: Redis database number (mặc định: `0`)
- `REDIS_PORT`: Redis port (mặc định: `6379`)
- `REDIS_IMAGE`: Redis image (mặc định: `redis:7-alpine`)
- `REDIS_CONTAINER_NAME`: Container name (mặc định: `notification_redis`)

#### Service Ports
- `NOTIFICATION_API_PORT`: API service port (mặc định: `8082`)
- `NOTIFICATION_WORKER_PORT`: Worker service port (mặc định: `8083`)

#### JWT & Logger
- `JWT_SECRET`: JWT secret key
- `LOGGER_LEVEL`: Log level (mặc định: `info`)

#### Notification Service
- `NOTIFICATION_WORKER_CONCURRENCY`: Worker concurrency (mặc định: `10`)
- `NOTIFICATION_STREAM_NAME`: Redis stream name (mặc định: `stream:notifications`)
- `NOTIFICATION_CONSUMER_GROUP`: Consumer group name (mặc định: `notifications`)
- `NOTIFICATION_DLQ_STREAM_NAME`: DLQ stream name (mặc định: `stream:notifications:dlq`)
- `NOTIFICATION_DELAYED_RETRY_ENABLED`: Enable delayed retry (mặc định: `true`)
- `NOTIFICATION_DELAYED_RETRY_KEY`: Delayed retry key (mặc định: `delayed:notifications`)
- `NOTIFICATION_IDEMPOTENCY_TTL_DAYS`: Idempotency TTL in days (mặc định: `7`)
- `NOTIFICATION_STREAM_MAX_LEN`: Stream max length (mặc định: `100000`)

### Override Environment Variables khi chạy Docker Run

Khi chạy container trực tiếp với `docker run`, có thể override config bằng environment variables:

```bash
docker run -d --name notification_service \
  -e APP_SERVER_PORT=8082 \
  -e APP_DATABASE_HOST=postgres \
  -e APP_DATABASE_PORT=5432 \
  -e APP_DATABASE_USER=postgres \
  -e APP_DATABASE_PASSWORD=postgres \
  -e APP_DATABASE_DBNAME=myapp \
  -e APP_REDIS_ADDR=redis:6379 \
  -e APP_NOTIFICATION_WORKER_CONCURRENCY=10 \
  notification-service:latest
```

### Sử dụng file .env khác

Nếu muốn sử dụng file `.env` ở vị trí khác:

```bash
# Sử dụng --env-file
docker-compose --env-file /path/to/.env -f docker-compose.notification.run.yml up -d
```

## 🐛 Troubleshooting

### Build fails
```bash
# Kiểm tra Dockerfile path
# Đảm bảo đang ở thư mục root khi build
docker build -f deployment/Dockerfile.notification -t notification-service:latest .
```

### Container không start
```bash
# Xem logs
docker logs notification_service

# Kiểm tra network
docker network ls
docker network inspect notification_network
```

### Migration fails
```bash
# Chạy migration thủ công với docker-compose
cd deployment
docker-compose -f docker-compose.notification.run.yml --profile migrate up notification-migrate

# Hoặc chạy migration trong container đang chạy
docker exec -it notification_api ./notification-service migrate
docker exec -it notification_worker ./notification-service migrate
```

## 📝 Notes

### File Docker Compose

- **`docker-compose.notification.run.yml`**: Chạy từ image đã build (không build). Sử dụng profiles để chọn services cần chạy.
- **`docker-compose.notification.full.yml`**: Build và chạy tất cả (PostgreSQL + Redis + Notification) cùng lúc.
- **`docker-compose.notification.yml`**: Chỉ chạy notification service (cần postgres và redis sẵn có).

### Workflow Khuyến nghị

- **Development**: 
  - Build image một lần: `docker build -f deployment/Dockerfile.notification -t notification-service:latest .`
  - Sử dụng `docker-compose.notification.run.yml` với profiles để chạy các services cần thiết
  - Hoặc dùng `docker-compose.notification.full.yml` để auto build và chạy tất cả
  
- **Production**: 
  - Build và push lên registry, sau đó pull và deploy
  - Sử dụng `docker-compose.notification.run.yml` với image từ registry
  
- **CI/CD**: 
  - Tự động hóa build và push trong pipeline
  - Deploy sử dụng image đã push lên registry

### Image và Services

- **Image size**: ~20-30MB (sau khi build multi-stage)
- **Ports**:
  - API service: 8082
  - Worker service: 8083
- **Commands**:
  - `migrate`: Chạy database migrations
  - `api`: Chạy API server only (không có worker)
  - `serve`: Chạy full service (API + worker)

