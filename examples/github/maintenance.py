"""Maintainer-run validation for PRs outside the product Agent pipeline.

Policy is read only from the trusted host configuration. No Agent is dispatched.
The maintainer reviews the engineering change and selects its checks; successful
commands attest to that exact head/main pair, never to future commits.
"""

import fnmatch
import json
import os
import subprocess
from pathlib import Path

from requirements import github


def receipt_path(settings, number):
    return Path(settings["registry"]) / "maintenance" / f"{number}.json"


def eligible(settings, pr):
    return (
        pr["state"] == "open"
        and not pr["draft"]
        and pr["base"]["ref"] == "main"
        and (pr["head"].get("repo") or {}).get("full_name") == settings["repository"]
    )


def verified(settings, pr, base):
    path = receipt_path(settings, pr["number"])
    if not path.exists():
        return False
    saved = json.loads(path.read_text())
    return (
        eligible(settings, pr)
        and saved.get("repository") == settings["repository"]
        and saved.get("head") == pr["head"]["sha"]
        and saved.get("base") == base
        and saved.get("policy") == settings.get("maintenance")
        and saved.get("result") == "success"
    )


def status(settings, head, state, description):
    github(
        f"repos/{settings['repository']}/statuses/{head}",
        {"context": "pipeline/refresh", "state": state, "description": description},
    )


def git(workspace, *args):
    return subprocess.check_output(
        ["git", "-C", str(workspace), *args], text=True, stderr=subprocess.PIPE
    ).strip()


def clean(workspace, head):
    if git(workspace, "rev-parse", "HEAD") != head or git(
        workspace, "status", "--porcelain", "--untracked-files=all"
    ):
        raise ValueError("Use a clean checkout of the exact PR head")


def verify(settings, number, workspace):
    repo = settings["repository"]
    pr = github(f"repos/{repo}/pulls/{number}")
    if not eligible(settings, pr):
        raise ValueError("Requires an open ready PR from the same repository into main")
    head = pr["head"]["sha"]
    status(settings, head, "pending", "Maintainer engineering checks running")
    receipt = receipt_path(settings, number)
    receipt.parent.mkdir(parents=True, exist_ok=True)
    # Invalidate earlier success before retrying; a failed retry cannot reuse it.
    receipt.write_text(json.dumps({"result": "pending"}))
    try:
        policy = settings.get("maintenance", {})
        paths, checks = policy.get("paths", []), policy.get("checks", [])
        if not paths or not checks:
            raise ValueError(
                "Configure maintenance.paths and nonempty maintenance.checks on the trusted host"
            )
        if not all(
            isinstance(p, str)
            and p
            and not p.startswith("/")
            and ".." not in p.split("/")
            for p in paths
        ):
            raise ValueError("Maintenance paths must be repository-relative globs")
        if not all(
            isinstance(cmd, list)
            and cmd
            and all(isinstance(arg, str) and arg for arg in cmd)
            for cmd in checks
        ):
            raise ValueError("Each maintenance check must be a nonempty argv array")
        workspace = Path(workspace).resolve()
        if Path(git(workspace, "rev-parse", "--show-toplevel")).resolve() != workspace:
            raise ValueError("Workspace must be the repository root")
        clean(workspace, head)
        base = github(f"repos/{repo}/commits/main")["sha"]
        # Missing commits and a stale checkout both fail. Fetch/sync is the
        # maintainer's responsibility; this verifier never changes product code.
        git(workspace, "merge-base", "--is-ancestor", base, head)
        changed = git(
            workspace, "diff", "--name-only", "--no-renames", "-z", base, head
        ).split("\0")
        changed = [p for p in changed if p]
        if not changed or any(
            not any(fnmatch.fnmatchcase(p, glob) for glob in paths) for p in changed
        ):
            raise ValueError(
                "PR includes paths outside the configured maintenance scope"
            )
        env = {
            k: v for k, v in os.environ.items() if k not in {"GH_TOKEN", "GITHUB_TOKEN"}
        }
        for index, cmd in enumerate(checks, 1):
            log = receipt.parent / f"{number}-{head}-{index}.log"
            with log.open("w") as output:
                subprocess.run(
                    cmd,
                    cwd=workspace,
                    env=env,
                    stdout=output,
                    stderr=subprocess.STDOUT,
                    timeout=policy.get("timeout_seconds", 600),
                    check=True,
                )
            print(
                f"Engineering check {index}/{len(checks)} passed; log: {log}",
                flush=True,
            )
        clean(workspace, head)
        latest = github(f"repos/{repo}/pulls/{number}")
        latest_base = github(f"repos/{repo}/commits/main")["sha"]
        if (
            not eligible(settings, latest)
            or latest["head"]["sha"] != head
            or latest_base != base
        ):
            raise ValueError("PR or main changed during verification; sync and rerun")
        receipt.write_text(
            json.dumps(
                {
                    "repository": repo,
                    "head": head,
                    "base": base,
                    "policy": policy,
                    "paths": changed,
                    "result": "success",
                },
                indent=2,
            )
            + "\n"
        )
        status(
            settings,
            head,
            "success",
            "Maintainer checks passed against main " + base[:12],
        )
        return receipt
    except BaseException:
        receipt.write_text(json.dumps({"result": "failure"}))
        status(
            settings,
            head,
            "failure",
            "Maintainer checks failed; inspect logs and retry",
        )
        raise
