# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go module files first (layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.Version=$(git describe --tags --always 2>/dev/null || echo dev) -s -w" \
    -o bin/hunter ./cmd/hunter

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -h /app hunter

WORKDIR /app

# Copy binary and configs
COPY --from=builder /app/bin/hunter /app/hunter
COPY --from=builder /app/configs/ /app/configs/

# Create data directory
RUN mkdir -p /app/data && chown -R hunter:hunter /app

USER hunter

# Environment
ENV HUNTER_DATA_DIR=/app/data
ENV HUNTER_CONFIG_DIR=/app/configs

# Health check
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

EXPOSE 8080

ENTRYPOINT ["/app/hunter"]
CMD ["--api", ":8080"]
