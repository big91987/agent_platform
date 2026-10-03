#!/usr/bin/env bash
# Local integration credentials never enter model arguments or stdout.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
export PATH="/opt/homebrew/bin:$PATH"
export PIPELINE_TOKEN
PIPELINE_TOKEN=$(gh auth token --hostname github.com)
exec "$root/bin/pipeline-tool" "$@"
