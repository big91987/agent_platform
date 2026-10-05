#!/usr/bin/env python3
"""Install or upgrade the versioned Model Relay deployment workflow."""

import argparse
import hashlib
import json
from pathlib import Path

WORKFLOW = Path(".github/workflows/deploy-local.yml")
MANIFEST = Path("deploy/platform-preview-source.json")


def digest(value):
    return hashlib.sha256(value).hexdigest()


def install(project, upgrade=False):
    project = project.resolve()
    if not (project / ".git").exists():
        raise ValueError("project must be a Git checkout")
    source = Path(__file__).with_name("deploy-local.yml").read_bytes()
    target = project / WORKFLOW
    manifest = project / MANIFEST
    if target.exists() != manifest.exists():
        raise ValueError("workflow or installation manifest is missing; inspect drift")
    if target.exists():
        recorded = json.loads(manifest.read_text())
        if recorded != {"path": str(WORKFLOW), "sha256": digest(target.read_bytes())}:
            raise ValueError("installed workflow changed outside installer")
        if target.read_bytes() == source:
            return False
        if not upgrade:
            raise ValueError("workflow update requires --upgrade")
    target.parent.mkdir(parents=True, exist_ok=True)
    manifest.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(source)
    manifest.write_text(
        json.dumps(
            {"path": str(WORKFLOW), "sha256": digest(source)}, sort_keys=True, indent=2
        )
        + "\n"
    )
    return True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--project", type=Path, required=True)
    parser.add_argument("--upgrade", action="store_true")
    args = parser.parse_args()
    print(
        "Workflow installed"
        if install(args.project, args.upgrade)
        else "Workflow unchanged"
    )


if __name__ == "__main__":
    main()
