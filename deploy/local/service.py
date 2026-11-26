#!/usr/bin/env python3
"""Launch an installed local platform with private server-only credentials."""

import json
import os
import stat
import sys
from pathlib import Path


def read_private(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        info = os.fstat(fd)
        if (
            not stat.S_ISREG(info.st_mode)
            or info.st_uid != os.getuid()
            or info.st_mode & 0o077
        ):
            raise ValueError("service configuration must be private and owned")
        with os.fdopen(fd, "r", closefd=False) as stream:
            return json.load(stream)
    finally:
        os.close(fd)


def main():
    root = Path(sys.argv[1]).resolve(strict=True)
    config = read_private(root / "instance.json")
    env = {
        key: os.environ[key]
        for key in ("HOME", "USER", "LANG", "TMPDIR")
        if key in os.environ
    }
    env["PATH"] = config["path"]
    credentials = root / "credentials.json"
    if credentials.exists():
        private = read_private(credentials)
        if not isinstance(private, dict) or any(
            not isinstance(k, str)
            or not isinstance(v, str)
            or "=" in k
            or "\0" in k + v
            for k, v in private.items()
        ):
            raise ValueError("invalid private environment")
        if set(private).intersection(
            {"HOME", "PATH", "PYTHONPATH", "DYLD_INSERT_LIBRARIES"}
        ):
            raise ValueError("reserved private environment key")
        env.update(private)
    os.execve(config["argv"][0], config["argv"], env)


if __name__ == "__main__":
    main()
