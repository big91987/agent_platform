#!/usr/bin/env python3
"""Fixed repository operations for the platform Workflow example, not CI routing."""

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path

from materials import install_material, verify_material


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


def task_docs(run):
    branch_for(run)  # Validate the stable Run identity before constructing paths.
    return "docs/workflow/runs/" + run["run_id"]


def prepare(workspace, run, base, scoped_docs=False):
    branch = branch_for(run)
    current = git(workspace, "branch", "--show-current")
    if current == branch:
        return {
            "branch": branch,
            "head": git(workspace, "rev-parse", "HEAD"),
            "continued": True,
            **({"document_root": task_docs(run)} if scoped_docs else {}),
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
    return {
        "branch": branch,
        "base": base,
        "head": git(workspace, "rev-parse", "HEAD"),
        **({"document_root": task_docs(run)} if scoped_docs else {}),
    }


def prepare_trellis(workspace, executable, version):
    environment = dict(os.environ)
    environment.pop("GH_TOKEN", None)
    actual = subprocess.check_output(
        [executable, "--version"], text=True, timeout=15, env=environment
    ).strip()
    if actual != version:
        raise ValueError(
            "Trellis version differs from installed Connector; upgrade through the installer"
        )
    trellis = workspace / ".trellis"
    initialized = (trellis / "scripts/get_context.py").is_file()
    if not initialized:
        if trellis.exists():
            raise ValueError(
                "Incomplete existing Trellis installation; repair with Trellis before retrying"
            )
        subprocess.run(
            [
                executable,
                "init",
                "--codex",
                "--yes",
                "--skip-existing",
                "--user",
                "workflow",
            ],
            cwd=workspace,
            env=environment,
            check=True,
            capture_output=True,
            text=True,
            timeout=120,
        )
        # Native platform Skill selection is authoritative. Generated host adapters,
        # personal journals, and local Trellis task state are not shared artifacts.
        ignore = workspace / ".gitignore"
        if ignore.is_symlink():
            raise ValueError("Material ignore file must not be a symlink")
        original = ignore.read_text() if ignore.exists() else ""
        patterns = [
            "/.agents/",
            "/.codex/",
            "/.trellis/workspace/",
            "/.trellis/tasks/",
            "/.trellis/.runtime/",
            "/.trellis/.developer",
        ]
        missing = [line for line in patterns if line not in original.splitlines()]
        if missing:
            ignore.write_text(
                original.rstrip()
                + "\n\n# Local Trellis execution state\n"
                + "\n".join(missing)
                + "\n"
            )
        config = trellis / "config.yaml"
        config.write_text(
            config.read_text()
            + "\n# Git publishing belongs to the platform Connector.\nsession_auto_commit: false\ncodex:\n  dispatch_mode: inline\n"
        )
    version_file = trellis / ".version"
    if not version_file.is_file() or version_file.read_text().strip() != version:
        raise ValueError(
            "Trellis project asset version differs; inspect trellis update --dry-run, "
            "upgrade with the pinned CLI and preserve local changes before retrying"
        )
    subprocess.run(
        [sys.executable, str(trellis / "scripts/get_context.py"), "--mode", "packages"],
        cwd=workspace,
        env=environment,
        check=True,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return {
        "version": actual,
        "initialized": not initialized,
        "spec_root": ".trellis/spec",
    }


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
            part
            in (
                "node_modules",
                ".codex",
                ".aws",
                ".workflow-evidence",
                ".workflow-input",
            )
            for part in file.parts
        ):
            raise ValueError("Refusing local credentials/cache artifact: " + name)
        if file.stat().st_size > 5 * 1024 * 1024:
            raise ValueError(
                "Artifact exceeds this example's 5 MiB publication limit: " + name
            )
        if private.search(file.read_bytes()):
            raise ValueError("Potential private credential in " + name)


def delivery_artifacts(workspace, run):
    directory = (workspace / task_docs(run)).resolve()
    if directory != workspace.resolve() / task_docs(run):
        raise ValueError("Task document directory escapes workspace")
    qa = next(
        (
            step
            for step in reversed(run.get("previous_results", []))
            if step.get("node_id") == "qa"
        ),
        None,
    )
    if (
        not qa
        or qa.get("status") != "completed"
        or qa.get("result", {}).get("route") != "next"
    ):
        raise ValueError("Missing completed QA handoff for this delivery")
    artifacts = qa["result"].get("artifacts", [])
    if not artifacts:
        raise ValueError("QA handoff has no evidence artifacts")
    for name in [*artifacts, task_docs(run) + "/pr.md"]:
        path = Path(name)
        actual = (workspace / path).resolve()
        if (
            path.is_absolute()
            or not actual.is_relative_to(directory)
            or not actual.is_file()
        ):
            raise ValueError(
                "Delivery artifact must be a file in this task's document directory: "
                + name
            )
        if not actual.stat().st_size:
            raise ValueError("Empty delivery artifact: " + name)


def publish(workspace, run, scoped_docs=False, materials=False):
    if materials:
        verify_material(workspace, run)
    branch = branch_for(run)
    if git(workspace, "branch", "--show-current") != branch:
        raise ValueError(
            "Current branch does not belong to this Run; no commit or push performed"
        )
    if scoped_docs:
        delivery_artifacts(workspace, run)
    else:  # Frozen installations keep their previous path contract until upgraded.
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


def run_tests(workspace, run, command):
    verify_material(workspace, run)
    if (
        not isinstance(command, list)
        or not command
        or any(not isinstance(x, str) or not x or "\0" in x for x in command)
    ):
        raise ValueError("Fixed test command must be a non-empty argv array")
    environment = dict(os.environ)
    environment.pop("GH_TOKEN", None)
    return subprocess.run(
        command, cwd=workspace, stdin=subprocess.DEVNULL, env=environment
    ).returncode


def prepare_material(workspace, run, repository):
    receipt = install_material(workspace, run, repository)
    if receipt:
        ignore = workspace / ".gitignore"
        if ignore.is_symlink():
            raise ValueError("Material ignore file must not be a symlink")
        original = ignore.read_text() if ignore.exists() else ""
        if "/.workflow-input/" not in original.splitlines():
            ignore.write_text(
                original.rstrip()
                + "\n\n# Private, pinned Workflow input materials\n/.workflow-input/\n"
            )
    return receipt


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=["prepare", "publish", "verify"])
    parser.add_argument("--repository", required=True)
    parser.add_argument("--base", default="main")
    parser.add_argument("--task-docs", action="store_true")
    parser.add_argument("--materials", action="store_true")
    parser.add_argument("--test-command")
    parser.add_argument("--trellis-executable")
    parser.add_argument("--trellis-version")
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
    if args.operation == "verify":
        if not args.test_command:
            raise ValueError("Fixed test argv required")
        raise SystemExit(run_tests(workspace, run, json.loads(args.test_command)))
    result = (
        prepare(workspace, run, args.base, args.task_docs)
        if args.operation == "prepare"
        else publish(workspace, run, args.task_docs, args.materials)
    )
    if args.operation == "prepare" and args.materials:
        material = prepare_material(workspace, run, args.repository)
        if material:
            result["material"] = material
    if args.operation == "prepare" and args.trellis_executable:
        result["trellis"] = prepare_trellis(
            workspace, args.trellis_executable, args.trellis_version
        )
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        # Keep actionable Git stderr; credentials are env references, never argv.
        print(error.stderr or str(error), file=sys.stderr)
        raise SystemExit(error.returncode)
