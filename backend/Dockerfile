# ──────────────────────────────────────────────────────────────────────────────
# Stage 1 — Build
# ──────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

# Install git (needed for go mod download of some packages).
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Download dependencies first — leverages Docker layer cache so this step is
# skipped on rebuilds when only source code changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /food-delivery-server ./cmd/server

# ──────────────────────────────────────────────────────────────────────────────
# Stage 2 — Runtime
# ──────────────────────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from the builder stage.
COPY --from=builder /food-delivery-server .

# Copy .env.example so operators can see all available variables.
COPY .env.example .env.example

EXPOSE 8080

ENTRYPOINT ["/app/food-delivery-server"]
