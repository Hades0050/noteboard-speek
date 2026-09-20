# syntax=docker/dockerfile:1

FROM oven/bun:1-alpine AS web
WORKDIR /app
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

FROM golang:1.27-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /app/.output/public ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/noteboard ./cmd/noteboard

FROM alpine:3.22
# ffmpeg нужен только режиму WHISPER_BIN (локальный whisper.cpp), wget — healthcheck
RUN apk add --no-cache ca-certificates ffmpeg tzdata wget \
 && adduser -D -u 10001 app \
 && mkdir -p /data && chown app:app /data
COPY --from=server /out/noteboard /usr/local/bin/noteboard
ENV ADDR=:8080 \
    DATA_DIR=/data \
    TRANSCRIBE_LANG=ru
USER app
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1
ENTRYPOINT ["noteboard"]
