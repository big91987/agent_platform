#!/usr/bin/env python3
"""Run the repository-owned adapter and SDK, without an installed platform tree."""

import runpy
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent

if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] not in ("legacy", "sandbox"):
        sys.exit("Usage: run_entry.py legacy|sandbox --config PRIVATE_CONFIG")
    mode = sys.argv.pop(1)
    sys.path[:0] = [str(HERE.parents[1] / "sdk/python"), str(HERE)]
    runpy.run_path(
        str(HERE / ("sandbox_entry.py" if mode == "sandbox" else "github_entry.py")),
        run_name="__main__",
    )
