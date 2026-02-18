# Multi-stage build for letheClaw API

# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy dependency manifest only (NOT go.sum — it's incomplete and will be regenerated)
COPY go.mod ./
RUN go mod download

# Copy source
COPY main.go ./
COPY handlers ./handlers/
COPY models ./models/
COPY services ./services/

# Regenerate go.sum with all transitive dependencies resolved from source imports
RUN go mod tidy

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o letheclaw-api .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/letheclaw-api .

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
CMD ["./letheclaw-api"]
