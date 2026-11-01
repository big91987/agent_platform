#!/usr/bin/env python3
"""Install a hash-verified source archive as an immutable experiment release."""

import argparse
import hashlib
import json
import re
import shutil
import tarfile
import tempfile
from pathlib import Path, PurePosixPath


def install(archive, root, revision, expected_sha):
    archive, root = Path(archive).resolve(), Path(root).resolve()
    if not re.fullmatch(r"[0-9a-f]{40}", revision) or not re.fullmatch(
        r"[0-9a-f]{64}", expected_sha
    ):
        raise ValueError("exact source revision and archive SHA-256 are required")
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    if digest != expected_sha:
        raise ValueError("source archive SHA-256 mismatch")
    releases = root / "releases"
    releases.mkdir(parents=True, exist_ok=True)
    target = releases / revision
    if target.exists():
        recorded = json.loads((target / "release.json").read_text())
        if recorded != {"revision": revision, "archive_sha256": expected_sha}:
            raise ValueError("existing immutable release differs")
        return target
    staging = Path(tempfile.mkdtemp(prefix=".install-", dir=releases))
    try:
        with tarfile.open(archive) as source:
            for member in source:
                relative = PurePosixPath(member.name)
                if (
                    relative.is_absolute()
                    or ".." in relative.parts
                    or ".git" in relative.parts
                    or member.issym()
                    or member.islnk()
                ):
                    raise ValueError("unsafe source archive member")
                if member.isdir():
                    (staging / relative).mkdir(parents=True, exist_ok=True)
                elif member.isfile():
                    destination = staging / relative
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    with source.extractfile(member) as content:
                        destination.write_bytes(content.read())
                    destination.chmod(0o755 if member.mode & 0o111 else 0o644)
                else:
                    raise ValueError("unsupported source archive member")
        if not (staging / "examples/agent-team-factory/factory.py").is_file():
            raise ValueError("archive does not contain the supported factory entry")
        (staging / "release.json").write_text(
            json.dumps({"revision": revision, "archive_sha256": expected_sha}, indent=2)
        )
        staging.rename(target)
        return target
    finally:
        if staging.exists():
            shutil.rmtree(staging)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive", required=True)
    parser.add_argument("--root", required=True)
    parser.add_argument("--revision", required=True)
    parser.add_argument("--sha256", required=True)
    args = parser.parse_args()
    target = install(args.archive, args.root, args.revision, args.sha256)
    print(
        json.dumps(
            {
                "release": str(target),
                "entry": str(target / "examples/agent-team-factory/factory.py"),
            },
            ensure_ascii=False,
        )
    )


if __name__ == "__main__":
    main()
