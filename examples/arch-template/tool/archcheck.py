#!/usr/bin/env python3
"""Supported CLI entry; keep a managed installation stable during execution."""

import fcntl
import json
from contextlib import ExitStack
from pathlib import Path


def run():
    try:
        with ExitStack() as resources:
            manifest = Path(__file__).parent / "SOURCE.json"
            if manifest.exists():
                lock = resources.enter_context(manifest.open("rb"))
                fcntl.flock(lock, fcntl.LOCK_SH | fcntl.LOCK_NB)
            from archcheck.cli import main

            return main()
    except OSError:
        print(
            json.dumps(
                dict(
                    status="incomplete",
                    errors=["tool installation is unavailable or upgrading"],
                )
            )
        )
        return 2


if __name__ == "__main__":
    raise SystemExit(run())
