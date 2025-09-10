# Build stage with optimizations
FROM --platform=$BUILDPLATFORM golang:1.25.0-alpine AS builder

# Install build dependencies
RUN apk add --update --no-cache ca-certificates git make

# Set up build environment with optimizations
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org
ENV GOCACHE=/go/cache
ENV GOMODCACHE=/go/mod

WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies with caching
RUN --mount=type=cache,target=/go/mod \
    --mount=type=cache,target=/go/cache \
    go mod download && go mod verify

# Copy source code (do this after mod download for better caching)
COPY . .

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH

# Build the binary with optimizations and caching
RUN --mount=type=cache,target=/go/mod \
    --mount=type=cache,target=/go/cache \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
    -trimpath \
    -ldflags="-w -s -extldflags '-static'" \
    -tags="netgo,osusergo" \
    -o engine \
    .

# Runtime stage - minimal Alpine image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --update --no-cache \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user for security
RUN addgroup -g 1000 apito && \
    adduser -D -s /bin/sh -u 1000 -G apito apito

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /build/engine .

# Copy required directories with proper ownership
COPY --chown=apito:apito plugins ./plugins/
COPY --chown=apito:apito keys ./keys/

# Make binary executable
RUN chmod +x ./engine

# Switch to non-root user
USER apito

# Set environment variables
ENV GOMEMLIMIT=1550MiB
ENV PORT=5050
ENV CGO_ENABLED=0

EXPOSE 5050

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD ["/bin/sh", "-c", "ps | grep -v grep | grep engine || exit 1"]

ENTRYPOINT ["./engine"]