FROM golang:1.24-bullseye AS builder

# Install CGO dependencies for sqlite3
RUN apt-get update && \
    apt-get install -y --no-install-recommends gcc libsqlite3-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Download deps
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN CGO_ENABLED=1 GOOS=linux \
    go build -ldflags="-s -w" -o /usr/local/bin/ai-app ./cmd

# ─────────── Final Stage ────────────
FROM debian:bullseye-slim

# install certs + curl + jq
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates sqlite3 libsqlite3-0 curl jq \
    && rm -rf /var/lib/apt/lists/*

# Copy compiled binary
COPY --from=builder /usr/local/bin/ai-app /usr/local/bin/ai-app

# Copy templates, static assets and db schema
COPY --from=builder /app/web/templates /app/web/templates
COPY --from=builder /app/pkg/database/schema.sql  /schema.sql

# Set working directory for runtime
WORKDIR /app

# env defaults (override at runtime)
ENV DB_PATH="./data.db" \
    DB_SCHEMA="./schema.sql" \
    OPENAI_API_KEY="" \
    PERMIT_API_KEY="" \
    PERMIT_PDP_URL="" \
    SESSION_SECRET="" \
    COOKIE_DOMAIN="localhost" \
    PORT=${PORT:-8080}

EXPOSE 8080

# healthcheck for orchestrators
HEALTHCHECK --interval=30s --timeout=5s \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/ai-app"]
