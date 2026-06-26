# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go module files first (layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 go build \
    -ldflags="-X main.Version=$(git describe --tags --always 2>/dev/null || echo dev) -s -w" \
    -o hunter cmd/hunter/main.go

# Runtime stage — distroless
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/hunter /hunter
COPY --from=builder /app/config/ /config

EXPOSE 8090

ENTRYPOINT ["/hunter"]
CMD ["--api", ":8090"]
