# Design Document — Food Delivery Order Processing Service

## 1. Architecture Overview

The system is composed of two independent processes connected over plain HTTP:

```
Node.js Generator  ──HTTP POST /events──►  Go Backend (Gin)  ──GORM──►  PostgreSQL
```

The Go backend is stateless. All state lives in PostgreSQL. The generator maintains a small in-memory set of confirmed order IDs so it can generate update events for orders that exist.

---

## 2. Why HTTP instead of a Message Broker?

The assignment gave free choice of transport. I chose **direct HTTP** for the following reasons:

| Concern | Reasoning |
|---------|-----------|
| **Simplicity** | No broker to operate. One less moving part means a reviewer can `docker compose up` and have a running system in seconds. |
| **Consistency** | With HTTP the generator gets an immediate 200/400/404 back from the backend. It can track which orders were successfully created without any additional coordination layer. |
| **Correctness** | The generator only tracks `orderId` values from successful 200 responses, so it never sends update events for orders that failed to persist. |
| **Scalability ceiling is acceptable** | HTTP is synchronous. At high throughput (thousands of events/sec) a message queue would decouple producer from consumer. At the scale of this assignment (~2 events/sec), that tradeoff is not worth the operational cost. |

**If this were a production system at scale:** I would introduce NATS or Kafka between the generator and backend. The backend would become a consumer group, enabling horizontal scale-out and durability through the broker's log. The design of the backend service (stateless, single `POST /events` processing path) makes that migration straightforward — only the transport layer would change.

---

## 3. Database Design

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

### Why JSONB for items?

The `items` field is a variable-length list of `{itemId, qty}` pairs. Normalising this into a separate `order_items` table would add a JOIN on every read. Since items are always read and written as a whole (the `order.update.items` event replaces the entire list), JSONB is a natural fit:

- Single-row reads/writes — no JOIN needed.
- PostgreSQL JSONB is indexed, validated, and efficiently stored.
- The full items list is always returned, so there is no over-fetching.

### Why a single `orders` table?

We are asked to maintain *current state*, not an audit log of every event. A single mutable row per order is the correct data model for this. An event-sourced append-only log would be the right choice if we needed full history, but that is a different requirement.

### Indexes

- `idx_orders_status` — supports the `?status=` filter on `GET /orders`.
- `idx_orders_updated_at` — supports the default sort order (`updated_at DESC`) on `GET /orders`.

---

## 4. Folder Structure & Clean Architecture

```
backend/
  cmd/server/main.go        # Entry point only — wiring, server start, shutdown
  internal/
    config/                 # Configuration — no business logic
    database/               # DB connection — no business logic
    models/                 # Domain types — no business logic
    repository/             # Data access — knows about DB, not about HTTP
    services/               # Business logic — knows about domain, not about HTTP or DB driver
    handlers/               # HTTP layer — translates HTTP ↔ service calls
    middleware/             # Cross-cutting HTTP concerns
    routes/                 # Route registration
    utils/                  # Shared HTTP response helpers
```

Each layer only imports the layer(s) below it. Handlers call services; services call repositories; repositories call the DB. This keeps business logic testable in isolation.

---

## 5. Event Flow

```
POST /events (JSON body)
        │
        ▼
EventHandler.Handle()
  ├── peek at "type" field
  ├── "order.create"        → handleCreate()  → OrderService.CreateOrder()
  ├── "order.update.status" → handleUpdateStatus() → OrderService.UpdateStatus()
  └── "order.update.items"  → handleUpdateItems()  → OrderService.UpdateItems()
        │
        ▼
  OrderService (business logic + validation)
        │
        ▼
  OrderRepository (GORM)
        │
        ▼
  PostgreSQL
        │
        ▼
  200 OK (with persisted order in body)
```

---

## 6. Design Considerations (§5 of the Assignment)

### 6.1 Out-of-Order Events

**Decision: Return 404 and do not create phantom orders.**

If an `order.update.status` or `order.update.items` arrives for an `orderId` that does not exist in the database, the backend returns `404 Not Found`.

**Reasoning:**

- The generator only sends update events for order IDs it received from successful `order.create` responses, so this scenario only occurs if an update genuinely arrives before the create — which cannot happen with synchronous HTTP and a single generator.
- In a distributed system with multiple generators or a message broker (where ordering is not guaranteed), a common approach is to buffer the early update and retry after a short delay, or to use a "create-if-not-exists" upsert. For this scope, 404 is the simplest correct behaviour and is explicitly documented.
- Alternatively, one could create a "ghost" order record on the first update, but this risks polluting the database with incomplete orders from genuinely invalid events.

### 6.2 Concurrency

**Decision: Row-level locking with `SELECT FOR UPDATE` inside a database transaction.**

