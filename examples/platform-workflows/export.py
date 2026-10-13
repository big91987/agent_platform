#!/usr/bin/env python3
"""Export a fixed Git version of examples and SDK into a new project-owned folder."""

import argparse
import hashlib
import json
import shutil
import subprocess
import tempfile
from pathlib import Path, PurePosixPath


def export(source, destination, ref="HEAD"):
    source, destination = Path(source).resolve(), Path(destination).absolute()
    if destination.exists() or destination.is_symlink():
        raise ValueError(
            "Destination already exists; export a new version and review project customizations before upgrading"
        )
    commit = subprocess.check_output(
        [
            "git",
            "-C",
            str(source),
            "rev-parse",
            "--verify",
            "--end-of-options",
            ref + "^{commit}",
        ],
        text=True,
    ).strip()
    tree = subprocess.check_output(
        [
            "git",
            "-C",
            str(source),
            "ls-tree",
            "-rz",
            commit,
            "--",
            "sdk/python",
            "examples",
            "go.mod",
            "go.sum",
        ]
    )
    rows = []
    for item in tree.split(b"\0"):
        if not item:
            continue
        info, name = item.split(b"\t", 1)
        mode, kind, blob = info.decode().split()
        decoded = name.decode()
        parts = PurePosixPath(decoded).parts
        if (
            decoded.startswith("/")
            or any(x in ("..", ".") for x in parts)
            or str(PurePosixPath(decoded)) != decoded
        ):
            raise ValueError("Example export refuses non-canonical Git paths")
        if kind != "blob" or mode not in ("100644", "100755"):
            raise ValueError(
                "Example export refuses links and non-regular tracked inputs"
            )
        rows.append((decoded, mode, blob))
    required = {
        "sdk/python/agent_platform_client/__init__.py",
        "examples/platform-workflows/run_entry.py",
        "examples/platform-workflows/sandbox_entry.py",
    }
    if not required.issubset({r[0] for r in rows}):
        raise ValueError("Selected version lacks the repository-owned entry and SDK")
    destination.parent.mkdir(parents=True, exist_ok=True)
    staging = Path(tempfile.mkdtemp(prefix=".harness-export-", dir=destination.parent))
    try:
        inventory = {}
        for name, mode, blob in rows:
            data = subprocess.check_output(
                ["git", "-C", str(source), "cat-file", "blob", blob]
            )
            file = staging / name
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_bytes(data)
            file.chmod(0o755 if mode == "100755" else 0o644)
            inventory[name] = {
                "sha256": hashlib.sha256(data).hexdigest(),
                "bytes": len(data),
                "mode": mode,
            }
        (staging / "SOURCE.json").write_text(
            json.dumps({"commit": commit, "files": inventory}, indent=2) + "\n"
        )
        staging.rename(destination)
    finally:
        if staging.exists():
            shutil.rmtree(staging)
    return commit


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--destination", required=True, type=Path)
    parser.add_argument("--ref", default="HEAD")
    args = parser.parse_args()
    print(export(Path(__file__).resolve().parents[2], args.destination, args.ref))
