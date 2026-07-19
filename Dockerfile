# Build context must be the repository root so the parent data/ directories are
# available to mount at runtime. Build:
#   docker build -f canary-go/Dockerfile -t canary-go .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY / ./
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
CMD ["-config", "config.lua", "-schema", "schema/mysql.sql", "-scripts", "scripts", "-migrate", "-seed"]
