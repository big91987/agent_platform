#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
test -z "$(gofmt -l cmd internal web)"
for script in scripts/*.sh examples/gitlab/prepare.sh; do
  bash -n "$script"
done
node --check web/app.js
node --check web/markdown.js
node --test web/request_test.js web/markdown_test.js web/app_test.js
ruff check sdk/python examples
ruff format --check sdk/python examples
PYTHONPATH=sdk/python python3 -m unittest discover -s sdk/python/tests -v
go vet ./...
go test -race ./... -timeout 120s
go build -o bin/agent-platform ./cmd/agent-platform
