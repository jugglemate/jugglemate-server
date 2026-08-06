#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cd "$ROOT_DIR/console/web"
./node_modules/.bin/vp build

cd "$ROOT_DIR"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./jchat-ai main.go

mkdir -p "$ROOT_DIR/build"
mv jchat-ai build/
