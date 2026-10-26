#!/usr/bin/env python3
"""Prepare one Run's pinned repository without copying host credentials or code."""

import argparse
import json
import os
import re
import subprocess
import tempfile
from pathlib import Path


def prepare(context, repository, workspace=Path("/workspace")):
    parameters = context["parameters"]
    commit = parameters["repository_commit"]
    base = parameters.get("review_base_commit", "")
    run = context["run_id"]
    if (
        not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository)
        or parameters["repository"] != repository
        or not re.fullmatch(r"[0-9a-f]{40}", commit)
        or (base and not re.fullmatch(r"[0-9a-f]{40}", base))
        or not re.fullmatch(r"[0-9a-f]{32}", run)
    ):
        raise ValueError("repository, pinned commit or Run does not match this entry")
    token = os.environ.get("GITHUB_TOKEN")
    if not token or "\x00" in token:
        raise ValueError("repository credential is unavailable")
    identity = {"repository": repository, "commit": commit, "run_id": run}
    if base:
        identity["review_base_commit"] = base
    marker = workspace / ".git/agent-platform-prepared.json"
    if any(workspace.iterdir()):
        if (
            marker.is_symlink()
            or not marker.is_file()
            or json.loads(marker.read_text()) != identity
        ):
            raise ValueError(
                "workspace has prior effects; inspect before explicit recovery"
            )
        print(json.dumps({"prepared": True, **identity, "reused": True}))
        return
    # The helper contains no secret value. Git sees the credential only through
    # the child environment, not the remote URL, argv, workspace or Git config.
    with tempfile.TemporaryDirectory(prefix="agent-platform-git-") as temp:
        askpass = Path(temp) / "askpass.py"
        askpass.write_text(
            '#!/usr/bin/env python3\nimport os,sys\nprint("x-access-token" if "username" in sys.argv[1].lower() else os.environ["GITHUB_TOKEN"])\n'
        )
        askpass.chmod(0o700)
        env = dict(
            os.environ,
            GIT_ASKPASS=str(askpass),
            GIT_TERMINAL_PROMPT="0",
            GIT_CONFIG_NOSYSTEM="1",
            GIT_CONFIG_GLOBAL="/dev/null",
        )

        def git(*args):
            subprocess.run(
                [
                    "git",
                    "-c",
                    "credential.helper=",
                    "-c",
                    "http.followRedirects=false",
                    "-c",
                    "core.hooksPath=/dev/null",
                    *args,
                ],
                cwd=workspace,
                env=env,
                check=True,
                timeout=480 if base and args[0] == "fetch" else 120,
            )

        git("init", "--quiet")
        git("remote", "add", "origin", "https://github.com/" + repository + ".git")
        # Fixed HEAD still needs reachable history for project regression gates
        # and OCR merge-base. Review preparation has a 600-second node budget;
        # allow its large-repository fetch up to 480 seconds without retries.
        git("fetch", "--quiet", "origin", commit, *([base] if base else []))
        actual = (
            subprocess.check_output(
                ["git", "rev-parse", commit + "^{commit}"],
                cwd=workspace,
                env=env,
                timeout=5,
            )
            .decode()
            .strip()
        )
        if actual != commit:
            raise ValueError("fetched source differs from frozen commit")
        git("checkout", "--quiet", "-b", "codex/run-" + run[:12], commit)
        marker.write_text(json.dumps(identity))
        marker.chmod(0o600)
    print(json.dumps({"prepared": True, **identity, "reused": False}))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", required=True)
    args = parser.parse_args()
    prepare(
        json.loads(Path(os.environ["WORKFLOW_INPUT_FILE"]).read_text()), args.repository
    )
