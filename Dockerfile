# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --update --no-cache ca-certificates git

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary for the target architecture
# CGO_ENABLED=0 for static binary, GOOS and GOARCH are automatically set by buildx
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o engine .

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --update --no-cache ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1000 apito && \
    adduser -D -s /bin/sh -u 1000 -G apito apito

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /build/engine .

# Copy required directories
COPY --chown=apito:apito plugins ./plugins/
COPY --chown=apito:apito keys ./keys/

# Switch to non-root user
USER apito

# Set environment variables
ENV GOMEMLIMIT=1550MiB
ENV PORT=5050

EXPOSE 5050

ENTRYPOINT ["./engine"]