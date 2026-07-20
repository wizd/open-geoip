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

RUN apk add --no-cache ca-certificates tzdata wget gzip \
	&& adduser -D -H -u 1000 appuser \
	&& mkdir -p /data /app/logs /app/data \
	&& chown -R appuser:appuser /data /app

WORKDIR /app

# Bundle GeoLite2-City from community mirror (no MaxMind license / no CN 451).
# Source: https://github.com/wp-statistics/GeoLite2-City
ARG GEOLITE2_CITY_URL=https://cdn.jsdelivr.net/npm/geolite2-city/GeoLite2-City.mmdb.gz
RUN wget -qO /tmp/GeoLite2-City.mmdb.gz "${GEOLITE2_CITY_URL}" \
	&& gzip -dc /tmp/GeoLite2-City.mmdb.gz > /app/data/GeoLite2-City.mmdb \
	&& wget -qS --spider "${GEOLITE2_CITY_URL}" 2>&1 \
		| sed -n 's/^  [Ee][Tt][Aa][Gg]: //p' | tr -d '\r' | head -1 \
		> /app/data/GeoLite2-City.mmdb.sha256 \
	&& rm -f /tmp/GeoLite2-City.mmdb.gz \
	&& chown -R appuser:appuser /app/data

COPY --from=builder /out/open-geoip /app/open-geoip
COPY --from=builder /src/assets /app/assets
COPY --from=builder /src/templates /app/templates
COPY --from=builder /src/cfg.docker.json /app/cfg.docker.json
COPY docker/entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/entrypoint.sh && chown appuser:appuser /app/entrypoint.sh

USER appuser

ENV PORT=8080
ENV GEOLITE2_CITY_URL=https://cdn.jsdelivr.net/npm/geolite2-city/GeoLite2-City.mmdb.gz

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
	CMD wget -qO- http://127.0.0.1:${PORT}/version || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]
