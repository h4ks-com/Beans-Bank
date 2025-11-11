.PHONY: help build run test swagger clean docker-build docker-up docker-down

help:
	@echo "Beapin - Bean Currency API"
	@echo ""
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  run          - Run the server"
	@echo "  test         - Run all tests"
	@echo "  test-unit    - Run unit tests only"
	@echo "  test-e2e     - Run E2E tests"
	@echo "  test-all     - Run unit and E2E tests"
	@echo "  test-cover   - Run tests with coverage"
	@echo "  swagger      - Generate Swagger documentation"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-up    - Start Docker Compose services"
	@echo "  docker-down  - Stop Docker Compose services"
	@echo "  fmt          - Format Go code"
	@echo "  lint         - Run linter"
	@echo "  deps         - Download and tidy dependencies"

build: swagger
	go build -o beapin ./cmd/server

run: swagger
	go run cmd/server/main.go

test:
	go test -v -race ./...

test-unit:
	go test -v -race ./internal/...

test-e2e:
	@echo "Running E2E tests with testcontainers..."
	go test -v ./tests/... -run E2E

test-all: test-unit test-e2e
	@echo "✅ All tests passed!"

test-cover:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

swagger:
	@command -v swag >/dev/null 2>&1 || { echo "Installing swag..."; go install github.com/swaggo/swag/cmd/swag@latest; }
	swag init -g cmd/server/main.go

clean:
	rm -f beapin
	rm -f coverage.out coverage.html
	rm -rf docs/

docker-build:
	docker build -t beapin:latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f beapin

fmt:
	go fmt ./...

lint:
	golangci-lint run

deps:
	go mod download
	go mod tidy
