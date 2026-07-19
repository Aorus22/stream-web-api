FROM golang:1.24-bookworm AS builder

RUN apt-get update && apt-get install -y \
    gcc \
    libc-dev \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w' -o stream-web-api ./cmd/app

FROM alpine:3.19

RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    sqlite

WORKDIR /app

COPY --from=builder /app/stream-web-api .

RUN mkdir -p /app/data /app/torrent_data /app/hls_cache

EXPOSE 6432

CMD ["./stream-web-api"]
