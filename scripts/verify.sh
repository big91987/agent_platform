#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
test -z "$(gofmt -l cmd internal web examples/pipeline-tool)"
for script in scripts/*.sh examples/gitlab/prepare.sh examples/github/install-tooling.sh; do
  bash -n "$script"
done
node --check web/workflow-model.js
node --check web/connectors.js
node --check web/workflows.js
node --check web/workflow-runs.js
node --test web/workflow-model_test.js
node --check web/app.js
node --check web/markdown.js
node --check web/transcript.js
node --test web/request_test.js web/markdown_test.js web/app_test.js web/transcript_test.js
NODE_PATH=examples/github/tooling/full_harness/browser/node_modules node --test web/live_browser_test.cjs
ruff check sdk/python examples
ruff format --check sdk/python examples
PYTHONPATH=sdk/python python3 -m unittest discover -s sdk/python/tests -v
PYTHONPATH=sdk/python:examples/github .data/runner-venv/bin/python -m unittest discover -s examples/github -p '*_test.py' -v
python3 -m unittest discover -s examples/github/local-preview/tests -p '*_test.py' -v
python3 -m unittest discover -s examples/platform-workflows -p '*_test.py' -v
go vet ./...
go test -race ./... -timeout 120s
go build -o bin/agent-platform ./cmd/agent-platform