The `UpdateStatus` and `UpdateItems` service methods wrap their work in a `db.Transaction()` block and acquire a row-level lock on the target order before reading or writing it:

```go
tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "order_id = ?", id)
```

**What this achieves:**

- Two concurrent goroutines updating the same order will serialise at the database level. The second goroutine blocks on `SELECT FOR UPDATE` until the first commits, then reads the committed state.
- No lost updates — the last writer wins in a controlled, serialised manner.
- Status transition validation always reads the current committed status before deciding whether the transition is valid.

**Alternative considered:** Optimistic locking (version counter / updated_at comparison). Rejected because it requires a retry loop on conflict, which adds complexity without benefit at this scale. Pessimistic locking is simpler and correct here.

### 6.3 Duplicate / Replayed Events

**For `order.create`:** Each call generates a new UUID. Replaying an identical create payload produces a second distinct order. This is acceptable because there is no natural deduplication key for creates in this design (no idempotency token in the payload). In production, the generator would include an `eventId` and the backend would maintain a seen-IDs table.

**For `order.update.status` and `order.update.items`:** These are naturally idempotent. Replaying `status=Preparing` on an order already in `Preparing` will fail the transition check (Preparing → Preparing is not in the allowed set) and return a 400. Replaying `update.items` with the same items will re-write the same JSONB and bump `updated_at` — a benign outcome.

### 6.4 Status Transition Rules

**Allowed transitions (enforced in `models/order.go`):**

```
Received  → Preparing   ✓
Received  → Cancelled   ✓
Preparing → Complete    ✓
Preparing → Cancelled   ✓
Complete  → (anything)  ✗  — terminal state
Cancelled → (anything)  ✗  — terminal state
```

Invalid transitions return `400 Bad Request` with a descriptive message.

**Reasoning:** `Complete` and `Cancelled` are terminal states — an order that has been delivered or cancelled cannot meaningfully change status. The two allowed paths from `Received` (either start preparing or cancel immediately) mirror real food-delivery workflows.

### 6.5 Throughput

At the current scale (~2 events/sec) the system is comfortably within the capacity of a single PostgreSQL instance and Go backend.

**Design choices that support higher throughput if needed:**

- **Stateless backend:** The Go service holds no in-process state. Horizontal scaling (multiple instances behind a load balancer) is straightforward. Row-level locking ensures correctness across instances.
- **Connection pool:** GORM is configured with `MaxOpenConns=25`, `MaxIdleConns=10`, and a 5-minute connection lifetime, preventing connection exhaustion under burst load.
- **Indexes:** The `status` and `updated_at` indexes prevent full-table scans on the most common query patterns.
- **Async transport:** Swapping HTTP for NATS/Kafka would decouple event ingestion rate from processing rate, enabling the backend to absorb traffic spikes without back-pressure on the generator.

### 6.6 "Latest" Guarantees

**Decision: Synchronous write-before-response.**

The backend does not return 200 until the event has been committed to PostgreSQL. The generator receives the 200 only after the state is durable.

This means:
- A `GET /orders` issued immediately after a successful `POST /events` will always reflect the latest state.
- There is no eventual consistency window — reads are strongly consistent for the single-node PostgreSQL setup used here.
- PostgreSQL's default isolation level (`READ COMMITTED`) means readers never see uncommitted writes.

---

## 7. Error Handling

All errors are returned as JSON with a consistent envelope:

```json
{ "success": false, "message": "…" }
```

The service layer uses sentinel errors (`ErrOrderNotFound`, `ErrInvalidStatusTransition`, etc.) which handlers map to the appropriate HTTP status code. This keeps HTTP concerns out of the service layer.

---

## 8. Logging

Zap is used throughout for structured, high-performance logging:

- **Startup:** configuration values, DB connection
- **Each event:** type logged on arrival
- **Creates:** new orderId logged
- **Updates:** orderId and new state logged
- **Errors:** full error chain with context
- **Every HTTP request:** method, path, status, latency (via middleware)

---

## 9. Future Improvements

| Improvement | Description |
|-------------|-------------|
| **Idempotency tokens** | Generator includes an `eventId`; backend deduplicates on it |
| **Event sourcing** | Append-only `order_events` table for full audit history |
| **Message broker** | Replace HTTP with NATS/Kafka for decoupling and durability |
| **Authentication** | API key or JWT for the `/events` endpoint |
| **Rate limiting** | Prevent generator misconfiguration from overwhelming the backend |
| **Metrics** | Prometheus `/metrics` endpoint for event rates, error rates, latency |
| **Tracing** | OpenTelemetry trace propagation across generator → backend |
| **Unit tests** | Service and repository layers are designed for testability (interfaces) |
