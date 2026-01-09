.PHONY: help init build run run-dev migrate migrate-create worker test test-api test-db \
        version clean proto proto-all proto-clean proto-help lint vet fmt docker-build \
        docker-run docker-compose-up docker-compose-down install-deps health-check \
        deps generate-docs bench load-test security-check dev

# Конфигурация
APP_NAME = api-gateway
BIN_DIR = bin
BUILD_INFO = $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT_HASH = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
PROTOC_IMAGE = local/protoc-go:latest
PROTO_ROOT = pkg/proto
GEN_DIR = pkg/gen

# Главная цель по умолчанию
.DEFAULT_GOAL := help

## 📚 Помощь
help:
	@echo "🚀 API Gateway - Makefile"
	@echo ""
	@echo "Доступные команды:"
	@echo ""
	@echo "📦 Proto файлы:"
	@echo "  make proto              - Build image and generate all proto files"
	@echo "  make proto-generate     - Generate code for internal use"
	@echo "  make proto-pkg          - Generate code for external services"
	@echo "  make proto-pkg-simple   - Simple version for Windows"
	@echo "  make proto-pkg-script   - Generate via script (recommended)"
	@echo "  make proto-clean        - Clean generated files"
	@echo ""
	@echo "🏗️  Сборка и запуск:"
	@echo "  make build              - Сборка бинарника"
	@echo "  make run                - Сборка и запуск сервера"
	@echo "  make run-dev            - Запуск в режиме разработки"
	@echo "  make dev                - Запуск с hot reload (требуется air)"
	@echo "  make clean              - Очистка сборки"
	@echo ""
	@echo "🔧 Управление:"
	@echo "  make migrate            - Выполнить миграции БД"
	@echo "  make migrate-create     - Создать новую миграцию"
	@echo "  make worker             - Запустить фоновых воркеров"
	@echo "  make health-check       - Проверить здоровье сервиса"
	@echo ""
	@echo "🧪 Тестирование:"
	@echo "  make test               - Запуск всех тестов"
	@echo "  make test-api           - Тестирование API"
	@echo "  make test-db            - Тестирование БД"
	@echo "  make bench              - Бенчмарки"
	@echo "  make load-test          - Нагрузочное тестирование"
	@echo "  make lint               - Линтинг кода"
	@echo "  make vet                - Проверка кода"
	@echo "  make fmt                - Форматирование кода"
	@echo "  make security-check     - Проверка безопасности"
	@echo ""

## 📦 Proto файлы
proto: proto-build proto-generate

proto-build:
	@echo "📦 Building protoc-go image..."
	docker build -t $(PROTOC_IMAGE) -f infra/protoc-go.Dockerfile .
	@echo "✅ Docker image built"

proto-generate:
	@echo "🔧 Generating Go code from shared proto files..."
	docker run --rm \
		-v "$(CURDIR):/workspace" \
		-v "$(CURDIR)/vendor:/workspace/vendor:ro" \
		$(PROTOC_IMAGE)
	@echo "✅ Proto files generated"

proto-clean:
	@echo "🧹 Cleaning generated files..."
	@if exist "pkg\gen" rmdir /s /q "pkg\gen" 2>nul || rm -rf pkg/gen
	@echo "✅ Clean complete"

## 🏗️  Сборка и запуск
build:
	@echo "🔨 Building $(APP_NAME)..."
	mkdir -p $(BIN_DIR)
	go build -ldflags="-X 'main.Version=$(BUILD_INFO)' \
		-X 'main.Commit=$(COMMIT_HASH)' \
		-X 'main.BuildDate=$(BUILD_DATE)'" \
		-o $(BIN_DIR)/$(APP_NAME) ./cmd/api-gateway
	@echo "✅ Build complete: $(BIN_DIR)/$(APP_NAME)"

run: build
	@echo "🚀 Starting API Gateway server..."
	@echo "Server will be available at: http://localhost:8080"
	@echo "Health check: http://localhost:8080/health"
	@echo ""
	@cd $(BIN_DIR) && ./$(APP_NAME) server --debug

