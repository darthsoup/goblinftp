# syntax=docker/dockerfile:1

# ── Stage 1: Build Nuxt SPA ────────────────────────────────────────────────────
FROM node:20-alpine AS frontend-builder
WORKDIR /build/frontend
COPY frontend/package*.json ./
RUN npm ci --prefer-offline
COPY frontend/ ./
RUN npm run generate

# ── Stage 2: Build Go binary ───────────────────────────────────────────────────
FROM golang:1.26-alpine AS backend-builder
WORKDIR /build/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/gftp ./cmd/gftp

# ── Stage 3: Final image ───────────────────────────────────────────────────────
FROM caddy:2-alpine

RUN mkdir -p /app/public /app/data /app/storage

COPY --from=frontend-builder /build/frontend/.output/public /app/public
COPY --from=backend-builder /app/gftp /app/gftp
COPY docker/Caddyfile /etc/caddy/Caddyfile
COPY docker/entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

EXPOSE 80
ENTRYPOINT ["/entrypoint.sh"]
