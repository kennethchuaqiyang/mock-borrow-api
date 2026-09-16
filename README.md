# Mock Borrow API

A lightweight mock API server built in Go, simulating a user metadata lookup and a borrow/lending flow. Built as a testing/automation portfolio project.

**Live demo:** https://mock-borrow-api.onrender.com

> Hosted on Render's free tier — the first request after a period of inactivity may take 20-30 seconds while the service wakes up.

## Endpoints

### GET /api/user
Fetches user metadata.

**Query params:** `username`, `location`, `userid` (int)
**Headers:** `X-Browser-Type`, `X-Admin-Flag` (boolean)

```bash
curl "https://mock-borrow-api.onrender.com/api/user?username=john&location=Singapore&userid=1" \
  -H "X-Browser-Type: Chrome" \
  -H "X-Admin-Flag: true"
```

**Response:**
```json
{
  "metadata": { "username": "john", "location": "Singapore", "salary": 5000 },
  "cache": { "username": "john", "user_identity": 1, "salary": 5000 }
}
```

### POST /api/borrow
Submits a borrow request. Rejected if the new total owed exceeds 200.

**Body (JSON):** `username`, `location`, `userid` (int), `amount_to_borrow`
**Headers:** `X-Browser-Type`, `X-Admin-Flag` (boolean)

```bash
curl -X POST "https://mock-borrow-api.onrender.com/api/borrow" \
  -H "Content-Type: application/json" \
  -H "X-Browser-Type: Chrome" \
  -H "X-Admin-Flag: false" \
  -d '{"username":"john","location":"Singapore","userid":1,"amount_to_borrow":50}'
```

**Response:**
```json
{
  "metadata": {
    "username": "john",
    "location": "Singapore",
    "amount_owed": 50,
    "amount_altered_to_borrow": 50,
    "allowed_to_borrow": true
  }
}
```

Both endpoints return `405 Method Not Allowed` for any other HTTP method.

## Response headers (both endpoints)
- `X-Browser` — echoes the `X-Browser-Type` request header
- `X-Secret-Key` — SHA-256 hash of `userid + username + current date + secret`

## Running locally

```bash
go run main.go
```
Listens on `:8080` by default (or `$PORT` if set, for deployment platforms like Render).

## Tests

```bash
go test -v .
```

Covers happy-path GET/POST, borrow-limit rejection, and wrong-method (405) handling for both endpoints.

## Data storage

Currently in-memory (resets on restart/redeploy). Structured to swap in a real DB (e.g. PostgreSQL via Neon) later without changing the handler logic.
