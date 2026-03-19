FROM golang:1.23-alpine AS builder

WORKDIR /build

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o mtproxy-panel .

# ── Runtime ──
FROM alpine:3.20

RUN apk add --no-cache docker-cli ca-certificates

WORKDIR /app

COPY --from=builder /build/mtproxy-panel .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static

RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./mtproxy-panel"]
