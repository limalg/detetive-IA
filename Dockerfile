# Detiv + Banco 3D: um único binário Go (web embutido via go:embed), imagem final alpine não-root.
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/detetive-ia ./cmd/server \
 && mkdir -p /out/data

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/detetive-ia /app/detetive-ia
# /app/data é o volume: SQLite, banco3d/rooms.json, banco3d_rooms.json e cache de lugares (caminhos relativos "data/...")
COPY --from=builder --chown=app:app /out/data /app/data
USER app
ENV PORT=8080 DB_PATH=/app/data/detetive.db BANCO3D_CACHE_FILE=/app/data/banco3d_places.json
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${PORT}/" && wget -q -O /dev/null "http://127.0.0.1:${PORT}/banco/" || exit 1
ENTRYPOINT ["/app/detetive-ia"]
