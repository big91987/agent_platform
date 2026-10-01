#!/usr/bin/env bash
# Install the local Python SDK once for CI adapters; no API Token is needed here.
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077
python3 -m venv .data/runner-venv
.data/runner-venv/bin/python -m pip install ./sdk/python
echo 'Runner Python: .data/runner-venv/bin/python'
