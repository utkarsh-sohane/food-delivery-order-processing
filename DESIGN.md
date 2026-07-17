# Design Document — Food Delivery Order Processing Service

## Overview

The system consists of two independent processes:

- **Node.js Event Generator** — continuously emits randomised order events via `POST /events`
- **Go Backend** — processes events, persists current order state to PostgreSQL, and exposes a REST API

```
Node.js Generator  ──HTTP POST /events──►  Go Backend (Gin)  ──GORM──►  PostgreSQL
```

The Go backend is fully stateless. All persistent state lives in PostgreSQL. The generator maintains an in-memory list of confirmed order IDs (from successful `order.create` responses) to ensure update events always target real orders.

---

## Transport: HTTP over a Message Broker

Direct HTTP was chosen over NATS, Kafka, or RabbitMQ for the following reasons:

- **Zero infrastructure overhead** — no broker to run, making the project immediately runnable with `docker compose up`.
- **Synchronous confirmation** — the generator receives a `200`/`400`/`404` response per event, so it can track created order IDs without a separate coordination mechanism.
- **Correctness by design** — update events are only sent for order IDs confirmed by a successful create response, eliminating phantom references.

For the expected workload of this assignment, the synchronous overhead of HTTP is negligible. The backend is designed to be stateless, so migrating to a message broker (NATS/Kafka) in the future would only require replacing the transport layer.

---

## Database Design

### Schema

```sql
CREATE TABLE orders (
    order_id      UUID         PRIMARY KEY,
    customer_id   TEXT         NOT NULL,
    restaurant_id TEXT         NOT NULL,
    status        TEXT         NOT NULL DEFAULT 'Received',
    items         JSONB        NOT NULL DEFAULT '[]',
    created_at    TIMESTAMPTZ  NOT NULL,
    updated_at    TIMESTAMPTZ  NOT NULL
);

CREATE INDEX idx_orders_status     ON orders(status);
CREATE INDEX idx_orders_updated_at ON orders(updated_at DESC);
```

### Design Decisions

**Single `orders` table:** The requirement is to maintain *current state*, not a full event history. A single mutable row per order is the correct model. Event sourcing would be the right approach if full history were needed, but that is out of scope here.

**JSONB for `items`:** Items are always read and written as a complete list (`order.update.items` replaces the entire array). Storing them as JSONB avoids a JOIN on every read without sacrificing query flexibility or storage efficiency.

**Indexes:** `idx_orders_status` supports status filtering on `GET /orders`. `idx_orders_updated_at` supports the default sort order (`updated_at DESC`).

---

## Application Architecture

```
backend/
  cmd/server/main.go     # Entry point: wiring, server lifecycle, graceful shutdown
  internal/
    config/              # Environment-based configuration
    database/            # GORM connection and migrations
    models/              # Domain types, status enum, transition rules
    repository/          # Data access interface and GORM implementation
    services/            # Business logic and validation
    handlers/            # HTTP request/response translation
    middleware/          # Zap request logger
    routes/              # Route registration
    utils/               # Shared response helpers
```

Each layer only imports the layer(s) below it. Business logic is isolated in the service layer, while handlers manage HTTP requests and repositories handle persistence. The repository interface enables service-layer testing without a real database.

---

## Event Processing

All three event types are received at `POST /events`. The handler inspects the `type` field and dispatches the request to the appropriate service method:

```
POST /events
  ├── "order.create"        → CreateOrder()
  ├── "order.update.status" → UpdateStatus()
  └── "order.update.items"  → UpdateItems()
```

Each path runs synchronously. The backend does not return `200 OK` until the event has been committed to PostgreSQL, ensuring reads always reflect the latest state.

---

## Design Decisions

### Out-of-Order Events

Update events for an unknown `orderId` return `404 Not Found`. No ghost records are created. With synchronous HTTP and a single generator, this situation cannot arise in normal operation. In a multi-producer or broker-based setup, a buffered retry or upsert strategy would be more appropriate.

### Concurrency

`UpdateStatus` and `UpdateItems` both acquire a row-level lock (`SELECT FOR UPDATE`) inside a database transaction before reading or modifying an order. This serialises concurrent updates to the same row at the database level, preventing lost writes and ensuring status transition validation always runs against committed state.



### Duplicate and Replayed Events

- **`order.create`:** Each invocation generates a new UUID, so replays produce a new order. Full deduplication would require an idempotency token in the payload.
- **`order.update.status`:** Replaying the same status fails the transition check and returns `400`. Idempotent by design.
- **`order.update.items`:** Replaying the same items rewrites the same JSONB and bumps `updated_at`. No harmful side effects.

### Status Transitions

Transitions are enforced via an allowlist in `models/order.go`:

| From | Allowed Next States |
|---|---|
| `Received` | `Preparing`, `Cancelled` |
| `Preparing` | `Complete`, `Cancelled` |
| `Complete` | — *(terminal)* |
| `Cancelled` | — *(terminal)* |

Invalid transitions return `400 Bad Request`. `Complete` and `Cancelled` are terminal states with no valid outbound transitions.

### Throughput

The backend is stateless and horizontally scalable. The design can be scaled further through:

- GORM connection pool (`MaxOpenConns=25`, `MaxIdleConns=10`, 5-minute lifetime)
- Indexed queries on `status` and `updated_at`
- Row-level locking that remains correct across multiple backend instances

Replacing HTTP with a message broker would decouple ingestion throughput from processing throughput if needed at scale.

### Latest State Guarantee

The backend writes synchronously before returning `200`. PostgreSQL's `READ COMMITTED` isolation ensures no reader ever sees an uncommitted write. `GET /orders` always reflects the most recently committed event.

---

## Error Handling

All errors are returned as:

```json
{ "success": false, "message": "..." }
```

The service layer uses typed sentinel errors (`ErrOrderNotFound`, `ErrInvalidStatusTransition`, `ErrInvalidItems`) that handlers map to appropriate HTTP status codes. HTTP concerns do not leak into the service layer.

---

## Logging

Structured logging via Uber Zap throughout:

- Server startup and configuration
- Every incoming event (type and order ID)
- State transitions (old status → new status)
- Validation and database errors
- Every HTTP request (method, path, status code, latency) via middleware

---

## Future Improvements

| Area | Description |
|---|---|
| Idempotency tokens | Add `eventId` to payloads; backend deduplicates on first-seen |
| Event history | Append-only `order_events` table for full audit trail |
| Message broker | NATS or Kafka for decoupled, durable event ingestion |
| Authentication | API key or JWT on `POST /events` |
| Rate limiting | Protect the event endpoint from misconfigured producers |
| Observability | Prometheus metrics + OpenTelemetry tracing |
| Tests | Service and repository interfaces are structured for unit testing |
