# ============================================
# Stage 1: Build Go Binaries
# ============================================
FROM golang:1.23-alpine AS builder

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
    CGO_ENABLED=0 GOOS=linux go build -o bin/signer ./cmd/signer

# ============================================
# Stage 2: Runtime Image
# ============================================
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache curl bash

# Copy binaries from builder
COPY --from=builder /app/bin ./bin

# Copy configs and data
COPY configs ./configs
COPY data ./data

# Copy seed script
COPY scripts/seed.sh ./seed.sh

# Make binaries and scripts executable
RUN chmod +x bin/* seed.sh

# Default command - run seeder
ENTRYPOINT ["./seed.sh"]
