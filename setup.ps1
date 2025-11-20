# Quick Setup Script for Windows (PowerShell)

Write-Host "==================================" -ForegroundColor Cyan
Write-Host "  Golang API Quick Setup Script" -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan
Write-Host ""

# Check if Go is installed
Write-Host "Checking Go installation..." -ForegroundColor Yellow
$goVersion = go version 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Go is not installed. Please install Go 1.25 or higher." -ForegroundColor Red
    exit 1
}
Write-Host "✓ $goVersion" -ForegroundColor Green
Write-Host ""

# Check if Docker is installed
Write-Host "Checking Docker installation..." -ForegroundColor Yellow
$dockerVersion = docker --version 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "Warning: Docker is not installed. Docker-based setup will not be available." -ForegroundColor Yellow
    $useDocker = $false
} else {
    Write-Host "✓ $dockerVersion" -ForegroundColor Green
    $useDocker = $true
}
Write-Host ""

# Download dependencies
Write-Host "Downloading Go dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to download dependencies." -ForegroundColor Red
    exit 1
}
Write-Host "✓ Dependencies downloaded" -ForegroundColor Green
Write-Host ""

# Ask user for setup preference
if ($useDocker) {
    Write-Host "Choose setup method:" -ForegroundColor Cyan
    Write-Host "1. Docker Compose (recommended - includes PostgreSQL)" -ForegroundColor White
    Write-Host "2. Local development (requires PostgreSQL to be installed)" -ForegroundColor White
    $choice = Read-Host "Enter choice (1 or 2)"
    
    if ($choice -eq "1") {
        Write-Host ""
        Write-Host "Starting Docker Compose services..." -ForegroundColor Yellow
        docker-compose -f deployment/docker-compose.yml up -d
        
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Error: Failed to start Docker services." -ForegroundColor Red
            exit 1
        }
        
        Write-Host "✓ Docker services started" -ForegroundColor Green
        Write-Host ""
        
        # Wait for PostgreSQL to be ready
        Write-Host "Waiting for PostgreSQL to be ready..." -ForegroundColor Yellow
        Start-Sleep -Seconds 5
        
        # Build the application
        Write-Host "Building application..." -ForegroundColor Yellow
        go build -o server.exe ./cmd/server
        
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Error: Failed to build application." -ForegroundColor Red
            exit 1
        }
        Write-Host "✓ Application built" -ForegroundColor Green
        Write-Host ""
        
        Write-Host "==================================" -ForegroundColor Green
        Write-Host "  Setup Complete!" -ForegroundColor Green
        Write-Host "==================================" -ForegroundColor Green
        Write-Host ""
        Write-Host "Services are running via Docker Compose." -ForegroundColor White
        Write-Host ""
        Write-Host "Next steps:" -ForegroundColor Cyan
        Write-Host "1. View logs: docker-compose -f deployment/docker-compose.yml logs -f auth" -ForegroundColor White
        Write-Host "2. Access Auth API: http://localhost:8081" -ForegroundColor White
        Write-Host "3. Health check: http://localhost:8081/health" -ForegroundColor White
        Write-Host "4. Stop services: docker-compose -f deployment/docker-compose.yml down" -ForegroundColor White
        
    } else {
        Write-Host ""
        Write-Host "Starting PostgreSQL with Docker..." -ForegroundColor Yellow
        docker-compose -f deployment/docker-compose.yml up -d postgres
        
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Error: Failed to start PostgreSQL." -ForegroundColor Red
            exit 1
        }
        
        Write-Host "✓ PostgreSQL started" -ForegroundColor Green
        Write-Host ""
        
        # Wait for PostgreSQL
        Write-Host "Waiting for PostgreSQL to be ready..." -ForegroundColor Yellow
        Start-Sleep -Seconds 5
        
        # Build the application
        Write-Host "Building application..." -ForegroundColor Yellow
        go build -o server.exe ./cmd/server
        
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Error: Failed to build application." -ForegroundColor Red
            exit 1
        }
        Write-Host "✓ Application built" -ForegroundColor Green
        Write-Host ""
        
        # Run migrations
        Write-Host "Running database migrations..." -ForegroundColor Yellow
        .\server.exe migrate
        
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Error: Failed to run migrations." -ForegroundColor Red
            exit 1
        }
        Write-Host "✓ Migrations completed" -ForegroundColor Green
        Write-Host ""
        
        Write-Host "==================================" -ForegroundColor Green
        Write-Host "  Setup Complete!" -ForegroundColor Green
        Write-Host "==================================" -ForegroundColor Green
        Write-Host ""
        Write-Host "PostgreSQL is running via Docker." -ForegroundColor White
        Write-Host ""
        Write-Host "Next steps:" -ForegroundColor Cyan
        Write-Host "1. Start server: .\server.exe serve" -ForegroundColor White
        Write-Host "2. Or use: go run cmd/server/main.go serve" -ForegroundColor White
        Write-Host "3. Access API: http://localhost:8080" -ForegroundColor White
        Write-Host "4. Stop PostgreSQL: docker-compose -f deployment/docker-compose.yml down" -ForegroundColor White
    }
} else {
    Write-Host "Docker is not available. Please ensure PostgreSQL is running." -ForegroundColor Yellow
    Write-Host ""
    
    # Build the application
    Write-Host "Building application..." -ForegroundColor Yellow
    go build -o server.exe ./cmd/server
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error: Failed to build application." -ForegroundColor Red
        exit 1
    }
    Write-Host "✓ Application built" -ForegroundColor Green
    Write-Host ""
    
    Write-Host "==================================" -ForegroundColor Green
    Write-Host "  Build Complete!" -ForegroundColor Green
    Write-Host "==================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Before running, ensure:" -ForegroundColor Yellow
    Write-Host "1. PostgreSQL is running on localhost:5432" -ForegroundColor White
    Write-Host "2. Database 'myapp' exists" -ForegroundColor White
    Write-Host "3. Update config/config.yaml if needed" -ForegroundColor White
    Write-Host ""
    Write-Host "Then:" -ForegroundColor Cyan
    Write-Host "1. Run migrations: .\server.exe migrate" -ForegroundColor White
    Write-Host "2. Start server: .\server.exe serve" -ForegroundColor White
}

Write-Host ""
Write-Host "For more information, see README.md and API_EXAMPLES.md" -ForegroundColor Cyan
Write-Host ""

