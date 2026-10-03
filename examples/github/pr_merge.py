"""Local, recoverable merge of an exact fetched main commit into a PR branch.

The caller serializes operations per workspace, persists the expected PR head,
base and original conflict list, and authorizes any additional staged changes.
No fetch, checkout, reset, rebase or push is performed here.
"""

import re
import subprocess
from pathlib import Path, PurePosixPath


class MergeError(RuntimeError):
    """The merge cannot safely proceed; existing work is preserved."""


class ManualResolutionRequired(MergeError):
    """Framework configuration conflicted and requires human resolution."""


MERGE_MESSAGE = "Merge latest main into task branch"


def _git(workspace: Path, *args: str, check: bool = True):
    try:
        result = subprocess.run(
            ["git", "--literal-pathspecs", "-C", str(workspace), *args],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            errors="surrogateescape",
        )
    except OSError:
        raise MergeError("Git could not be executed") from None
    if check and result.returncode:
        # Git stderr may include credential-bearing remotes or sensitive paths.
        raise MergeError("Git operation failed; inspect the workspace locally")
    return result


def _value(workspace: Path, *args: str) -> str:
    return _git(workspace, *args).stdout.strip()


def _paths(workspace: Path, *args: str) -> list[str]:
    return sorted(set(filter(None, _git(workspace, *args, "-z").stdout.split("\0"))))


def _protected(path: str) -> bool:
    parts = PurePosixPath(path).parts
    return "AGENTS.md" in parts or parts[0] in {
        ".github",
        "harness",
        "full_harness",
        "scripts",
    }


def _repository(workspace: Path) -> None:
    root = _value(workspace, "rev-parse", "--show-toplevel")
    if Path(root).resolve() != workspace.resolve():
        raise MergeError("Workspace must be the registered repository root")


def _commit_sha(workspace: Path, sha: str) -> None:
    if not re.fullmatch(r"(?:[0-9a-f]{40}|[0-9a-f]{64})", sha):
        raise MergeError("An exact commit SHA is required")
    if _value(workspace, "cat-file", "-t", sha) != "commit":
        raise MergeError("Requested object is not a commit")


def _merge_head(workspace: Path) -> str:
    result = _git(workspace, "rev-parse", "--verify", "MERGE_HEAD", check=False)
    return result.stdout.strip() if result.returncode == 0 else ""


def _conflicts(workspace: Path) -> list[str]:
    return _paths(workspace, "diff", "--name-only", "--diff-filter=U")


def _commit(workspace: Path) -> str:
    _git(
        workspace,
        "-c",
        "core.hooksPath=/dev/null",
        "-c",
        "commit.gpgsign=false",
        "commit",
        "--no-verify",
        "-m",
        MERGE_MESSAGE,
    )
    return _value(workspace, "rev-parse", "HEAD")


