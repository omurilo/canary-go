.PHONY: build run client test db-up db-down smoke tidy

build:
	go build -o bin/canary ./cmd/canary
	go build -o bin/canary-client ./cmd/canary-client

test:
	go test ./...

tidy:
	go mod tidy

db-up:
	docker compose -f deploy/docker-compose.yml up -d

db-down:
	docker compose -f deploy/docker-compose.yml down

# Run the server (applies schema + seeds the test account).
run: build
	./bin/canary -config config.lua -schema schema/mysql.sql -scripts scripts -migrate -seed

# Just the login/game client against a running server.
client: build
	./bin/canary-client -account god -password god -rsa key.pem

# Full end-to-end check: brings the server up, runs the client, tears down.
smoke: build
	./scripts/smoke.sh
