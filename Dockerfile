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
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cloud-api main.go


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
