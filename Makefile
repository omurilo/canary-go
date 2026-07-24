.PHONY: build run client test docker-up docker-down docker-fup docker-fdown smoke tidy

build:
	go build -o bin/canary ./cmd/canary
	go build -o bin/canary-client ./cmd/canary-client

test:
	go test ./...

tidy:
	go mod tidy

docker-up:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --build

docker-down:
	docker compose -f deploy/docker-compose.yml down

docker-fup:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --profile full --build

docker-fdown: docker compose -f deploy/docker-compose.yml down --profile full

# Run the server (applies schema + seeds the test account).
run: build
	./bin/canary -config config.lua -schema schema/mysql.sql -scripts scripts -migrate -seed

# Just the login/game client against a running server.
client: build
	./bin/canary-client -account god -password god -rsa key.pem

# Full end-to-end check: brings the server up, runs the client, tears down.
smoke: build
	./scripts/smoke.sh
