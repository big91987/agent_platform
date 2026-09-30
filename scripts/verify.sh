#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
test -z "$(gofmt -l cmd internal web)"
node --check web/app.js
node --check web/markdown.js
node --test web/request_test.js web/markdown_test.js
go vet ./...
go test -race ./... -timeout 120s
go build -o bin/agent-platform ./cmd/agent-platform
