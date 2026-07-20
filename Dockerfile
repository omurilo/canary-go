# Build context must be the canary-go directory. Build:
#   cd canary-go
#   docker build -t canary-go .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# Install protoc and go protobuf compiler
RUN apk add --no-cache protobuf bash
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

COPY / ./
# Generate protobufs for appearances
RUN ./scripts/generate_appearances.sh

RUN CGO_ENABLED=0 go build -trimpath -o /out/canary ./cmd/canary

FROM alpine:3.20
RUN adduser -D -u 10001 canary
WORKDIR /app
COPY --from=build /out/canary /app/canary
COPY config.lua /app/config.lua
COPY key.pem /app/key.pem
COPY schema /app/schema
COPY scripts /app/scripts
# data/, data-canary/, data-otservbr-global/ are mounted at runtime (see compose).
USER canary
EXPOSE 7171 7172
ENTRYPOINT ["/app/canary"]
CMD ["-config", "config.lua", "-schema", "schema/mysql.sql", "-scripts", "scripts", "-appearances", "data/items/appearances.dat", "-migrate", "-seed"]
