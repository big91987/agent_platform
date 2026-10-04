#!/usr/bin/env bash
# Install this example's locked tools; do not modify the product repository.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"
bash scripts/setup-runner.sh
.data/runner-venv/bin/python -m pip install -r examples/github/requirements.txt
go build -o bin/pipeline-tool ./examples/pipeline-tool
npm ci --ignore-scripts --prefix examples/github/quality
PYTHONPATH="$root/examples/github/tooling" .data/runner-venv/bin/python - <<'PY'
from pathlib import Path
from full_harness.browser import prepare
prepare(Path('examples/github/tooling').resolve())
PY