def prepare_merge(
    workspace: Path, branch: str, expected_head: str, base_sha: str
) -> dict:
    """Merge a fetched base, or resume precisely the same interrupted merge.

    `conflicts` may be empty on recovery after a resolution was already staged.
    A committed merge is recovered by the caller using its persisted head/state;
    this function never treats an unexpected new HEAD as authorized.
    """
    workspace = Path(workspace)
    _repository(workspace)
    _commit_sha(workspace, expected_head)
    _commit_sha(workspace, base_sha)
    _git(workspace, "check-ref-format", "refs/heads/" + branch)
    current_branch = _value(workspace, "symbolic-ref", "--quiet", "HEAD")
    if current_branch != "refs/heads/" + branch:
        raise MergeError("Workspace is not on the registered PR branch")
    head = _value(workspace, "rev-parse", "HEAD")
    if head != expected_head:
        raise MergeError("PR branch HEAD changed; refresh the expected head")
    merge_head = _merge_head(workspace)
    if merge_head:
        if (
            merge_head != base_sha
            or _value(workspace, "rev-parse", "ORIG_HEAD") != expected_head
        ):
            raise MergeError("A different merge is already in progress")
        conflicts = _conflicts(workspace)
        if any(_protected(path) for path in conflicts):
            raise ManualResolutionRequired(
                "Existing merge has protected configuration conflicts"
            )
        return {"state": "conflicts", "conflicts": conflicts, "head": head}
    if _git(workspace, "status", "--porcelain=v1", "--untracked-files=all").stdout:
        raise MergeError("Workspace has local changes; merge was not started")
    for name in (
        "CHERRY_PICK_HEAD",
        "REVERT_HEAD",
        "rebase-merge",
        "rebase-apply",
        "sequencer",
    ):
        path = Path(_value(workspace, "rev-parse", "--git-path", name))
        if not path.is_absolute():
            path = workspace / path
        if path.exists():
            raise MergeError("Another Git operation is in progress")
    ancestor = _git(
        workspace, "merge-base", "--is-ancestor", base_sha, head, check=False
    )
    if ancestor.returncode == 0:
        return {"state": "unchanged", "conflicts": [], "head": head}
    if ancestor.returncode != 1:
        raise MergeError("Cannot determine merge ancestry")
    merged = _git(
        workspace,
        "-c",
        "core.hooksPath=/dev/null",
        "merge",
        "--no-ff",
        "--no-commit",
        "--no-edit",
        base_sha,
        check=False,
    )
    conflicts = _conflicts(workspace)
    if conflicts:
        if any(_protected(path) for path in conflicts):
            _git(workspace, "merge", "--abort")
            raise ManualResolutionRequired(
                "Protected configuration conflicts; merge aborted"
            )
        if _merge_head(workspace) != base_sha:
            raise MergeError("Git reported conflicts without the expected merge state")
        return {"state": "conflicts", "conflicts": conflicts, "head": head}
    if merged.returncode or _merge_head(workspace) != base_sha:
        raise MergeError("Merge did not complete; inspect the retained workspace state")
    return {"state": "merged", "conflicts": [], "head": _commit(workspace)}


def finish_merge(workspace: Path, allowed_conflicts: list[str]) -> str:
    """Stage approved resolved paths and commit an existing two-parent merge.

    Other authorized product/document fixes must be staged by the caller. Clean
    changes inherited from main are already staged and remain part of the merge.
    The caller verifies the persisted branch/base/head before authorizing finish.
    """
    workspace = Path(workspace)
    _repository(workspace)
    if not _merge_head(workspace):
        raise MergeError("No merge is in progress")
    if _value(workspace, "rev-parse", "HEAD") != _value(
        workspace, "rev-parse", "ORIG_HEAD"
    ):
        raise MergeError("HEAD changed during merge resolution")
    allowed = set(allowed_conflicts)
    for name in allowed:
        path = PurePosixPath(name)
        if (
            not name
            or path.is_absolute()
            or ".." in path.parts
            or path.as_posix() != name
        ):
            raise MergeError(
                "Conflict paths must be exact repository-relative file names"
            )
        if _protected(name) or ".git" in path.parts:
            raise ManualResolutionRequired(
                "Protected paths cannot be resolved automatically"
            )
    unresolved = set(_conflicts(workspace))
    if not unresolved.issubset(allowed):
        raise MergeError("Merge contains conflicts outside the approved list")
    unstaged = set(_paths(workspace, "diff", "--name-only"))
    untracked = set(_paths(workspace, "ls-files", "--others", "--exclude-standard"))
    if not (unstaged | untracked).issubset(allowed):
        raise MergeError(
            "Additional local changes must be explicitly reviewed and staged"
        )
    # Check before staging, so rejected marker-bearing content remains unmerged.
    if (
        _git(workspace, "diff", "--check", check=False).returncode
        or _git(workspace, "diff", "--cached", "--check", check=False).returncode
    ):
        raise MergeError("Conflict markers or whitespace errors remain")
    tracked = set(_paths(workspace, "ls-files"))
    staged = set(_paths(workspace, "diff", "--cached", "--name-only"))
    if not allowed.issubset(tracked | untracked | staged):
        raise MergeError("Approved conflict path is missing from the merge")
    to_stage = allowed & (tracked | untracked)
    if to_stage:
        _git(workspace, "add", "--", *sorted(to_stage))
    if _conflicts(workspace):
        raise MergeError("Unresolved index entries remain")
    if _git(workspace, "diff", "--cached", "--check", check=False).returncode:
        raise MergeError("Conflict markers or whitespace errors remain in the index")
    return _commit(workspace)
