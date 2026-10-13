#!/usr/bin/env bash
set -euo pipefail
source_dir=$(cd "$(dirname "$0")" && pwd)
output_dir=${1:?Usage: build.sh NEW_OUTPUT_DIRECTORY}
if [[ -e "$output_dir" ]]; then
  echo 'Build directory exists; inspect the previous attempt instead of overwriting it.' >&2
  exit 1
fi
umask 077
mkdir -p "$output_dir/context"
for file in Dockerfile package.json package-lock.json check.py .dockerignore; do
  cp "$source_dir/$file" "$output_dir/context/$file"
done
(cd "$output_dir/context" && sha256sum Dockerfile package.json package-lock.json check.py .dockerignore) > "$output_dir/source.sha256"
context_sha=$(sha256sum "$output_dir/source.sha256" | cut -d' ' -f1)
image_tag="agent-platform-runtime:${context_sha:0:12}"
# Docker does not inherit the trusted runner's proxy automatically. Predefined
# build arguments are excluded from image history; never put proxy values in ENV.
build_args=()
for proxy_name in HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy; do
  alternate_name=${proxy_name,,}
  if [[ "$alternate_name" == "$proxy_name" ]]; then alternate_name=${proxy_name^^}; fi
  proxy_value=${!proxy_name:-${!alternate_name:-}}
  if [[ -n "$proxy_value" ]]; then
    export "$proxy_name=$proxy_value"
    build_args+=(--build-arg "$proxy_name")
  fi
done
docker build "${build_args[@]}" --platform linux/amd64 --iidfile "$output_dir/image-id" --tag "$image_tag" "$output_dir/context" > "$output_dir/build.log" 2>&1
image_id=$(cat "$output_dir/image-id")
timeout --signal=TERM --kill-after=10s 300s docker run --rm --network none --pids-limit 256 --memory 2g --cpus 2 --security-opt no-new-privileges "$image_id" python3 /opt/agent-runtime/check.py > "$output_dir/check.jsonl" 2> "$output_dir/check.log"
printf 'RUNTIME_BUILD_PASS %s\n' "$image_id"
