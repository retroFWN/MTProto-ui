FROM golang:1.23-alpine AS builder

WORKDIR /build

RUN apk add --no-cache gcc musl-dev

COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o mtproxy-panel .

# ── Runtime ──
FROM alpine:3.20

RUN apk add --no-cache docker-cli ca-certificates python3 py3-pip

WORKDIR /app

COPY --from=builder /build/mtproxy-panel .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static
COPY --from=builder /build/bot ./bot

RUN pip3 install --no-cache-dir --break-system-packages -r bot/requirements.txt
RUN mkdir -p /app/data

EXPOSE 8080 80 443

CMD ["./mtproxy-panel"]
