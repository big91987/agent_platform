#!/usr/bin/env python3
"""Install a repository-neutral, versioned Issue forwarding workflow."""

import argparse
import hashlib
import json
import re
import subprocess
import tempfile
from pathlib import Path

from export import export

WORKFLOW = Path(".github/workflows/agent-platform-entry.yml")
MANIFEST = Path(".github/agent-platform-entry-source.json")


def digest(content):
    return hashlib.sha256(content).hexdigest()


def install(project, repository, upgrade=False, template="legacy"):
    project = project.resolve(strict=True)
    if not re.fullmatch(r"[\w.-]+/[\w.-]+", repository):
        raise ValueError("repository must be owner/name")
    remote = subprocess.check_output(
        ["git", "-C", str(project), "remote", "get-url", "origin"], text=True
    ).strip()
    if remote not in (
        f"https://github.com/{repository}.git",
        f"https://github.com/{repository}",
        f"git@github.com:{repository}.git",
    ):
        raise ValueError("Project origin does not match configured repository")
    if template not in ("legacy", "sandbox"):
        raise ValueError("Choose the legacy or sandbox Issue entry")
    source = (
        Path(__file__)
        .with_name("github-entry.yml" if template == "legacy" else "sandbox-entry.yml")
        .read_bytes()
    )
    workflow = (
        WORKFLOW
        if template == "legacy"
        else Path(".github/workflows/agent-platform-sandbox-entry.yml")
    )
    manifest_path = (
        MANIFEST
        if template == "legacy"
        else Path(".github/agent-platform-sandbox-entry-source.json")
    )
    target, manifest = project / workflow, project / manifest_path
    if not target.resolve().is_relative_to(
        project
    ) or not manifest.resolve().is_relative_to(project):
        raise ValueError("Installation path escapes project")
    recorded = json.loads(manifest.read_text()) if manifest.exists() else None
    expected = {"path": str(workflow), "sha256": digest(source)}
    if recorded is not None and (
        set(recorded) != {"path", "sha256"}
        or recorded["path"] != str(workflow)
        or not re.fullmatch(r"[0-9a-f]{64}", str(recorded["sha256"]))
    ):
        raise ValueError("Invalid installation manifest; inspect drift")
    actual = digest(target.read_bytes()) if target.exists() else None
    if (
        actual is not None
        and actual != digest(source)
        and (recorded is None or recorded["sha256"] != actual)
    ):
        raise ValueError("Installed workflow drift; reconcile before upgrading")
    if recorded and recorded != expected and not upgrade:
        raise ValueError("Use --upgrade for a source version change")
    if actual is None and recorded and recorded != expected:
        raise ValueError(
            "Workflow missing; recover using its recorded source version first"
        )
    created = ensure_bundle(project)
    if actual == digest(source) and recorded == expected:
        return created
    # Both writes are atomic. If interrupted between them, rerunning the same
    # version safely recognizes exact source bytes without overwriting edits.
    if actual != digest(source):
        atomic_write(target, source)
    atomic_write(manifest, (json.dumps(expected, indent=2) + "\n").encode())
    return True


def ensure_bundle(project):
    bundle = project / ".harness/agent-platform"
    if not bundle.resolve().is_relative_to(project) or bundle.is_symlink():
        raise ValueError("Harness bundle escapes project")
    created = not bundle.exists()
    if created:
        export(Path(__file__).resolve().parents[2], bundle)
    # Project customizations are owned by its Git history. Never silently
    # overwrite them when updating a workflow YAML.
    for name in (
        "sdk/python/agent_platform_client/__init__.py",
        "examples/platform-workflows/run_entry.py",
        "examples/platform-workflows/sandbox_entry.py",
        "examples/platform-workflows/github_entry.py",
        "examples/platform-workflows/materials.py",
        "examples/github/requirements.py",
    ):
        file = bundle / name
        if (
            not file.is_file()
            or not file.resolve().is_relative_to(bundle.resolve())
            or file.is_symlink()
        ):
            raise ValueError(
                "Project-owned Harness is incomplete; review its export before installing the entry"
            )
    return created


def atomic_write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as output:
        output.write(content)
        temporary = Path(output.name)
    temporary.replace(path)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--project", type=Path, required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--upgrade", action="store_true")
    parser.add_argument("--template", choices=("legacy", "sandbox"), default="legacy")
    args = parser.parse_args()
    print(
        "Installed"
        if install(args.project, args.repository, args.upgrade, args.template)
        else "Unchanged"
    )


if __name__ == "__main__":
    main()
