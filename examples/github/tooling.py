"""Resolve owner-pinned verification tools, defaulting to the bundled snapshot."""

from pathlib import Path

BUNDLED = Path(__file__).resolve().parent / "tooling"


def tooling_source(settings):
    source = Path(settings.get("browser_source") or BUNDLED).resolve()
    required = (
        "full_harness/browser/check.cjs",
        "full_harness/common.py",
        "full_harness/quality.py",
    )
    if not all((source / name).is_file() for name in required):
        raise ValueError("Incomplete Harness tooling source: " + str(source))
    return source
