#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [ ! -f .data/platform.pid ]; then echo 'Not running'; exit 0; fi
pid=$(cat .data/platform.pid)
if ! kill -0 "$pid" 2>/dev/null; then rm .data/platform.pid; echo 'Not running'; exit 0; fi
command=$(ps -p "$pid" -o command=)
case "$command" in
  *"$(pwd)/bin/agent-platform"*) kill -TERM "$pid" ;;
  *) echo 'PID belongs to another process; refusing to stop it'; exit 1 ;;
esac
for attempt in $(seq 1 80); do
  if ! kill -0 "$pid" 2>/dev/null; then rm .data/platform.pid; echo 'Stopped'; exit 0; fi
  sleep 0.25
done
echo 'Still stopping; inspect .data/platform.log'
exit 1
