# Food Delivery Order Processing Service

A real-time order-monitoring backend that ingests a continuous stream of food-delivery order events, keeps each order's state current, and exposes a REST API returning the latest state of every order.

---

## Architecture Overview

```
┌─────────────────────────────┐
│  Node.js Event Generator    │
│  ~2 events/sec              │
│  order.create (40%)         │
│  order.update.status (30%)  │
│  order.update.items (30%)   │
└────────────┬────────────────┘
             │ HTTP POST /events
             ▼
┌─────────────────────────────┐       ┌──────────────────┐
│  Go Backend (Gin)           │◄─────►│  PostgreSQL 16   │
│  POST /events               │  GORM │  orders (JSONB)  │
│  GET  /orders               │       └──────────────────┘
│  GET  /orders/:id           │
│  GET  /health               │
└─────────────────────────────┘
```

**Transport:** Direct HTTP (no message broker). The generator POSTs events synchronously; the backend processes and persists before returning 200 OK.

---

## Project Structure

```
food-delivery/
├── backend/
│   ├── cmd/server/main.go          # Entry point + graceful shutdown
│   ├── internal/
│   │   ├── config/config.go        # Env-based configuration
│   │   ├── database/database.go    # GORM + PostgreSQL setup
│   │   ├── models/order.go         # Domain model + status enum
│   │   ├── repository/             # Data access layer
│   │   ├── services/               # Business logic
│   │   ├── handlers/               # HTTP handlers
│   │   ├── middleware/             # Zap logger middleware
│   │   ├── routes/                 # Route registration
│   │   └── utils/                  # Response helpers
│   ├── Dockerfile
│   ├── go.mod
│   └── .env.example
│
├── generator/
│   ├── server.js                   # Main event loop
│   ├── services/eventService.js    # Event payload builders
│   ├── utils/http.js               # Fetch wrapper with retry
│   ├── package.json
│   └── .env.example
│
├── docker-compose.yml
├── README.md
├── API.md
└── DESIGN.md
```

---

## Prerequisites

| Tool | Minimum Version |
|------|----------------|
| Docker & Docker Compose | 24.x |
| Go | 1.24+ (only needed for local non-Docker run) |
| Node.js | 18+ |
| npm | 9+ |

---

## Quick Start (Recommended — Docker)

### 1. Clone the repository

```bash
git clone <repo-url>
cd food-delivery
```

### 2. Start PostgreSQL + Go Backend

```bash
docker compose up -d
```

The backend will be available at **http://localhost:8080** once the container is healthy (usually ~10 seconds).

Verify:
```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

### 3. Start the Event Generator

```bash
cd generator
cp .env.example .env   # Edit BACKEND_URL if needed
npm install
npm start
```

You will see output like:
```
============================================================
  Food Delivery Order Event Generator
============================================================
  Backend URL  : http://localhost:8080
  Interval     : 500ms (~2 events/sec)
  Log payloads : false
============================================================

[generator] → order.create
[generator] ✓ order created: 3f1a2b4c-… (total tracked: 1)
[generator] → order.update.status
[generator] → order.create
…
```

### 4. Query the API

```bash
# All orders (latest first)
curl "http://localhost:8080/orders"

# Filter by status
curl "http://localhost:8080/orders?status=Preparing&page=1&limit=10"

# Single order
curl "http://localhost:8080/orders/<orderId>"
```

---

## Running Locally (Without Docker)

### Backend

```bash
# 1. Start PostgreSQL (or use an existing instance)
# 2. Set environment variables
cd backend
cp .env.example .env
# Edit .env with your PostgreSQL credentials

# 3. Download dependencies
go mod download

# 4. Run
go run ./cmd/server
```

### Generator

```bash
cd generator
cp .env.example .env
npm install
npm start
```

---

## Environment Variables

### Backend (`backend/.env`)

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP listen port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL user |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `food_delivery` | Database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |

### Generator (`generator/.env`)

| Variable | Default | Description |
|----------|---------|-------------|
| `BACKEND_URL` | `http://localhost:8080` | Go backend base URL |
| `EVENT_INTERVAL_MS` | `500` | Milliseconds between events |
| `LOG_EVENTS` | `false` | Log full JSON payloads |

---

## Stopping the System

```bash
# Stop backend + DB
docker compose down

# Stop and remove data volume
docker compose down -v
```

---

## Design Decisions

See [DESIGN.md](./DESIGN.md) for a full architecture write-up and reasoning behind every major design decision.

## API Reference

See [API.md](./API.md) for complete endpoint documentation with request/response examples.
