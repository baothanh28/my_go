# Golang Microservices API

A production-ready microservices architecture built with Golang, featuring clean architecture, dependency injection with Uber FX, and independent service deployment.

## 🏗️ Architecture

This project implements a **pure microservices architecture** where each service is completely independent and can be deployed, scaled, and maintained separately.

```
firebase_selfplan/
├── internal/
│   ├── service/
│   │   ├── auth/                # Authentication Service
│   │   │   ├── cmd/main.go     # Standalone executable
│   │   │   ├── app.go          # Service dependencies
│   │   │   ├── config/config.yaml  # Service config (Port 8081)
│   │   │   └── *.go            # Auth logic
│   │   │
│   │   └── product/            # Product Service
│   │       ├── cmd/main.go     # Standalone executable
│   │       ├── app.go          # Service dependencies
│   │       ├── config/config.yaml  # Service config (Port 8082)
│   │       └── *.go            # Product logic
│   │
│   └── pkg/                     # Shared Infrastructure
│       ├── config/              # Configuration management
│       ├── database/            # Database + BaseRepository
│       ├── logger/              # Structured logging
│       └── server/              # HTTP server
```

## ✨ Features

- ✅ **Independent Microservices** - Each service runs standalone
- ✅ **Clean Architecture** - Clear separation of concerns
- ✅ **Dependency Injection** - Uber FX for lifecycle management
- ✅ **Generic Repository Pattern** - Type-safe CRUD operations
- ✅ **JWT Authentication** - Secure token-based auth
- ✅ **Shared Infrastructure** - Reusable pkg modules
- ✅ **Docker Support** - Containerized deployment
- ✅ **Database Migrations** - Per-service migrations

## 🚀 Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL 15+
- Docker (optional)

### 1. Start PostgreSQL

```bash
docker-compose -f deployment/docker-compose.yml up -d postgres
```

### 2. Build Services

```bash
# Build all services
make build-all

# Or build individually
make build-auth
make build-product
```

Services will be built in their respective directories:
- `internal/service/auth/auth-service.exe`
- `internal/service/product/product-service.exe`

### 3. Run Migrations

```bash
# Migrate all services
make migrate-all

# Or migrate individually
make migrate-auth
make migrate-product
```

### 4. Start Services

```bash
# Terminal 1: Auth Service
cd internal/service/auth
./auth-service.exe serve
# Runs on http://localhost:8081

# Terminal 2: Product Service
cd internal/service/product
./product-service.exe serve
# Runs on http://localhost:8082
```

## 📡 Services

### Auth Service (Port 8081)

**Endpoints:**
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login and get JWT token
- `GET /api/v1/auth/profile` - Get user profile (protected)
- `GET /health` - Health check

**Config:** `internal/service/auth/config/config.yaml`

### Product Service (Port 8082)

**Endpoints:**
- `POST /api/v1/products` - Create product (protected)
- `GET /api/v1/products` - List products (protected)
- `GET /api/v1/products/:id` - Get product (protected)
- `PUT /api/v1/products/:id` - Update product (protected)
- `DELETE /api/v1/products/:id` - Delete product (protected)
- `GET /api/v1/products/search` - Search products (protected)
- `GET /health` - Health check

**Config:** `internal/service/product/config/config.yaml`

**Note:** Product service requires JWT token from Auth service.

## 🔧 Development

### Using Make

```bash
make build-all          # Build all services
make run-auth           # Run auth service
make run-product        # Run product service
make migrate-all        # Run all migrations
make test              # Run tests
```

### Using Go Run

```bash
# Auth Service
cd internal/service/auth
go run ./cmd/main.go serve

# Product Service
cd internal/service/product
go run ./cmd/main.go serve
```

## 🧪 Testing the API

See `API_EXAMPLES.md` for complete examples.

### Quick Test

```bash
# 1. Register user (Auth Service)
curl -X POST http://localhost:8081/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"pass123","name":"Test"}'

# 2. Login (Auth Service)
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"pass123"}'
# Save the token!

# 3. Create product (Product Service)
curl -X POST http://localhost:8082/api/v1/products \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Laptop","price":999.99,"stock":10,"sku":"LAP-001"}'
```

## ⚙️ Configuration

Each service has its own configuration file:

- **Auth Service:** `internal/service/auth/config/config.yaml`
- **Product Service:** `internal/service/product/config/config.yaml`

