# ============================================
# Stage 1: Build Go Binaries
# ============================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for dependency fetching
RUN apk add --no-cache git

# Copy go mod files first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/crawler ./cmd/crawler && \
    CGO_ENABLED=0 GOOS=linux go build -o bin/normalizer ./cmd/normalizer && \
    CGO_ENABLED=0 GOOS=linux go build -o bin/uploader ./cmd/uploader && \
    CGO_ENABLED=0 GOOS=linux go build -o bin/formatter ./cmd/formatter && \
    CGO_ENABLED=0 GOOS=linux go build -o bin/signer ./cmd/signer && \
    CGO_ENABLED=0 GOOS=linux go build -o bin/seed ./cmd/seed

# ============================================
# Stage 2: Runtime Image
# ============================================
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (curl for health checks)
RUN apk add --no-cache curl ca-certificates

# Copy binaries from builder
COPY --from=builder /app/bin ./bin

# Copy configs and data
COPY configs ./configs
COPY data ./data

# Make binaries executable
RUN chmod +x bin/*

# Default command - run seeder
ENTRYPOINT ["./bin/seed"]

