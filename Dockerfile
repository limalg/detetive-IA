# ==============================================================================
# Detetive IA - Dockerfile Multi-Stage Otimizado (<30MB Image, <60MB RAM)
# ==============================================================================

# Stage 1: Build
FROM golang:alpine AS builder

WORKDIR /src

# Dependências básicas de build
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compilação sem CGO com strip de símbolos para imagem mínima
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o /bin/detetive-ia ./cmd/server

# Stage 2: Imagem Final Enxuta
FROM alpine:3.21

# Certificados SSL para chamadas HTTPS à IA (Gemini)
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Diretório para persistência do SQLite
RUN mkdir -p /data && chown -R appuser:appgroup /data /app

COPY --from=builder /bin/detetive-ia /app/detetive-ia

USER appuser

EXPOSE 8080

ENV PORT=8080 \
    DB_PATH=/data/detetive.db

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["/app/detetive-ia"]
