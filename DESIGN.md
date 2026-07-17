# Design Document — Food Delivery Order Processing Service

## Overview

The project consists of two applications:

- **Node.js Event Generator** – Continuously generates random order events.
- **Go Backend** – Processes events, stores the latest order state in PostgreSQL, and exposes REST APIs.

```
Node.js Generator
        |
        | HTTP POST /events
        v
Go Backend (Gin)
        |
        | GORM
        v
 PostgreSQL
```

---

## Architecture

The backend follows a layered architecture:

```
Handlers
    ↓
Services
    ↓
Repositories
    ↓
PostgreSQL
```

- **Handlers** receive HTTP requests.
- **Services** contain business logic.
- **Repositories** interact with PostgreSQL.
- **Models** define the application's data structures.

This separation keeps the code organized and easier to maintain.

---

## Database

A single `orders` table stores the latest state of each order.

| Column |
|--------|
| order_id |
| customer_id |
| restaurant_id |
| status |
| items (JSONB) |
| created_at |
| updated_at |

Indexes:

- status
- updated_at

---

## Event Processing

The backend accepts three event types:

- `order.create`
- `order.update.status`
- `order.update.items`

All events are sent to:

```
POST /events
```

The backend identifies the event type and processes it using the appropriate service.

---

## Order State

Supported status flow:

```
Received
    |
    +--> Preparing
    |       |
    |       +--> Complete
    |       |
    |       +--> Cancelled
    |
    +--> Cancelled
```

Invalid transitions return **400 Bad Request**.

---

## Error Handling

The service returns standard HTTP status codes.

| Status | Meaning |
|--------|---------|
| 200 | Success |
| 400 | Invalid request or status transition |
| 404 | Order not found |
| 500 | Internal server error |

All responses follow the same JSON format.

---

## Logging

Uber Zap is used for structured logging.

The application logs:

- incoming requests
- processed events
- errors
- server startup

---

