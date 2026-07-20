#!/bin/sh
set -e

# Seed persistent /data from the image-bundled database when missing.
# (A volume mount would otherwise hide files baked into the image.)
if [ ! -f /data/GeoLite2-City.mmdb ] && [ -f /app/data/GeoLite2-City.mmdb ]; then
	echo "seeding /data/GeoLite2-City.mmdb from image bundle"
	cp /app/data/GeoLite2-City.mmdb /data/GeoLite2-City.mmdb
	if [ -f /app/data/GeoLite2-City.mmdb.sha256 ]; then
		cp /app/data/GeoLite2-City.mmdb.sha256 /data/GeoLite2-City.mmdb.sha256
	fi
fi

exec ./open-geoip -c cfg.docker.json
