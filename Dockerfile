# Go API Base - Multi-stage Dockerfile
# Builder stage for compiling the binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO disabled for static linking
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api

# Runtime stage with minimal alpine image
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS and tzdata for timezone support
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy binary from builder stage
COPY --from=builder /app/bin/api .

# Set ownership to non-root user
RUN chown -R appuser:appgroup /app

# Expose the API port
EXPOSE 8080

# Switch to non-root user
USER appuser:appgroup

# Set entrypoint and default command
ENTRYPOINT ["./api"]
CMD ["serve"]