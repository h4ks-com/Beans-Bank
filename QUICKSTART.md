# Beapin Quick Start Guide

## 🚀 Get Started in 5 Minutes

### 1. Clone and Setup

```bash
git clone https://github.com/h4ks-com/beapin.git
cd beapin
cp .env.example .env
```

### 2. Start the Server

**Option A: Local (requires Go 1.23+)**
```bash
go mod download
swag init -g cmd/server/main.go
go run cmd/server/main.go
```

**Option B: Docker**
```bash
docker compose up -d
```

### 3. Test with curl

```bash
# Create first user's wallet (TEST_MODE auto-creates with 1 bean)
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "X-Test-Username: alice" \
  -H "Content-Type: application/json" \
  -d '{"expires_in": "720h"}'

# Save the token from response
export TOKEN="your_token_here"

# Check wallet
curl http://localhost:8080/api/v1/wallet \
  -H "Authorization: Bearer $TOKEN"

# Transfer beans (force creates recipient if needed)
curl -X POST http://localhost:8080/api/v1/transfer \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"to_user": "bob", "amount": 5, "force": true}'

# Check total beans in system
curl http://localhost:8080/api/v1/total
```

### 4. Web Interface

Visit `http://localhost:8080` to:
- See total beans in the system
- Create transfer links
- View API documentation at `/swagger/index.html`

## 🧪 Testing Flow

```bash
# Enable test mode in .env
TEST_MODE=true

# Create token for alice
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "X-Test-Username: alice" \
  -H "Content-Type: application/json" \
  -d '{"expires_in": "24h"}' | jq -r '.token'

# Use token for authenticated requests
curl http://localhost:8080/api/v1/wallet \
  -H "Authorization: Bearer <token>"

# Make a transfer
curl -X POST http://localhost:8080/api/v1/transfer \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"to_user": "bob", "amount": 3, "force": true}'
```

## 📝 Admin Operations

```bash
# Set admin user in .env
ADMIN_USERS=alice

# Get admin token
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "X-Test-Username: alice" \
  -d '{"expires_in": "24h"}'

# List all users
curl http://localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer <admin_token>"

# Update wallet balance
curl -X PUT http://localhost:8080/api/v1/admin/wallet/bob \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"bean_amount": 1000}'
```

## 🔐 Production Setup

1. Set `TEST_MODE=false` in `.env`
2. Configure Logto OIDC credentials
3. Change `JWT_SECRET` and `SESSION_SECRET`
4. Use PostgreSQL instead of SQLite:
   ```
   DATABASE_URL=postgres://user:pass@host:5432/beapin
   ```

## 📚 Next Steps

- Read [README.md](README.md) for full documentation
- Explore API at http://localhost:8080/swagger/index.html
- Run tests: `go test ./...`
- Check out the [GitHub repo](https://github.com/h4ks-com/beapin)
