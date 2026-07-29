# Build context must be the canary-go directory. Build:
#   cd canary-go
#   docker build -t canary-go .
FROM golang:1.25-alpine AS build
WORKDIR /src

# 1. Instala dependências do sistema e ferramentas primeiro.
# Elas raramente mudam, então o cache dessa camada será preservado
# mesmo se o go.mod ou código-fonte mudar.
RUN apk add --no-cache protobuf bash && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# 2. Copia os arquivos de módulo e baixa as dependências Go.
# Usamos cache mount para acelerar o download e reuso de pacotes.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 3. Copia o restante do código.
COPY . .

# 4. Gera os protobufs.
RUN ./scripts/generate_appearances.sh

# 5. Realiza o build utilizando cache mounts do Go.
# Isso garante que recompilações (quando você muda apenas um arquivo .go)
# sejam extremamente rápidas.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /out/canary ./cmd/canary

# --- Estágio Final ---
FROM alpine:3.20

RUN adduser -D -u 10001 canary
WORKDIR /app

# Copia pastas estáticas e o binário gerado primeiro.
COPY schema /app/schema
COPY scripts /app/scripts
COPY --from=build /out/canary /app/canary

# Copia arquivos de configuração por último, pois são os mais
# propensos a sofrerem alterações de última hora durante testes locais.
COPY config.lua /app/config.lua
COPY key.pem /app/key.pem

USER canary
EXPOSE 7171 7172

ENTRYPOINT ["/app/canary"]
CMD ["-appearances", "data/items/appearances.dat", "-migrate", "-seed"]
