#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CANARY_GO_DIR="$DIR/.."
CANARY_DIR="$DIR/../.."

PROTO_DIR="$CANARY_DIR/src/protobuf"
OUT_DIR="$CANARY_GO_DIR/internal/protobuf/appearances"

mkdir -p "$OUT_DIR"

protoc --proto_path="$PROTO_DIR" \
       --go_out="$OUT_DIR" \
       --go_opt=paths=source_relative \
       --go_opt=Mappearances.proto=github.com/opentibiabr/canary-go/internal/protobuf/appearances \
       "$PROTO_DIR/appearances.proto"

echo "Successfully generated appearances protobuf for Go."
