"""Trusted repository delivery scope shared by QA, publication and PR review."""

from pathlib import PurePosixPath

PRODUCT_PATHS = ("app", "tests", ".trellis/spec", "README.md", "deploy")


def extra_files(settings=None):
    values = (settings or {}).get("extra_product_files", [])
    if not isinstance(values, list):
        raise ValueError("extra_product_files must be a list of exact relative files")
    for name in values:
        if not isinstance(name, str) or not name:
            raise ValueError("extra_product_files requires exact relative files")
        path = PurePosixPath(name)
        if (
            path.is_absolute()
            or path.as_posix() != name
            or any(p in (".", "..", ".git", "AGENTS.md") for p in path.parts)
            or any(c in name for c in "*?[]\\\n\r\0")
            or name.startswith(("-", ":"))
            or path.parts[0]
            in (".github", ".codex", ".agents", ".harness", "harness", "full_harness")
            or name.startswith(".trellis/scripts/")
            or name in ("harness-project.json", "harness-upstream.json")
        ):
            raise ValueError("Invalid or protected extra product file: " + name)
    return sorted(set(values))


def product_paths(settings=None):
    return [*PRODUCT_PATHS, *extra_files(settings)]


def allowed_product_path(name, settings=None):
    if "AGENTS.md" in PurePosixPath(name).parts:
        return False
    return name in extra_files(settings) or any(
        name == root or name.startswith(root + "/") for root in (*PRODUCT_PATHS, "docs")
    )


def validate_files(workspace, settings=None):
    for name in extra_files(settings):
        path = workspace / name
        if (
            path.is_dir()
            or path.is_symlink()
            or any(
                parent.is_symlink()
                for parent in path.parents
                if parent != workspace and parent.is_relative_to(workspace)
            )
        ):
            raise ValueError("Extra product paths must be regular files: " + name)
