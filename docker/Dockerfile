# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
# - ca-certificates: for HTTPS
# - tzdata: for timezone support
# - poppler-utils: for pdftotext
# - chromium: for headless browser
# - nss, freetype, harfbuzz: chromium dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    poppler-utils \
    chromium \
    nss \
    freetype \
    harfbuzz

# Set Chrome path for chromedp
ENV CHROME_PATH=/usr/bin/chromium-browser

# Copy binary from builder
COPY --from=builder /server /app/server

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Run the server
CMD ["/app/server"]