run-dev:
	@echo "🚀 Starting in development mode..."
	@echo "For hot reload use: make dev"
	DEBUG=true go run ./cmd/api-gateway server

dev:
	@echo "🔥 Starting with hot reload..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "⚠ air is not installed. Install: go install github.com/cosmtrek/air@latest"; \
		echo "Running without hot reload..."; \
		make run-dev; \
	fi

## 🔧 Управление
migrate: build
	@echo "🔄 Running migrations..."
	@cd $(BIN_DIR) && ./$(APP_NAME) migrate up

migrate-create: build
	@echo "📝 Creating migration..."
	@read -p "Enter migration name: " name; \
	cd $(BIN_DIR) && ./$(APP_NAME) migrate create --name $$name

worker: build
	@echo "👷 Starting workers..."
	@cd $(BIN_DIR) && ./$(APP_NAME) worker --workers 5 --queue video_processing

health-check:
	@echo "❤️  Health checking service..."
	@if curl -s http://localhost:8080/health > /dev/null; then \
		echo "✅ Service is running"; \
	else \
		echo "❌ Service is not available"; \
	fi

## 🧪 Тестирование
test: proto
	@echo "🧪 Running all tests..."
	go test -v -race ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo "✅ Tests completed"

bench:
	@echo "📊 Running benchmarks..."
	go test -bench=. -benchmem ./...

load-test:
	@echo "⚡ Running load tests..."
	@if command -v k6 > /dev/null; then \
		k6 run scripts/loadtest.js; \
	else \
		echo "⚠ k6 is not installed. Install: https://k6.io/docs/getting-started/installation/"; \
	fi

## 🛠️  Code quality
lint:
	@echo "🔍 Linting code..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "⚠ golangci-lint is not installed"; \
	fi

vet:
	@echo "🔎 Checking code with vet..."
	go vet ./...
	@echo "✅ Vet completed"

fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...
	@echo "✅ Formatting completed"

security-check:
	@echo "🔒 Security checking..."
	@if command -v gosec > /dev/null; then \
		gosec ./...; \
	else \
		echo "⚠ gosec is not installed. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

## 📋 Утилиты
version: build
	@echo "📋 Version information:"
	@cd $(BIN_DIR) && ./$(APP_NAME) version

generate-docs: build
	@echo "📖 Generating documentation..."
	@cd $(BIN_DIR) && ./$(APP_NAME) generate docs
	@echo "✅ Documentation generated"

install-deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	@echo "✅ Dependencies installed"

deps:
	@echo "🔄 Updating dependencies..."
	go mod tidy
	go mod vendor
	@echo "✅ Dependencies updated"

init: install-deps proto
	@echo "✅ Project initialized"

clean:
	@echo "🧹 Cleaning..."
	rm -rf $(BIN_DIR) coverage.out
	go clean
	@echo "✅ Clean completed"
## 🌐 Dual API (HTTP + gRPC)
run-dual:
	@echo "🚀 Starting in DUAL mode (HTTP:8080 + gRPC:9090)..."
	@echo "HTTP REST: http://localhost:8080"
	@echo "gRPC:      localhost:9090"
	@echo ""
	go run ./cmd/api-gateway server --debug --grpc-port=9090

test-dual:
	@echo "🧪 Testing DUAL API..."
	@echo "1. Starting server..."
	@make run-dual &
	@SERVER_PID=$$!
	@sleep 3
	@echo ""
	@echo "2. Testing HTTP API..."
	@curl -s http://localhost:8080/health
	@echo ""
	@echo ""
	@echo "3. Testing gRPC client..."
	@cd scripts/clients && go run test_grpc_client.go
	@echo ""
	@echo "4. Testing HTTP Python client..."
	@cd scripts/clients && python test_http_client.py
	@echo ""
	@echo "✅ Dual API tests completed"
	@kill $$SERVER_PID 2>/dev/null || true

grpc-client:
	@echo "🚀 Running gRPC client..."
	@cd scripts/clients && go run test_grpc_client.go

http-client:
	@echo "🌐 Running HTTP client..."
	@cd scripts/clients && python test_http_client.py
