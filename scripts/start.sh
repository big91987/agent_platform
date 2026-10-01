#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p bin .data
if [ -f .data/platform.pid ] && kill -0 "$(cat .data/platform.pid)" 2>/dev/null; then
  command=$(ps -p "$(cat .data/platform.pid)" -o command=)
  case "$command" in
    *"$(pwd)/bin/agent-platform"*) echo 'Agent Platform is already running. Open http://127.0.0.1:8788'; exit 0 ;;
    *) echo 'Saved PID belongs to another process; starting the platform without touching that process' ;;
  esac
fi
go build -o bin/agent-platform ./cmd/agent-platform
python3 - <<'PY'
from pathlib import Path
import subprocess
root=Path.cwd()
with (root/'.data/platform.log').open('ab') as log:
    p=subprocess.Popen([str(root/'bin/agent-platform'),'-data',str(root/'.data')],cwd=root,stdout=log,stderr=log,start_new_session=True)
(root/'.data/platform.pid').write_text(str(p.pid)+'\n')
PY
for attempt in $(seq 1 40); do
  if curl --fail --silent http://127.0.0.1:8788/api/health >/dev/null; then
    echo 'Agent Platform: http://127.0.0.1:8788'
    echo 'First-start login: admin/admin; existing account passwords are preserved'
    exit 0
  fi
  sleep 0.25
done
tail -20 .data/platform.log
exit 1
