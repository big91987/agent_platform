#!/usr/bin/env bash
set -euo pipefail
source_root=$(cd "$(dirname "$0")/../.." && pwd)
revision=${1:?Usage: build.sh SOURCE_REVISION NEW_BUILD_DIRECTORY}
output_dir=${2:?Usage: build.sh SOURCE_REVISION NEW_BUILD_DIRECTORY}
if [[ ! "$revision" =~ ^[0-9a-f]{40}$ ]] || [[ -e "$output_dir" ]]; then
  echo 'Use an exact source revision and a new build directory; previous evidence is retained.' >&2
  exit 1
fi
if [[ $(uname -s) != Linux ]]; then
  echo 'Build the Linux CGO binary on the deployment host, not with CGO disabled.' >&2
  exit 1
fi
umask 077
mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd)
cd "$source_root"
python3 - "$output_dir/source.sha256" <<'PY'
import hashlib,sys
from pathlib import Path
paths=[Path('go.mod'),Path('go.sum')]
for root in ('cmd','internal','web'):
    paths.extend(p for p in Path(root).rglob('*') if p.is_file())
with Path(sys.argv[1]).open('x') as output:
    for path in sorted(paths):
        if path.is_symlink():raise RuntimeError('source symlinks are not accepted')
        output.write(hashlib.sha256(path.read_bytes()).hexdigest()+'  '+str(path)+'\n')
PY
export GOTOOLCHAIN=go1.25.6 GOWORK=off CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GOFLAGS=-mod=readonly
# Dependencies remain verified by go.sum and the standard module checksum service.
# Existing trusted runner proxy settings are inherited, never printed or stored.
go version > "$output_dir/toolchain.txt"
go mod verify > "$output_dir/module-verification.log" 2>&1
go build -trimpath -buildvcs=false -ldflags "-X agentplatform/internal/platform.BuildRevision=$revision" -o "$output_dir/agent-platform" ./cmd/agent-platform > "$output_dir/build.log" 2>&1
"$output_dir/agent-platform" --version > "$output_dir/version.json"
python3 - "$output_dir/version.json" "$revision" <<'PY'
import json,sys
from pathlib import Path
if json.loads(Path(sys.argv[1]).read_text()).get('revision')!=sys.argv[2]:raise RuntimeError('built binary revision differs')
PY
sha256sum "$output_dir/agent-platform" > "$output_dir/binary.sha256"
printf 'PLATFORM_BUILD_PASS %s\n' "$revision"
