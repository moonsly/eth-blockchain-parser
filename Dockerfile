# Multi-stage build for eth-blockchain-parser
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files first (for better caching)
COPY go.mod go.sum ./

# Download dependencies (this layer will be cached unless go.mod/go.sum changes)
RUN go mod download

# Copy only necessary source files to avoid cache invalidation
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# Build both binaries
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o infura-parser ./cmd/infura-parser/
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o server-run ./cmd/server-run/

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    bash \
    shadow \
    sqlite \
    tzdata \
    curl \
    jq \
    dcron \
    su-exec \
    sudo \
    && rm -rf /var/cache/apk/*

# Create app directory and user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN mkdir -p /app/data /app/logs /var/log/eth_parser
RUN chown -R appuser:appgroup /app /var/log/eth_parser
RUN usermod -G wheel -a appuser

# Set working directory
WORKDIR /app

# Copy built binaries from builder stage
COPY --from=builder /app/infura-parser .
COPY --from=builder /app/server-run .

# Copy script templates
COPY docker/start.sh /app/start.sh
COPY docker/run_parser.sh /app/run_parser.sh

# Make scripts executable
RUN chmod +x /app/start.sh /app/run_parser.sh

# Change ownership
RUN chown -R appuser:appgroup /app

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f -u "$SERVER_USERNAME:$SERVER_PASSWORD" http://localhost:$SERVER_PORT/api/transactions?limit=1 || exit 1

# Expose API port
EXPOSE 8015

# Set default environment variables
ENV INFURA_API_KEY=${INFURA_API_KEY} \
    INFURA_NETWORK=mainnet \
    DB_PATH=/app/data/blockchain.db \
    CSV_PATH=/app/data/whale_txns.csv \
    LAST_BLOCK_PATH=/app/data/last_block.dat \
    SERVER_PORT=8015 \
    SERVER_HOST=0.0.0.0 \
    SERVER_USERNAME=admin \
    SERVER_PASSWORD=password123

# Volume for persistent data
VOLUME ["/app/data", "/var/log/eth_parser"]

# Default command
CMD ["/app/start.sh"]
