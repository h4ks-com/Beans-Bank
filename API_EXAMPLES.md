# Beapin API Examples

Complete examples for all API endpoints.

## Authentication

All authenticated endpoints require a Bearer token:
```
Authorization: Bearer <your_jwt_token>
```

In TEST_MODE, use header instead:
```
X-Test-Username: <username>
```

## Public Endpoints

### Get Total Beans in System

```bash
curl http://localhost:8080/api/v1/total
```

**Response:**
```json
{
  "total_beans": 1250
}
```

## Wallet Endpoints

### Get Wallet Balance

```bash
curl http://localhost:8080/api/v1/wallet \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Response:**
```json
{
  "username": "alice",
  "bean_amount": 100
}
```

### Get Transaction History

```bash
curl http://localhost:8080/api/v1/transactions \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Response:**
```json
[
  {
    "id": 1,
    "from_user": "alice",
    "to_user": "bob",
    "amount": 50,
    "timestamp": "2024-01-15T10:30:00Z"
  },
  {
    "id": 2,
    "from_user": "charlie",
    "to_user": "alice",
    "amount": 25,
    "timestamp": "2024-01-14T15:20:00Z"
  }
]
```

## Transfer Endpoints

### Transfer Beans

```bash
curl -X POST http://localhost:8080/api/v1/transfer \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "to_user": "bob",
    "amount": 50,
    "force": false
  }'
```

**Response (Success):**
```json
{
  "message": "transfer successful",
  "amount": 50,
  "to_user": "bob"
}
```

**Response (Insufficient Balance):**
```json
{
  "error": "insufficient balance"
}
```

**Response (Recipient Not Found):**
```json
{
  "error": "recipient not found, use force=true to create wallet"
}
```

### Transfer with Force Create

```bash
curl -X POST http://localhost:8080/api/v1/transfer \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "to_user": "newuser",
    "amount": 10,
    "force": true
  }'
```

Creates the recipient wallet if it doesn't exist.

## Token Management

### Create API Token

```bash
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "expires_in": "720h"
  }'
```

Duration formats: `24h`, `7d`, `720h` (30 days), etc.

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2024-02-14T10:30:00Z"
}
```

⚠️ **Important:** Save the token immediately! It's only shown once.

### List API Tokens

```bash
curl http://localhost:8080/api/v1/tokens \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Response:**
```json
[
  {
    "id": 1,
    "expires_at": "2024-02-14T10:30:00Z",
    "created_at": "2024-01-15T10:30:00Z"
  },
  {
    "id": 2,
    "expires_at": "2024-03-15T12:00:00Z",
    "created_at": "2024-01-20T12:00:00Z"
  }
]
```

### Delete API Token

```bash
curl -X DELETE http://localhost:8080/api/v1/tokens/1 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Response:**
```json
{
  "message": "token deleted successfully"
}
```

## Admin Endpoints

Requires admin user (configured in `ADMIN_USERS` env var).

### List All Users

```bash
curl http://localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer ADMIN_TOKEN"
```

**Response:**
```json
[
  {
    "username": "alice",
    "bean_amount": 150,
    "created_at": "2024-01-10T08:00:00Z"
  },
  {
    "username": "bob",
    "bean_amount": 75,
    "created_at": "2024-01-12T10:30:00Z"
  }
]
```

### List All Transactions

```bash
curl http://localhost:8080/api/v1/admin/transactions \
  -H "Authorization: Bearer ADMIN_TOKEN"
```

**Response:**
```json
[
  {
    "id": 1,
    "from_user": "alice",
    "to_user": "bob",
    "amount": 50,
    "timestamp": "2024-01-15T10:30:00Z"
  }
]
```

### Update Wallet Balance

```bash
curl -X PUT http://localhost:8080/api/v1/admin/wallet/alice \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "bean_amount": 1000
  }'
```

**Response:**
```json
{
  "username": "alice",
  "bean_amount": 1000
}
```

## Error Responses

All errors follow this format:

```json
{
  "error": "error description here"
}
```

Common HTTP status codes:
- `400` - Bad request (invalid input)
- `401` - Unauthorized (missing or invalid token)
- `403` - Forbidden (admin access required)
- `404` - Not found (user or resource doesn't exist)
- `500` - Internal server error

## Test Mode Examples

For development with `TEST_MODE=true`:

```bash
# Create token
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "X-Test-Username: alice" \
  -H "Content-Type: application/json" \
  -d '{"expires_in": "24h"}'

# Check wallet
curl http://localhost:8080/api/v1/wallet \
  -H "X-Test-Username: alice"

# Transfer
curl -X POST http://localhost:8080/api/v1/transfer \
  -H "X-Test-Username: alice" \
  -H "Content-Type: application/json" \
  -d '{"to_user": "bob", "amount": 10, "force": true}'
```

## Web Transfer Flow

Create a transfer link:
```
http://localhost:8080/transfer/alice/bob/50
```

This opens a web page where the sender can:
1. View transfer details
2. Authenticate (if not in TEST_MODE)
3. Confirm the transfer
4. See success/error message

## Rate Limiting & Best Practices

- Token expiry: Choose appropriate duration (24h-720h recommended)
- Regularly clean up expired tokens
- Use force flag cautiously (only when you trust the recipient username)
- Admin endpoints: Limit access to trusted administrators only
- Keep JWT_SECRET secure and rotate periodically
