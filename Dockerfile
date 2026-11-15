# Этап сборки
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /parser ./cmd/parser

# Финальный образ: Chromium нужен для chromedp
FROM alpine:3.20
RUN apk add --no-cache chromium ca-certificates font-noto
WORKDIR /data
COPY --from=builder /parser /usr/local/bin/parser
COPY shops.yaml /etc/parser/shops.yaml
ENV SHOPS_CONFIG=/etc/parser/shops.yaml
# CSV пишется в /data — смонтируйте туда папку: -v "$PWD:/data"
ENTRYPOINT ["parser"]
