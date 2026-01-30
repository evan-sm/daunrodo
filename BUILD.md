# 🔨 Build Instructions for daunrodo

This document provides comprehensive instructions for building, testing, and developing the daunrodo project.

## 📋 Prerequisites

### Required Software
- **Go 1.25+** - Check with `go version`
- **Docker** (optional, for containerized builds)
- **Docker Compose** (optional, for local development)
- **golangci-lint** (for linting) - Install via: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **Task** (optional, for task runner) - Install via: `go install github.com/go-task/task/v3/cmd/task@latest`

### System Dependencies (for local development)
- **ffmpeg** - Required for video processing
  - macOS: `brew install ffmpeg`
  - Debian/Ubuntu: `apt-get install ffmpeg`
  - The Docker image installs this automatically

- **Python 3** + **pip** (for gallery-dl support)
  - macOS: Usually pre-installed, or `brew install python3`
  - Debian/Ubuntu: `apt-get install python3 python3-pip`
  - Install gallery-dl: `pip3 install gallery-dl`

## 🚀 Quick Start

### Local Development (without Docker)

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd daunrodo
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Build the application**
   ```bash
   go build -ldflags "-s -w" -o app
   ```

4. **Run the application**
   ```bash
   ./app
   ```

### Docker Development

1. **Build the Docker image**
   ```bash
   docker build -t daunrodo .
   ```

2. **Run with Docker Compose**
   ```bash
   docker-compose up -d
   docker-compose logs -f
   ```

## 🧪 Testing

### Run All Tests
```bash
go test -v -cover ./...
```

### Run Tests for Specific Package
```bash
go test -v ./internal/downloader
go test -v ./internal/service
go test -v ./internal/storage
```

### Run Tests with Coverage Report
```bash
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run Tests in Watch Mode (requires additional tooling)
```bash
# Install air for hot reload
go install github.com/cosmtrek/air@latest
air
```

## 🔍 Linting

### Run golangci-lint
```bash
golangci-lint run
```

### Run with Taskfile
```bash
task lint
```

### Fix Auto-fixable Issues
```bash
golangci-lint run --fix
```

## 🏗️ Building

### Development Build
```bash
go build -o app
```

### Production Build (optimized)
```bash
go build -ldflags "-s -w" -o app
```

### Cross-platform Build
```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o app-linux-amd64

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o app-linux-arm64

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o app-darwin-amd64

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o app-darwin-arm64
```

## 🐳 Docker Build

### Build Docker Image
```bash
docker build -t daunrodo:latest .
```

### Build for Multiple Platforms
```bash
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64 -t daunrodo:latest .
```

### Build with Custom Build Args
```bash
docker build \
  --build-arg TARGETOS=linux \
  --build-arg TARGETARCH=amd64 \
  -t daunrodo:latest .
```

## ⚙️ Configuration

### Environment Variables

Create a `.env` file or set environment variables:

```bash
# Application
DAUNRODO_APP_LOG_LEVEL=info                    # Log level: debug, info, warn, error
DAUNRODO_APP_JOB_WORKERS=2                     # Number of worker goroutines
DAUNRODO_APP_JOB_TIMEOUT=5m                    # Job processing timeout
DAUNRODO_APP_JOB_QUEUE_SIZE=100               # Job queue size

# HTTP Server
DAUNRODO_HTTP_PORT=:8080                       # HTTP server port
DAUNRODO_HTTP_HANDLER_TIMEOUT=20s              # HTTP handler timeout
DAUNRODO_HTTP_DOWNLOAD_TIMEOUT=30m             # Download timeout
DAUNRODO_HTTP_SHUTDOWN_TIMEOUT=10s             # Graceful shutdown timeout

# Directories
DAUNRODO_DIR_DOWNLOAD=./data/downloads         # Download directory
DAUNRODO_DIR_CACHE=./data/cache                # Cache directory (yt-dlp, gallery-dl)
DAUNRODO_DIR_COOKIE_FILE=./data/cookies/cookies.txt  # Cookie file path (optional)

# Storage
DAUNRODO_APP_STORAGE_TTL=168h                  # Time-to-live for stored jobs (7 days)
DAUNRODO_APP_STORAGE_CLEANUP_INTERVAL=1h       # Cleanup interval

