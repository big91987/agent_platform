"""Keep installed role assets at immutable, content-addressed paths."""

import hashlib
import json
import shutil
import stat
import tempfile
from pathlib import Path


def excluded(name):
    return name in (".git", "__pycache__", ".DS_Store") or name.endswith(".pyc")


def tree_digest(root):
    rows = []
    for path in sorted(root.rglob("*")):
        relative = path.relative_to(root)
        if any(excluded(part) for part in relative.parts):
            continue
        mode = path.lstat().st_mode
        if stat.S_ISDIR(mode):
            rows.append([relative.as_posix(), "directory"])
        elif stat.S_ISREG(mode):
            rows.append(
                [
                    relative.as_posix(),
                    mode & 0o111,
                    hashlib.sha256(path.read_bytes()).hexdigest(),
                ]
            )
        else:
            raise ValueError("Asset tree contains an unsupported file: " + str(path))
    return hashlib.sha256(json.dumps(rows).encode()).hexdigest()


def snapshot(source, root, upgrade, previous=None):
    source = Path(source).resolve(strict=True)
    digest = tree_digest(source)
    if previous and previous["sha256"] != digest and not upgrade:
        raise ValueError("Use --upgrade to install a new asset version: " + str(source))
    target = root / digest / source.name
    if not target.exists():
        target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        with tempfile.TemporaryDirectory(dir=target.parent) as temporary:
            copy = Path(temporary) / target.name
            shutil.copytree(
                source,
                copy,
                ignore=lambda _, names: [name for name in names if excluded(name)],
            )
            if tree_digest(copy) != digest:
                raise ValueError("Asset changed while being installed: " + str(source))
            copy.rename(target)
    if target.is_symlink() or tree_digest(target) != digest:
        raise ValueError(
            "Installed asset differs from its recorded digest: " + str(target)
        )
    return target, {"source": str(source), "path": str(target), "sha256": digest}
