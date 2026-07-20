# Build stage
FROM golang:1.19-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/open-geoip .

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget \
	&& adduser -D -H -u 1000 appuser \
	&& mkdir -p /data /app/logs \
	&& chown -R appuser:appuser /data /app

WORKDIR /app

COPY --from=builder /out/open-geoip /app/open-geoip
COPY --from=builder /src/assets /app/assets
COPY --from=builder /src/templates /app/templates
COPY --from=builder /src/cfg.docker.json /app/cfg.docker.json

USER appuser

ENV PORT=8080

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
	CMD wget -qO- http://127.0.0.1:${PORT}/version || exit 1

CMD ["./open-geoip", "-c", "cfg.docker.json"]