# Binary Dependency Manager
DAUNRODO_DEPMANAGER_BINS_DIR=./bin             # Directory for downloaded binaries
DAUNRODO_DEPMANAGER_LAZY_DOWNLOAD=false        # Lazy-load binaries on first use
DAUNRODO_DEPMANAGER_UPDATE_INTERVAL=24h        # Check for binary updates interval

# Proxy Configuration
DAUNRODO_PROXY_LIST=socks5h://proxy1:1080,socks5h://proxy2:1080  # Comma-separated proxy list
DAUNRODO_PROXY_MAX_FAILURES=3                  # Failures before proxy is marked unavailable
DAUNRODO_PROXY_FAILURE_BACKOFF=5m              # Backoff time after failures
DAUNRODO_PROXY_HEALTH_CHECK_INTERVAL=1m        # Health check interval (0 to disable)
```

### Example `.env` File
```env
DAUNRODO_APP_LOG_LEVEL=debug
DAUNRODO_HTTP_PORT=:8080
DAUNRODO_DIR_DOWNLOAD=./data/downloads
DAUNRODO_DIR_CACHE=./data/cache
DAUNRODO_DIR_COOKIE_FILE=./data/cookies/cookies.txt

# Proxy rotation (optional)
DAUNRODO_PROXY_LIST=socks5h://myproxy:1080
DAUNRODO_PROXY_HEALTH_CHECK_INTERVAL=5m
```

## 📁 Project Structure

```
daunrodo/
├── internal/
│   ├── config/          # Configuration management
│   ├── consts/          # Application constants
│   ├── depmanager/      # Binary dependency management (yt-dlp, ffmpeg, gallery-dl)
│   ├── downloader/      # Downloader implementations (yt-dlp, gallery-dl)
│   ├── entity/          # Domain entities (Job, Publication, etc.)
│   ├── errs/            # Error definitions
│   ├── infrastructure/  # Infrastructure layer (HTTP, middleware)
│   ├── observability/   # Prometheus metrics
│   ├── proxymgr/        # Proxy rotation and health checking
│   ├── service/         # Business logic
│   └── storage/         # Storage layer (in-memory with TTL)
│   ├── entity/          # Domain entities (Job, Publication, etc.)
│   ├── errs/            # Error definitions
│   ├── infrastructure/  # Infrastructure layer (HTTP, middleware)
│   ├── service/         # Business logic
│   └── storage/         # Storage layer (in-memory with TTL)
├── pkg/                 # Shared packages
│   ├── calc/            # Calculation utilities
│   ├── gen/             # Generation utilities (UUIDs)
│   ├── http/            # HTTP server utilities
│   ├── logger/          # Logging utilities
│   ├── maths/           # Math utilities
│   ├── ptr/             # Pointer utilities
│   └── urls/             # URL utilities
├── main.go              # Application entry point
├── Dockerfile           # Docker build instructions
├── compose.yml          # Docker Compose configuration
├── go.mod               # Go module dependencies
├── Taskfile.yml         # Task runner configuration
└── .golangci.yml        # golangci-lint configuration
```

## 🔧 Development Workflow

### 1. Setting Up Development Environment

```bash
# Clone and navigate
git clone <repository-url>
cd daunrodo

# Install Go dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/go-task/task/v3/cmd/task@latest

