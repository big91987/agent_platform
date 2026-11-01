"""Git operations owned by the experiment controller, never a product Worker."""

import fnmatch
import hashlib
import re
import subprocess
from pathlib import Path


def git(repo, *args):
    result = subprocess.run(
        ["git", *map(str, args)], cwd=repo, capture_output=True, text=True
    )
    if result.returncode:
        raise subprocess.CalledProcessError(
            result.returncode, result.args, result.stdout, result.stderr
        )
    return result.stdout.strip()


def validate_experiment_branch(branch):
    if not isinstance(branch, str) or not re.fullmatch(
        r"agent-team/[a-z][a-z0-9-]*", branch
    ):
        raise ValueError("integration must be a dedicated agent-team/<name> branch")


class Workspace:
    def __init__(self, repo, root, integration):
        self.repo = Path(repo).resolve()
        self.root = Path(root).resolve()
        validate_experiment_branch(integration)
        self.integration = integration

    def check_target(self):
        if git(self.repo, "branch", "--show-current") != self.integration:
            raise ValueError(
                "repository is not on the configured experimental integration branch"
            )
        if git(self.repo, "status", "--porcelain", "--untracked-files=all"):
            raise ValueError(
                "integration worktree must be clean; existing work is preserved"
            )

    def prepare(self, task_id, attempt):
        if (
            not re.fullmatch(r"[a-z][a-z0-9-]{0,63}", task_id)
            or type(attempt) is not int
            or attempt < 1
        ):
            raise ValueError("invalid task execution")
        self.check_target()
        base = git(self.repo, "rev-parse", "HEAD")
        branch = f"agent-team/task-{task_id}-{attempt}"
        path = self.root / "agent-team-worktrees" / f"{task_id}-{attempt}"
        path.parent.mkdir(parents=True, exist_ok=True)
        if path.exists() or git(self.repo, "branch", "--list", branch):
            raise ValueError(
                "task branch/worktree already exists; refusing uncertain replay"
            )
        git(self.repo, "worktree", "add", "-b", branch, path, base)
        return {"path": path, "branch": branch, "base": base}

    def commit(self, task, work):
        root = Path(work["path"]).resolve()
        if (
            git(root, "branch", "--show-current") != work["branch"]
            or git(root, "rev-parse", "HEAD") != work["base"]
        ):
            raise ValueError(
                "Worker changed branch or committed independently; preserving evidence"
            )
        changed = set(
            git(root, "diff", "--name-only", "--no-renames", "-z", "HEAD").split("\0")
        )
        changed.update(
            git(root, "ls-files", "--others", "--exclude-standard", "-z").split("\0")
        )
        changed.discard("")
        self.validate_paths(task, root, changed)
        if not changed:
            return work["base"]
        git(root, "add", "--", *sorted(changed))
        git(root, "commit", "-m", f"agent-team: {task['title']}")
        candidate = git(root, "rev-parse", "HEAD")
        committed = set(
            git(
                root,
                "diff",
                "--name-only",
                "--no-renames",
                "-z",
                work["base"],
                candidate,
            ).split("\0")
        ) - {""}
        self.validate_paths(task, root, committed)
        if git(root, "branch", "--show-current") != work["branch"] or git(
            root, "status", "--porcelain", "--untracked-files=all"
        ):
            raise ValueError(
                "commit hook changed branch or left uncommitted changes; candidate preserved but not accepted"
            )
        return candidate

    @staticmethod
    def validate_paths(task, root, changed):
        for relative in changed:
            target = root / relative
            if not target.resolve().is_relative_to(root):
                raise ValueError("changed path escapes worktree")
            if not any(
                fnmatch.fnmatchcase(relative, pattern)
                or relative.startswith(pattern.rstrip("/") + "/")
                for pattern in task["write_paths"]
            ):
                raise ValueError(f"out-of-scope change preserved: {relative}")
            if target.suffix.lower() in (
                ".pem",
                ".key",
                ".sqlite",
                ".sqlite3",
            ) or target.name in (".env", "auth.json", "credentials.json"):
                raise ValueError(
                    f"private runtime file cannot be committed: {relative}"
                )

    def integrate(self, commit):
        self.check_target()
        if not re.fullmatch(r"[a-f0-9]{40}", commit):
            raise ValueError("integration requires an exact commit")
        git(self.repo, "merge", "--no-ff", "--no-edit", commit)
        return git(self.repo, "rev-parse", "HEAD")

    def bundle(self, expected_head):
        self.check_target()
        if (
            not re.fullmatch(r"[a-f0-9]{40}", expected_head)
            or git(self.repo, "rev-parse", "HEAD") != expected_head
        ):
            raise ValueError("export requires the selected exact integration HEAD")
        base = git(self.repo, "rev-parse", "refs/remotes/origin/" + self.integration)
        if base == expected_head:
            raise ValueError("integration has no new commits beyond tracked origin")
        git(self.repo, "merge-base", "--is-ancestor", base, expected_head)
        directory = self.root / "bundles"
        directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        path = directory / (expected_head + ".bundle")
        if not path.exists():
            git(
                self.repo,
                "bundle",
                "create",
                path,
                "refs/heads/" + self.integration,
                "^" + base,
            )
        git(self.repo, "bundle", "verify", path)
        if git(self.repo, "bundle", "list-heads", path).split() != [
            expected_head,
            "refs/heads/" + self.integration,
        ]:
            raise ValueError("bundle contains a different HEAD or additional refs")
        return {
            "bundle": str(path),
            "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            "branch": self.integration,
            "head": expected_head,
            "base": base,
        }
