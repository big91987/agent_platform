#!/usr/bin/env bash
# Run before handing the persistent project directory to the Agent.
set -euo pipefail
umask 077
: "${PLATFORM_WORKSPACE_PATH:?Set the prepared path on the shared mount}"
: "${CI_PROJECT_DIR:?GitLab checkout is required}"
: "${CI_PROJECT_URL:?Use the credential-free project URL}"
: "${CI_COMMIT_SHA:?The input commit is required}"
: "${CI_PIPELINE_ID:?The pipeline task key is required}"
case "$PLATFORM_WORKSPACE_PATH" in
  /*) ;;
  *) echo 'PLATFORM_WORKSPACE_PATH must be absolute' >&2; exit 1 ;;
esac
expected=$(printf '%s\n%s' "$CI_PROJECT_URL" "$CI_COMMIT_SHA")
marker="$PLATFORM_WORKSPACE_PATH/.agent-platform-input"
if [ -e "$PLATFORM_WORKSPACE_PATH" ]; then
  if [ -f "$marker" ] && [ "$(cat "$marker")" = "$expected" ] &&
    [ -d "$PLATFORM_WORKSPACE_PATH/.git" ] &&
    git -C "$PLATFORM_WORKSPACE_PATH" rev-parse --verify HEAD >/dev/null 2>&1; then
    echo 'Keeping the existing prepared workspace; no checkout, reset or cleanup'
    exit 0
  fi
  echo 'Workspace exists without the matching input checkpoint; refusing to overwrite it' >&2
  exit 1
fi
mkdir -p "$(dirname "$PLATFORM_WORKSPACE_PATH")"
# A standalone clone keeps .git valid after the Runner checkout is cleaned.
git clone --no-hardlinks -- "$CI_PROJECT_DIR" "$PLATFORM_WORKSPACE_PATH"
git -C "$PLATFORM_WORKSPACE_PATH" remote set-url origin "$CI_PROJECT_URL"
git -C "$PLATFORM_WORKSPACE_PATH" checkout -b "codex/pipeline-$CI_PIPELINE_ID" "$CI_COMMIT_SHA"
# Owners can add explicit copies of selected earlier-job artifacts here.
if [ -d "$CI_PROJECT_DIR/handoff" ]; then
  mkdir -p "$PLATFORM_WORKSPACE_PATH/handoff"
  cp -R "$CI_PROJECT_DIR/handoff/." "$PLATFORM_WORKSPACE_PATH/handoff/"
fi
printf '%s\n' "$expected" > "$marker"
echo 'Prepared persistent project directory; Runner owns branch initialization'
