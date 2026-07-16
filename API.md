# API Reference — Food Delivery Order Processing Service

Base URL: `http://localhost:8080`

All responses use the following envelope:

**Success**
```json
{ "success": true, "data": { … } }
```

**Error**
```json
{ "success": false, "message": "…" }
```

---

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/events` | Ingest an order event |
| `GET` | `/orders` | List all orders (paginated) |
| `GET` | `/orders/:id` | Get a single order |

---

## GET /health

Health check for the running service.

### Response `200 OK`

```json
{ "status": "ok" }
```

---

## POST /events

Accepts all three event types. The event type is determined by the `type` field.

### Headers

```
Content-Type: application/json
```

---

### Event: `order.create`

Creates a new order. The backend generates `orderId`, `createdAt`, `updatedAt`, and sets the initial status to `Received`.

**Request Body**

```json
{
  "type": "order.create",
  "customerId": "cust-abc-123",
  "restaurantId": "rest-xyz-456",
  "items": [
    { "itemId": "item-001", "qty": 2 },
    { "itemId": "item-002", "qty": 1 }
  ]
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `type` | string | ✓ | Must be `"order.create"` |
| `customerId` | string | ✓ | |
| `restaurantId` | string | ✓ | |
| `items` | array | ✓ | Min 1 item; each item requires `itemId` (string) and `qty` (int > 0) |

**Response `200 OK`**

```json
{
  "success": true,
  "data": {
    "orderId": "3f1a2b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c",
    "customerId": "cust-abc-123",
    "restaurantId": "rest-xyz-456",
    "status": "Received",
    "items": [
      { "itemId": "item-001", "qty": 2 },
      { "itemId": "item-002", "qty": 1 }
    ],
    "createdAt": "2024-07-16T14:00:00Z",
    "updatedAt": "2024-07-16T14:00:00Z"
  }
}
```

**Error Responses**

| Status | Condition |
|--------|-----------|
| `400` | Missing required fields or invalid items |

---

### Event: `order.update.status`

Updates the status of an existing order. Invalid transitions are rejected.

**Allowed status transitions:**

```
Received  → Preparing
Received  → Cancelled
Preparing → Complete
Preparing → Cancelled
```

All other transitions (e.g. `Complete → Preparing`) return `400`.

**Request Body**

```json
{
  "type": "order.update.status",
  "orderId": "3f1a2b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c",
  "status": "Preparing"
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `type` | string | ✓ | Must be `"order.update.status"` |
| `orderId` | string (UUID) | ✓ | Must reference an existing order |
| `status` | string (enum) | ✓ | `Received` \| `Preparing` \| `Complete` \| `Cancelled` |

**Response `200 OK`**

```json
{
  "success": true,
  "data": {
    "orderId": "3f1a2b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c",
    "customerId": "cust-abc-123",
    "restaurantId": "rest-xyz-456",
    "status": "Preparing",
    "items": [
      { "itemId": "item-001", "qty": 2 }
    ],
    "createdAt": "2024-07-16T14:00:00Z",
    "updatedAt": "2024-07-16T14:01:00Z"
  }
}
```

**Error Responses**

| Status | Condition |
|--------|-----------|
| `400` | Invalid or unrecognised status, invalid status transition |
| `404` | Order not found |

---

### Event: `order.update.items`

Replaces the entire item list of an existing order.

**Request Body**

```json
{
  "type": "order.update.items",
  "orderId": "3f1a2b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c",
  "items": [
    { "itemId": "item-003", "qty": 3 }
  ]
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `type` | string | ✓ | Must be `"order.update.items"` |
| `orderId` | string (UUID) | ✓ | Must reference an existing order |
| `items` | array | ✓ | Replaces existing items. Min 1; qty must be > 0 |

**Response `200 OK`**

```json
{
  "success": true,
  "data": {
    "orderId": "3f1a2b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c",
    "customerId": "cust-abc-123",
    "restaurantId": "rest-xyz-456",
    "status": "Preparing",
    "items": [
      { "itemId": "item-003", "qty": 3 }
    ],
    "createdAt": "2024-07-16T14:00:00Z",
    "updatedAt": "2024-07-16T14:02:00Z"
  }
}
```

**Error Responses**

| Status | Condition |
|--------|-----------|
| `400` | Missing or invalid items (empty array, qty ≤ 0) |
| `404` | Order not found |

---

## GET /orders

Returns a paginated list of all orders.

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | `1` | Page number (1-indexed) |
| `limit` | int | `20` | Results per page (max 100) |
| `status` | string | — | Filter by status (`Received` \| `Preparing` \| `Complete` \| `Cancelled`) |
| `sort` | string | `updated_at` | Sort field |
| `order` | string | `desc` | Sort direction (`asc` \| `desc`) |

### Examples

```bash
# All orders, latest first
GET /orders

# Paginated
GET /orders?page=2&limit=10

# Filter by status
GET /orders?status=Preparing

# Oldest first
GET /orders?sort=updated_at&order=asc

# Combined
GET /orders?page=1&limit=10&status=Preparing&sort=updated_at&order=desc
```

### Response `200 OK`

```json
{
  "success": true,
  "data": [
    {
      "orderId": "3f1a2b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c",
      "customerId": "cust-abc-123",
      "restaurantId": "rest-xyz-456",
      "status": "Preparing",
      "items": [
        { "itemId": "item-001", "qty": 2 }
      ],
      "createdAt": "2024-07-16T14:00:00Z",
      "updatedAt": "2024-07-16T14:01:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 142,
    "totalPages": 8
  }
}
```

**Error Responses**

| Status | Condition |
|--------|-----------|
| `500` | Internal server error |

---

## GET /orders/:id

Returns a single order by its UUID.

### Path Parameter

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string (UUID) | The order ID returned by `order.create` |

### Example

```bash
GET /orders/3f1a2b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c
```

### Response `200 OK`

```json
{
  "success": true,
  "data": {
    "orderId": "3f1a2b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c",
    "customerId": "cust-abc-123",
    "restaurantId": "rest-xyz-456",
    "status": "Complete",
    "items": [
      { "itemId": "item-001", "qty": 2 },
      { "itemId": "item-002", "qty": 1 }
    ],
    "createdAt": "2024-07-16T14:00:00Z",
    "updatedAt": "2024-07-16T14:10:00Z"
  }
}
```

**Error Responses**

| Status | Condition |
|--------|-----------|
| `404` | Order with given ID does not exist |
| `500` | Internal server error |

---

## Status Codes Summary

| Code | Meaning |
|------|---------|
| `200` | Request processed successfully |
| `400` | Bad request (validation error, invalid transition) |
| `404` | Resource not found |
| `500` | Internal server error |
