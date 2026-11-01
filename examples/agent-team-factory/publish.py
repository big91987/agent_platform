#!/usr/bin/env python3
"""Publish a verified experiment bundle using credentials on the control host."""

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
import time
from pathlib import Path
from urllib.parse import urlsplit

from git_workspace import validate_experiment_branch


def publish(bundle, expected_sha, remote, branch, head, github_auth=False):
    validate_experiment_branch(branch)
    if not re.fullmatch(r"[a-f0-9]{40}", head) or not re.fullmatch(
        r"[a-f0-9]{64}", expected_sha
    ):
        raise ValueError("exact integration HEAD and bundle SHA-256 are required")
    path = Path(bundle).resolve(strict=True)
    payload = path.read_bytes()
    if hashlib.sha256(payload).hexdigest() != expected_sha:
        raise ValueError("bundle SHA-256 mismatch")
    url = urlsplit(remote)
    if url.scheme and (
        url.scheme != "https"
        or not url.hostname
        or url.username
        or url.password
        or url.query
        or url.fragment
    ):
        raise ValueError(
            "use HTTPS without embedded credentials, or a local Git remote"
        )
    if github_auth and (url.scheme != "https" or url.hostname != "github.com"):
        raise ValueError("GitHub authentication is only for https://github.com")
    if any(ord(c) <= 32 for c in remote):
        raise ValueError("remote contains whitespace/control characters")
    receipts = path.parent / "publish-receipts"
    receipts.mkdir(mode=0o700, exist_ok=True)
    receipt_path = receipts / (str(time.time_ns()) + ".json")
    receipt = {
        "branch": branch,
        "head": head,
        "remote": remote,
        "bundle_sha256": expected_sha,
        "started": time.time(),
        "status": "failed",
        "commands": [],
    }
    phase = "prepare"

    def save():
        temporary = receipt_path.with_suffix(".pending")
        with temporary.open("w") as output:
            json.dump(receipt, output, ensure_ascii=False, indent=2)
            output.flush()
            os.fsync(output.fileno())
        temporary.chmod(0o600)
        temporary.replace(receipt_path)

    def run(*args, cwd=None):
        argv = ["git"]
        if github_auth:
            # Let Git ask the existing trusted control-host gh session. Neither
            # the token nor authentication files are copied to the executor.
            argv += [
                "-c",
                "credential.helper=",
                "-c",
                "credential.helper=!gh auth git-credential",
            ]
        argv += list(map(str, args))
        try:
            result = subprocess.run(
                argv,
                cwd=cwd,
                capture_output=True,
                text=True,
                env={**os.environ, "GIT_TERMINAL_PROMPT": "0"},
                timeout=90,
            )
        except subprocess.TimeoutExpired as error:
            receipt["commands"].append({"args": list(map(str, args)), "timeout": True})
            raise RuntimeError(
                "Git transport timed out; inspect the publication receipt before retrying"
            ) from error
        receipt["commands"].append(
            {
                "args": list(map(str, args)),
                "exit": result.returncode,
                "stdout": result.stdout,
                "stderr": result.stderr,
            }
        )
        if result.returncode:
            raise RuntimeError(
                "git " + str(args[0]) + " failed: " + result.stderr[-3000:]
            )
        return result.stdout.strip()

    try:
        with tempfile.TemporaryDirectory(prefix="publish-", dir=receipts) as tmp:
            frozen = Path(tmp) / "selected.bundle"
            frozen.write_bytes(payload)
            frozen.chmod(0o400)
            mirror = Path(tmp) / "repo.git"
            run(
                "clone",
                "--bare",
                "--filter=blob:none",
                "--single-branch",
                "--branch",
                branch,
                remote,
                mirror,
            )
            run("bundle", "verify", frozen, cwd=mirror)
            if run("bundle", "list-heads", frozen, cwd=mirror).split() != [
                head,
                "refs/heads/" + branch,
            ]:
                raise ValueError(
                    "bundle has a different selected HEAD or additional refs"
                )
            current = run("rev-parse", "refs/heads/" + branch, cwd=mirror)
            receipt["remote_before"] = current
            if current == head:
                receipt.update(status="already_published", remote_after=current)
            else:
                # No force fetch or push: a remote divergence must be inspected.
                run(
                    "fetch",
                    "--no-tags",
                    frozen,
                    "refs/heads/" + branch + ":refs/heads/" + branch,
                    cwd=mirror,
                )
                if run("rev-parse", "refs/heads/" + branch, cwd=mirror) != head:
                    raise ValueError("imported HEAD does not match the selected commit")
                phase = "push"
                receipt.update(
                    status="unknown",
                    phase="push",
                    pending_command=["push", "origin", head + ":refs/heads/" + branch],
                )
                # Record uncertainty before the remote mutation. Even SIGTERM,
                # which bypasses finally, must leave an honest pending receipt.
                save()
                run(
                    "push",
                    "origin",
                    head + ":refs/heads/" + branch,
                    cwd=mirror,
                )
                receipt.pop("pending_command", None)
                observed = run(
                    "ls-remote", "--refs", "origin", "refs/heads/" + branch, cwd=mirror
                ).split()
                if observed != [head, "refs/heads/" + branch]:
                    raise RuntimeError(
                        "remote readback does not match the selected HEAD"
                    )
                receipt.update(status="published", remote_after=head)
    except BaseException as error:
        receipt["status"] = "unknown" if phase == "push" else "failed"
        receipt["error"] = str(error)
        raise
    finally:
        receipt.update(finished=time.time(), receipt=str(receipt_path))
        save()
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--remote", required=True)
    parser.add_argument("--branch", default="agent-team/experiment")
    parser.add_argument("--expected-head", required=True)
    parser.add_argument("--github-auth", action="store_true")
    args = parser.parse_args()
    result = publish(
        args.bundle,
        args.sha256,
        args.remote,
        args.branch,
        args.expected_head,
        args.github_auth,
    )
    print(
        json.dumps(
            {k: v for k, v in result.items() if k != "commands"},
            ensure_ascii=False,
            indent=2,
        )
    )


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
