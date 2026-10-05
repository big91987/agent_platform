#!/usr/bin/env python3
"""Fixed repository operations for the platform Workflow example, not CI routing."""

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path


def git(workspace, *args):
    result = subprocess.run(
        [
            "git",
            "-C",
            str(workspace),
            *(
                [
                    "-c",
                    "credential.helper=",
                    "-c",
                    "credential.helper=!gh auth git-credential",
                ]
                if os.environ.get("GH_TOKEN") and args[0] in ("fetch", "push")
                else []
            ),
            *args,
        ],
        capture_output=True,
        text=True,
        check=True,
        timeout=90,
        env={**os.environ, "GIT_TERMINAL_PROMPT": "0"},
    )
    return result.stdout.strip()


def remote_repository(value):
    match = re.fullmatch(
        r"(?:https://github\.com/|git@github\.com:)([\w.-]+/[\w.-]+?)(?:\.git)?", value
    )
    if not match:
        raise ValueError("origin must be credential-free GitHub HTTPS or SSH")
    return match[1]


def branch_for(run):
    if not re.fullmatch(r"[a-f0-9]{32}", run.get("run_id", "")):
        raise ValueError("valid platform run_id required")
    return "workflow/" + run["run_id"]


def prepare(workspace, run, base):
    branch = branch_for(run)
    current = git(workspace, "branch", "--show-current")
    if current == branch:
        return {
            "branch": branch,
            "head": git(workspace, "rev-parse", "HEAD"),
            "continued": True,
        }
    if git(workspace, "status", "--porcelain"):
        raise ValueError(
            "Workspace has existing changes; choose a clean dedicated checkout. Nothing was reset."
        )
    git(workspace, "check-ref-format", "--branch", base)
    git(
        workspace,
        "fetch",
        "origin",
        "+refs/heads/" + base + ":refs/remotes/origin/" + base,
    )
    git(workspace, "checkout", "-b", branch, "origin/" + base)
    return {"branch": branch, "base": base, "head": git(workspace, "rev-parse", "HEAD")}


def check_publish_files(workspace):
    # Inspect what git add --all would include, including currently staged files.
    names = (
        subprocess.check_output(
            ["git", "-C", str(workspace), "ls-files", "-co", "--exclude-standard", "-z"]
        )
        .decode()
        .split("\0")
    )
    private = re.compile(
        rb"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,})"
    )
    for name in set(filter(None, names)):
        file = workspace / name
        if not file.exists():
            continue
        if not file.resolve().is_relative_to(workspace.resolve()):
            raise ValueError("Refusing workspace-external symlink: " + name)
        if file.is_dir():
            raise ValueError(
                "Submodule/directory staging requires separate review: " + name
            )
        if file.name in (
            ".env",
            "auth.json",
            "credentials.json",
            "id_rsa",
            "id_ed25519",
        ) or any(
            part in ("node_modules", ".codex", ".aws", ".workflow-evidence")
            for part in file.parts
        ):
            raise ValueError("Refusing local credentials/cache artifact: " + name)
        if file.stat().st_size > 5 * 1024 * 1024:
            raise ValueError(
                "Artifact exceeds this example's 5 MiB publication limit: " + name
            )
        if private.search(file.read_bytes()):
            raise ValueError("Potential private credential in " + name)


def publish(workspace, run):
    branch = branch_for(run)
    if git(workspace, "branch", "--show-current") != branch:
        raise ValueError(
            "Current branch does not belong to this Run; no commit or push performed"
        )
    for name in ("docs/workflow/qa.md", "docs/workflow/pr.md"):
        if not (workspace / name).is_file():
            raise ValueError("Missing delivery artifact: " + name)
    check_publish_files(workspace)
    git(workspace, "add", "--all")
    if git(workspace, "diff", "--cached", "--name-only"):
        title = run.get("input", "Workflow delivery").splitlines()[0][:120]
        git(workspace, "commit", "-m", title)
    git(workspace, "push", "origin", "HEAD:refs/heads/" + branch)
    return {
        "branch": branch,
        "head": git(workspace, "rev-parse", "HEAD"),
        "pushed": True,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=["prepare", "publish"])
    parser.add_argument("--repository", required=True)
    parser.add_argument("--base", default="main")
    args = parser.parse_args()
    run = json.load(sys.stdin)
    workspace = Path(run["workspace_path"]).resolve()
    if workspace != Path.cwd().resolve():
        raise ValueError("Run workspace and process directory differ")
    if (
        remote_repository(git(workspace, "remote", "get-url", "origin"))
        != args.repository
    ):
        raise ValueError("Workspace origin differs from configured repository")
    result = (
        prepare(workspace, run, args.base)
        if args.operation == "prepare"
        else publish(workspace, run)
    )
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        # Keep actionable Git stderr; credentials are env references, never argv.
        print(error.stderr or str(error), file=sys.stderr)
        raise SystemExit(error.returncode)
