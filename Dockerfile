FROM chainguard/go:latest AS builder

WORKDIR /app

# Download deps
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" -o /usr/local/bin/ai-app ./cmd

# ─────────── Final Stage ────────────
FROM debian:bullseye-slim

# install certs + curl + jq
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl jq \
    && rm -rf /var/lib/apt/lists/*

# Copy compiled binary
COPY --from=builder /usr/local/bin/ai-app /usr/local/bin/ai-app

# Copy templates and static assets
COPY --from=builder /app/web/templates /app/web/templates

# env defaults (override at runtime)
ENV DB_PATH="./data.db" \
    DB_SCHEMA="./pkg/database/schema.sql" \
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