### Key Settings

```yaml
server:
  port: 8081  # Unique per service

database:
  host: "localhost"
  dbname: "myapp"  # Can be separate per service

jwt:
  secret: "your-secret"  # MUST be identical across services!
```

### Environment Variables

Override config using `APP_` prefix:

```bash
export APP_SERVER_PORT=8081
export APP_DATABASE_HOST=localhost
export APP_JWT_SECRET=your-secret
```

## 🐳 Docker Deployment

```bash
# Build images
docker build -t auth-service:1.0 -f deployment/Dockerfile.auth .
docker build -t product-service:1.0 -f deployment/Dockerfile .

# Run with Docker Compose
docker-compose -f deployment/docker-compose.yml up -d
```

## 📁 Project Structure

```
internal/service/
├── auth/                       # Auth Microservice
│   ├── cmd/main.go            # Entry point
│   ├── app.go                 # FX dependencies
│   ├── config/config.yaml     # Config
│   ├── model.go               # Data models
│   ├── service.go             # Business logic
│   ├── handler.go             # HTTP handlers
│   ├── router.go              # Routes
│   ├── migration.go           # DB migrations
│   └── jwt_middleware.go      # JWT validation
│
└── product/                    # Product Microservice
    ├── cmd/main.go            # Entry point
    ├── app.go                 # FX dependencies
    ├── config/config.yaml     # Config
    ├── model.go               # Data models
    ├── repository.go          # Data access
    ├── service.go             # Business logic
    ├── handler.go             # HTTP handlers
    ├── router.go              # Routes
    └── migration.go           # DB migrations
```

## 🔑 Key Concepts

### Independent Services

Each service:
- Has its own `main.go` executable
- Manages its own dependencies via `app.go`
- Has service-specific configuration
- Can be deployed independently
- Shares infrastructure via `internal/pkg`

### Shared Infrastructure

All services use common infrastructure from `internal/pkg`:
- **Config** - Configuration management with Viper
- **Database** - PostgreSQL with GORM + BaseRepository
- **Logger** - Structured logging with Zap
- **Server** - Echo HTTP server

### Generic Repository

`BaseRepository[T]` provides type-safe CRUD:

```go
type ProductRepository struct {
    *database.BaseRepository[Product]
}

// Automatic methods available:
repo.Insert(product)
repo.GetByID(id)
repo.GetAll(limit, offset)
repo.Update(id, product)
repo.Delete(id)
```

## 🔐 Security

- JWT tokens for authentication
- Bcrypt password hashing
- CORS configured
- Request validation
- SQL injection protection (GORM)

**Important:** JWT secret must be identical across all services!

## 📊 Service Communication

Services are independent but can communicate:

1. **Auth Service** generates JWT tokens
2. **Product Service** validates JWT tokens
3. Both share the same JWT secret
4. Services can share database or have separate DBs

## 🆕 Adding a New Service

See `CONTRIBUTING.md` for detailed guide.

Quick steps:
1. Create `internal/service/newservice/` directory
2. Add `cmd/main.go`, `app.go`, `config/config.yaml`
3. Implement service logic
4. Build and run independently

## 📖 Documentation

- `QUICKSTART.md` - Quick start guide
- `MICROSERVICES.md` - Architecture details
- `API_EXAMPLES.md` - Complete API examples
- `DEPLOYMENT.md` - Production deployment
- `CONTRIBUTING.md` - Development guide

## 🛠️ Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25+ |
| HTTP Framework | Echo v4 |
| DI Container | Uber FX |
| ORM | GORM |
| Database | PostgreSQL |
| Logger | Zap |
| CLI | Cobra |
| Config | Viper |
| Auth | JWT + bcrypt |

## 🔍 Troubleshooting

**Services can't connect to database**
```bash
docker ps  # Check PostgreSQL is running
docker-compose -f deployment/docker-compose.yml up -d postgres
```

**JWT validation fails**
- Ensure JWT secret is identical in all service configs

**Port already in use**
```bash
netstat -ano | findstr :8081  # Find process
# Change port in service config
```

## 📝 License

MIT License

## 🤝 Contributing

See `CONTRIBUTING.md` for development guidelines.

---

**Pure Microservices Architecture** - Each service is independent and production-ready! 🚀
