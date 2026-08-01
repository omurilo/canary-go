.PHONY: build run client test up down fup fdown logs smoke tidy

c ?=

# Stamp the build so the running server can say which commit it is, instead of it
# having to be inferred from timestamps across machines.
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT_AT := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.buildCommit=$(COMMIT) -X main.buildTime=$(BUILT_AT)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/canary ./cmd/canary
	go build -o bin/canary-client ./cmd/canary-client

test:
	go test ./...

tidy:
	go mod tidy

up:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --build

down:
	docker compose -f deploy/docker-compose.yml down

fup:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --profile full --build

fdown:
	docker compose -f deploy/docker-compose.yml down --profile full

logs:
	docker compose -f deploy/docker-compose.yml logs -f $(c)

# Run the server (applies schema + seeds the test account).
run: build
	./bin/canary -config config.lua -schema schema/schema.sql -scripts scripts -migrate -seed

# Just the login/game client against a running server.
client: build
	./bin/canary-client -account god -password god -rsa key.pem

# Full end-to-end check: brings the server up, runs the client, tears down.
smoke: build
	./scripts/smoke.sh
