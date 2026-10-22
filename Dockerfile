# ── Stage 1: Build Vue frontend ─────────────────────────────────────────────
FROM node:20-alpine AS ui-builder

WORKDIR /app/ui

COPY ui/package*.json ./
RUN npm install

COPY ui/ ./

RUN npm run build


# ── Stage 2: Build Go binary ─────────────────────────────────────────────────
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOMAXPROCS=2 go build -p 2 -ldflags="-s -w" -o cloud-api main.go


# ── Stage 3: Go API runtime ──────────────────────────────────────────────────
FROM alpine:3.20 AS api

# rclone dipakai menyalin bukti foto ke Google Drive memakai remote yang sudah
# dikonfigurasi di host (berkas rclone.conf dipasang read-only lewat compose).
RUN apk add --no-cache tzdata ca-certificates rclone

WORKDIR /app

COPY --from=go-builder /app/cloud-api ./cloud-api

RUN mkdir -p uploads

EXPOSE 4000
CMD ["./cloud-api"]


# ── Stage 4: Nginx frontend ──────────────────────────────────────────────────
# SSL ditangani oleh Cloudflare — Nginx hanya perlu HTTP (port 80).
FROM nginx:alpine AS frontend

COPY --from=ui-builder /app/ui/dist /usr/share/nginx/html
COPY nginx/default.http-only.conf /etc/nginx/conf.d/default.conf

EXPOSE 80


# ── Stage 5: WhatsApp gateway (modul Go terpisah, butuh Go ≥ 1.26 untuk whatsmeow) ──
FROM golang:1.26-alpine AS wa-builder

WORKDIR /src

COPY wa-gateway/go.mod wa-gateway/go.sum ./
RUN go mod download

COPY wa-gateway/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /wa-gateway .


FROM alpine:3.20 AS wa-gateway

RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app

COPY --from=wa-builder /wa-gateway ./wa-gateway

EXPOSE 4100
CMD ["./wa-gateway"]

# Penghubung memakai standard library Go saja; pemasangan outlet tidak perlu
# membangun API cloud atau mengunduh seluruh dependensi backend.
FROM golang:1.24-alpine AS cctv-gateway-builder
WORKDIR /gateway
COPY go.mod ./
COPY internal/cctvrelay/ ./internal/cctvrelay/
COPY cmd/cctv-gateway/ ./cmd/cctv-gateway/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /cctv-gateway ./cmd/cctv-gateway

# Penghubung outlet: relay lokal + klien HTTPS keluar, tanpa VPN/port publik.
FROM alexxit/go2rtc:1.9.14 AS cctv-gateway
COPY --from=cctv-gateway-builder /cctv-gateway /usr/local/bin/cctv-gateway
COPY go2rtc/gateway.yaml /etc/cctv/go2rtc.yaml
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/cctv-gateway"]
