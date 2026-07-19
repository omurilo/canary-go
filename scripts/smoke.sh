#!/usr/bin/env bash
# End-to-end smoke test: start MariaDB, run the server (schema + seed) on the
# host, connect the client, perform login + enter-world + walk/chat/ping/logout,
# then shut down. Verifies the full protocol + MariaDB persistence.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "== building =="
go build -o bin/canary ./cmd/canary
go build -o bin/canary-client ./cmd/canary-client

echo "== starting MariaDB =="
docker compose -f deploy/docker-compose.yml up -d db >/dev/null
for _ in $(seq 1 30); do
  if [ "$(docker inspect -f '{{.State.Health.Status}}' canary-go-db 2>/dev/null)" = "healthy" ]; then
    break
  fi
  sleep 2
done

echo "== starting server =="
./bin/canary -config config.lua -schema schema/mysql.sql -scripts scripts -migrate -seed \
  > /tmp/canary-server.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null || true' EXIT
sleep 3

echo "== running client =="
./bin/canary-client -account god -password god -rsa key.pem

echo "== server log =="
tail -n 8 /tmp/canary-server.log
echo "== smoke test passed =="
