# Multi-stage Dockerfile for Go services
# Usage: docker build --build-arg SERVICE=gateway .
# Lua scripts are embedded via go:embed — no runtime file copy needed.

# ── Stage 1: Build ──
FROM golang:1.22-alpine AS builder

ARG SERVICE

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the specific service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/${SERVICE}

# ── Stage 2: Runtime ──
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget tzdata

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 3000 3001 3002 3003 3004

ENTRYPOINT ["./server"]
