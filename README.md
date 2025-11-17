# Beans Bank API 🫘

Bean currency management system for h4ks.com

## Features

- 🪙 Integer-based bean currency system
- 👛 Automatic wallet creation with 1 bean initial balance
- 💸 Safe transfers with ACID transaction guarantees
- 🔐 JWT API token authentication
- 🎫 User-managed API tokens with expiry
- 👨‍💼 Admin endpoints for system management
- 📊 Automatic Swagger documentation
- 🐳 Docker and Docker Compose support
- ✅ Comprehensive test coverage

## Quick Start

### Local Development

```bash
# Install dependencies
go mod download

# Setup environment
cp .env.example .env
# Edit .env with your configuration

# Generate Swagger documentation
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go

# Run the server
go run cmd/server/main.go
```

Server will start at `http://localhost:8080`

### Docker

```bash
# Start with Docker Compose
docker compose up -d

# View logs
docker compose logs -f beapin

# Stop services
docker compose down
```

## API Documentation

Visit `http://localhost:8080/swagger/index.html` for interactive API documentation.

## Testing

```bash
# Run unit tests
make test-unit

# Run all tests with coverage
make test-cover

# Quick test
go test ./internal/services/...

# Generate coverage HTML
make test-cover
# Opens coverage.html
```

For E2E testing, see `tests/manual_e2e.md` for step-by-step manual testing guide.

## Environment Variables

See `.env.example` for all available configuration options.

Key variables:
- `PORT` - Server port (default: 8080)
- `DATABASE_URL` - PostgreSQL connection string
- `JWT_SECRET` - Secret for JWT token signing
- `SESSION_SECRET` - Secret for session cookie encryption
- `SESSION_SECURE` - Set to `true` in production with HTTPS (default: false)
- `ADMIN_USERS` - Comma-separated list of admin usernames
- `TEST_MODE` - Set to `true` to bypass authentication (testing only)

## API Endpoints

### Public
- `GET /` - Home page with transfer link generator
- `GET /total` - Get total beans in system
- `GET /swagger/*` - API documentation

### Authenticated (requires Bearer token)
- `GET /api/v1/wallet` - Get wallet balance
- `GET /api/v1/transactions` - Get transaction history
- `POST /api/v1/transfer` - Transfer beans
- `POST /api/v1/tokens` - Create API token
- `GET /api/v1/tokens` - List API tokens
- `DELETE /api/v1/tokens/:id` - Delete API token

### Admin (requires admin user)
- `GET /api/v1/admin/users` - List all users
- `GET /api/v1/admin/transactions` - List all transactions
- `PUT /api/v1/admin/wallet/:username` - Update wallet balance

## Authentication

### API Tokens

1. Create a token (requires existing token or TEST_MODE):
```bash
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"expires_in": "720h"}'
```

2. Use the token:
```bash
curl http://localhost:8080/api/v1/wallet \
  -H "Authorization: Bearer YOUR_NEW_TOKEN"
```

### Test Mode

For development and testing, enable TEST_MODE:

```bash
export TEST_MODE=true
```

Then use the `X-Test-Username` header:

```bash
curl http://localhost:8080/api/v1/wallet \
  -H "X-Test-Username: alice"
```

## Project Structure

```
beapin/
├── cmd/server/         # Application entry point
├── internal/
│   ├── config/         # Configuration management
│   ├── models/         # GORM data models
│   ├── handlers/       # HTTP request handlers
│   ├── middleware/     # Authentication & authorization
│   ├── services/       # Business logic
│   ├── repository/     # Data access layer
│   └── database/       # Database connection
├── web/templates/      # HTML templates
├── docs/               # Generated Swagger docs
├── .github/workflows/  # CI/CD pipelines
└── tests/              # Test files

```

## Development

```bash
# Run tests on file changes
go test -v ./... -count=1

# Format code
go fmt ./...

# Lint code (requires golangci-lint)
golangci-lint run

# Build binary
go build -o beapin ./cmd/server
```

## License

MIT