# Install system dependencies (macOS example)
brew install ffmpeg python3
pip3 install gallery-dl
```

### 2. Making Changes

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**
   - Follow Go conventions and project structure
   - Add tests for new functionality
   - Update documentation as needed

3. **Run tests and linting**
   ```bash
   go test -v ./...
   golangci-lint run
   ```

4. **Build and test locally**
   ```bash
   go build -o app
   ./app
   ```

### 3. Adding a New Downloader

To add a new downloader (like gallery-dl):

1. **Create the downloader implementation**
   - File: `internal/downloader/gallerydl.go`
   - Implement the `Downloader` interface:
     ```go
     type Downloader interface {
         Process(ctx context.Context, job *entity.Job, storer storage.Storer) error
     }
     ```

2. **Add downloader constant**
   - File: `internal/consts/consts.go`
   - Add: `DownloaderGalleryDL = "gallery-dl"`

3. **Create factory function**
   ```go
   func NewGalleryDL(log *slog.Logger, cfg *config.Config) Downloader {
       return &GalleryDL{...}
   }
   ```

4. **Update main.go** (if needed)
   - Wire up the new downloader or use a hybrid approach

5. **Add tests**
   - Create `internal/downloader/gallerydl_test.go`
   - Test JSON parsing, error handling, etc.

6. **Update Dockerfile** (if system dependencies needed)
   - Add installation steps for required tools

### 4. Testing Downloaders

The project includes test data for downloaders:
- `internal/downloader/testdata/` - Contains sample yt-dlp output for testing

To test a new downloader:
1. Capture sample output from the tool
2. Save it in `testdata/`
3. Write tests that parse this output

## 🚢 CI/CD

The project uses GitHub Actions for CI/CD. The workflow includes:

1. **Lint** - Runs golangci-lint
2. **Test** - Runs all tests with coverage
3. **Build** - Builds the Go binary
4. **Docker** - Builds and pushes Docker images for multiple platforms

### Local CI Simulation

```bash
# Run lint
golangci-lint run

# Run tests
go test -v -cover ./...

# Build
go build -ldflags "-s -w" -o app

# Docker build
docker build -t daunrodo:test .
```

## 🐛 Debugging

### Enable Debug Logging
```bash
export DAUNRODO_APP_LOG_LEVEL=debug
./app
```

### Run with Verbose Output
```bash
go run -race main.go
```

### Debug Docker Container
```bash
docker run -it --rm \
  -v $(pwd)/data:/app/data \
  -e DAUNRODO_APP_LOG_LEVEL=debug \
  daunrodo:latest
```

## 📦 Dependencies

### Go Dependencies
- `github.com/caarlos0/env/v11` - Environment variable parsing
- `github.com/google/uuid` - UUID generation
- `github.com/prometheus/client_golang` - Prometheus metrics

### System Dependencies (Auto-managed)
The following binaries are automatically downloaded, verified (SHA256), and managed:
- **yt-dlp** - Video/audio downloader
- **ffmpeg** - Video processing
- **ffprobe** - Media info extraction
- **gallery-dl** - Image gallery downloader (TikTok slideshows, etc.)

## 🔄 Updating Dependencies

### Update Go Dependencies
```bash
go get -u ./...
go mod tidy
```

### Update Specific Dependency
```bash
go get -u github.com/prometheus/client_golang@latest
go mod tidy
```

### Binary Dependencies
Binary dependencies (yt-dlp, ffmpeg, gallery-dl) are automatically updated based on `DAUNRODO_DEPMANAGER_UPDATE_INTERVAL`. You can also manually trigger updates by restarting the service.

## 📝 Code Style

- Follow standard Go conventions
- Use `gofmt` or `goimports` for formatting
- Follow the linting rules in `.golangci.yml`
- Use structured logging with `log/slog`
- Add comments for exported functions and types

## 🎯 Common Tasks

### Run the Application
```bash
go run main.go
```

### Build and Run
```bash
go build -o app && ./app
```

### Run Tests with Coverage
```bash
go test -v -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### Format Code
```bash
go fmt ./...
```

### Check for Issues
```bash
go vet ./...
golangci-lint run
```

## 🆘 Troubleshooting

### Issue: `yt-dlp` not found
- The `go-ytdlp` library auto-installs yt-dlp on first run
- Check that the cache directory is writable
- On first run, it may take time to download binaries

### Issue: `gallery-dl` not found
- Ensure Python 3 and pip are installed
- Install: `pip3 install gallery-dl`
- In Docker, it's installed automatically

### Issue: Permission denied
- Ensure download and cache directories are writable
- Check file permissions: `chmod -R 755 ./data`

### Issue: Port already in use
- Change the port: `export DAUNRODO_HTTP_PORT=:8081`
- Or kill the process using the port

## 📚 Additional Resources

- [Go Documentation](https://go.dev/doc/)
- [yt-dlp Documentation](https://github.com/yt-dlp/yt-dlp)
- [gallery-dl Documentation](https://github.com/mikf/gallery-dl)
- [Docker Documentation](https://docs.docker.com/)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting
5. Submit a pull request

---

For questions or issues, please open an issue on GitHub.

