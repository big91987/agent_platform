#!/usr/bin/env python3
"""Fixed repository operations for the platform Workflow example, not CI routing."""

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path

from materials import install_material, verify_material


def git(workspace, *args):
    result = subprocess.run(
        [
            "git",
            "-C",
            str(workspace),
            *(
                [
                    "-c",
                    "credential.helper=",
                    "-c",
                    "credential.helper=!gh auth git-credential",
                ]
                if os.environ.get("GH_TOKEN") and args[0] in ("fetch", "push")
                else []
            ),
            *args,
        ],
        capture_output=True,
        text=True,
        check=True,
        timeout=90,
        env={**os.environ, "GIT_TERMINAL_PROMPT": "0"},
    )
    return result.stdout.strip()


def remote_repository(value):
    match = re.fullmatch(
        r"(?:https://github\.com/|git@github\.com:)([\w.-]+/[\w.-]+?)(?:\.git)?", value
    )
    if not match:
        raise ValueError("origin must be credential-free GitHub HTTPS or SSH")
    return match[1]


def branch_for(run):
    if not re.fullmatch(r"[a-f0-9]{32}", run.get("run_id", "")):
        raise ValueError("valid platform run_id required")
    return "workflow/" + run["run_id"]


def task_docs(run):
    branch_for(run)  # Validate the stable Run identity before constructing paths.
    return "docs/workflow/runs/" + run["run_id"]


def published_head(run, branch):
    for step in reversed(run.get("previous_results", [])):
        if step.get("node_id") != "publish":
            continue
        receipt = step.get("connector_receipt", {})
        if step.get("status") != "completed" or receipt.get("exit_code") != 0:
            raise ValueError(
                "Last publication failed or is uncertain; inspect it before integration"
            )
        try:
            output = json.loads(receipt.get("output", ""))
        except (ValueError, TypeError):
            raise ValueError(
                "Last publication receipt is invalid; no integration attempted"
            ) from None
        if (
            not isinstance(output, dict)
            or output.get("branch") != branch
            or output.get("pushed") is not True
            or not re.fullmatch(r"[a-f0-9]{40}", str(output.get("head", "")))
        ):
            raise ValueError(
                "Last publication receipt does not identify this candidate"
            )
        return output["head"]
    return None


def integrate_published_base(workspace, run, base, head):
    if git(workspace, "rev-parse", "HEAD") != head:
        raise ValueError(
            "Task HEAD differs from its last publication; inspect before preparing again"
        )
    git(workspace, "check-ref-format", "--branch", base)
    pending = subprocess.run(
        ["git", "-C", str(workspace), "rev-parse", "--verify", "MERGE_HEAD"],
        capture_output=True,
        text=True,
        timeout=15,
    )
    if pending.returncode == 0:
        # A lost prepare receipt must not fetch a moving base or replay the merge.
        target = pending.stdout.strip()
        if target != git(workspace, "rev-parse", "origin/" + base):
            raise ValueError(
                "Pending merge differs from the prepared base; inspect without replay"
            )
        conflicts = git(workspace, "diff", "--name-only", "--diff-filter=U", "-z")
        return {
            "integration_base": target,
            "integration_pending": True,
            "integration_conflicts": [name for name in conflicts.split("\0") if name],
        }
    if pending.returncode != 128:
        raise ValueError("Unable to inspect pending integration; no merge attempted")
    if git(workspace, "diff", "--cached", "--name-only"):
        raise ValueError(
            "Task has staged changes; preserve and inspect them before integration"
        )
    paths = subprocess.check_output(
        ["git", "-C", str(workspace), "diff", "HEAD", "--name-only", "-z"]
    ) + subprocess.check_output(
        [
            "git",
            "-C",
            str(workspace),
            "ls-files",
            "--others",
            "--exclude-standard",
            "-z",
        ]
    )
    prefix = task_docs(run).encode() + b"/"
    if any(not name.startswith(prefix) for name in paths.split(b"\0") if name):
        raise ValueError(
            "Task has uncommitted product changes; preserve them before integrating its base"
        )
    git(
        workspace,
        "fetch",
        "origin",
        "+refs/heads/" + base + ":refs/remotes/origin/" + base,
    )
    target = git(workspace, "rev-parse", "origin/" + base)
    try:
        git(workspace, "merge", "--no-commit", "--no-ff", target)
    except subprocess.CalledProcessError:
        # Content conflicts are preparation output, not a completed integration.
        # Preserve the index and files; development edits them before fixed tests.
        conflicts = git(workspace, "diff", "--name-only", "--diff-filter=U", "-z")
        if not conflicts or git(workspace, "rev-parse", "MERGE_HEAD") != target:
            raise
        return {
            "integration_base": target,
            "integration_pending": True,
            "integration_conflicts": [name for name in conflicts.split("\0") if name],
        }
    # Git may report already integrated; neither path commits or pushes.
    pending = subprocess.run(
        ["git", "-C", str(workspace), "rev-parse", "--verify", "MERGE_HEAD"],
        capture_output=True,
        text=True,
        timeout=15,
    )
    return {"integration_base": target, "integration_pending": pending.returncode == 0}


def prepare(workspace, run, base, scoped_docs=False):
    branch = branch_for(run)
    current = git(workspace, "branch", "--show-current")
    if current == branch:
        head = published_head(run, branch)
        integration = (
            integrate_published_base(workspace, run, base, head) if head else {}
        )
        return {
            "branch": branch,
            "head": git(workspace, "rev-parse", "HEAD"),
            "continued": True,
            **integration,
            **({"document_root": task_docs(run)} if scoped_docs else {}),
        }
    if git(workspace, "status", "--porcelain"):
        raise ValueError(
            "Workspace has existing changes; choose a clean dedicated checkout. Nothing was reset."
        )
    git(workspace, "check-ref-format", "--branch", base)
    git(
        workspace,
        "fetch",
        "origin",
        "+refs/heads/" + base + ":refs/remotes/origin/" + base,
    )
    git(workspace, "checkout", "-b", branch, "origin/" + base)
    return {
        "branch": branch,
        "base": base,
        "head": git(workspace, "rev-parse", "HEAD"),
        **({"document_root": task_docs(run)} if scoped_docs else {}),
    }


def prepare_trellis(workspace, executable, version):
    environment = dict(os.environ)
    environment.pop("GH_TOKEN", None)
    actual = subprocess.check_output(
        [executable, "--version"], text=True, timeout=15, env=environment
    ).strip()
    if actual != version:
        raise ValueError(
            "Trellis version differs from installed Connector; upgrade through the installer"
        )
    trellis = workspace / ".trellis"
    initialized = (trellis / "scripts/get_context.py").is_file()
    if not initialized:
        if trellis.exists():
            raise ValueError(
                "Incomplete existing Trellis installation; repair with Trellis before retrying"
            )
        subprocess.run(
            [
                executable,
                "init",
                "--codex",
                "--yes",
                "--skip-existing",
                "--user",
                "workflow",
            ],
            cwd=workspace,
            env=environment,
            check=True,
            capture_output=True,
            text=True,
            timeout=120,
        )
        # Native platform Skill selection is authoritative. Generated host adapters,
        # personal journals, and local Trellis task state are not shared artifacts.
        ignore = workspace / ".gitignore"
        if ignore.is_symlink():
            raise ValueError("Material ignore file must not be a symlink")
        original = ignore.read_text() if ignore.exists() else ""
        patterns = [
            "/.agents/",
            "/.codex/",
            "/.trellis/workspace/",
            "/.trellis/tasks/",
            "/.trellis/.runtime/",
            "/.trellis/.developer",
        ]
        missing = [line for line in patterns if line not in original.splitlines()]
        if missing:
            ignore.write_text(
                original.rstrip()
                + "\n\n# Local Trellis execution state\n"
                + "\n".join(missing)
                + "\n"
            )
        config = trellis / "config.yaml"
        config.write_text(
            config.read_text()
            + "\n# Git publishing belongs to the platform Connector.\nsession_auto_commit: false\ncodex:\n  dispatch_mode: inline\n"
        )
    version_file = trellis / ".version"
    if not version_file.is_file() or version_file.read_text().strip() != version:
        raise ValueError(
            "Trellis project asset version differs; inspect trellis update --dry-run, "
            "upgrade with the pinned CLI and preserve local changes before retrying"
        )
    subprocess.run(
        [sys.executable, str(trellis / "scripts/get_context.py"), "--mode", "packages"],
        cwd=workspace,
        env=environment,
        check=True,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return {
        "version": actual,
        "initialized": not initialized,
        "spec_root": ".trellis/spec",
    }


def check_publish_files(workspace, exclude_runtime=False):
    # Inspect what git add --all would include, including currently staged files.
    names = (
        subprocess.check_output(
            ["git", "-C", str(workspace), "ls-files", "-co", "--exclude-standard", "-z"]
        )
        .decode()
        .split("\0")
    )
    for name in set(filter(None, names)):
        if exclude_runtime and Path(name).is_relative_to("harness-output"):
            continue
        file = workspace / name
        if not file.exists():
            continue
        if not file.resolve().is_relative_to(workspace.resolve()):
            raise ValueError("Refusing workspace-external symlink: " + name)
        if file.is_dir():
            raise ValueError(
                "Submodule/directory staging requires separate review: " + name
            )
        if file.stat().st_size > 5 * 1024 * 1024:
            raise ValueError("Artifact exceeds publication limit: " + name)
        check_publish_blob(name, file.read_bytes())


def check_publish_blob(name, raw):
    private = re.compile(
        rb"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,})"
    )
    if Path(name).name in (
        ".env",
        "auth.json",
        "credentials.json",
        "id_rsa",
        "id_ed25519",
    ) or any(
        part
        in (
            "node_modules",
            ".codex",
            ".aws",
            ".workflow-evidence",
            ".workflow-input",
        )
        for part in Path(name).parts
    ):
        raise ValueError("Refusing local credentials/cache artifact: " + name)
    if len(raw) > 5 * 1024 * 1024:
        raise ValueError(
            "Artifact exceeds this example's 5 MiB publication limit: " + name
        )
    if private.search(raw):
        raise ValueError("Potential private credential in " + name)


def delivery_artifacts(workspace, run):
    directory = (workspace / task_docs(run)).resolve()
    if directory != workspace.resolve() / task_docs(run):
        raise ValueError("Task document directory escapes workspace")
    qa = next(
        (
            step
            for step in reversed(run.get("previous_results", []))
            if step.get("node_id") == "qa"
        ),
        None,
    )
    if (
        not qa
        or qa.get("status") != "completed"
        or qa.get("result", {}).get("route") != "next"
    ):
        raise ValueError("Missing completed QA handoff for this delivery")
    artifacts = qa["result"].get("artifacts", [])
    if not artifacts:
        raise ValueError("QA handoff has no evidence artifacts")
    for name in [*artifacts, task_docs(run) + "/pr.md"]:
        path = Path(name)
        actual = (workspace / path).resolve()
        if (
            path.is_absolute()
            or not actual.is_relative_to(directory)
            or not actual.is_file()
        ):
            raise ValueError(
                "Delivery artifact must be a file in this task's document directory: "
                + name
            )
        if not actual.stat().st_size:
            raise ValueError("Empty delivery artifact: " + name)


class ReviewedSourceChanged(ValueError):
    pass


def verify_reviewed_tree(workspace, run, tree):
    steps = run["previous_results"]

    positions = {
        step.get("node_id", step.get("node")): index for index, step in enumerate(steps)
    }
    if "code_review" not in positions:
        raise ValueError("Missing completed platform code-review receipt")
    review_index = positions["code_review"]
    review = steps[review_index]
    receipt = review.get("connector_receipt") or {}
    if (
        review.get("status") != "completed"
        or receipt.get("exit_code") != 0
        or receipt.get("log", {}).get("truncated")
    ):
        raise ValueError("Missing completed platform code-review receipt")
    summary = json.loads(receipt["output"])
    candidate = summary.get("candidate", "")
    if (
        summary.get("exit_code") != 0
        or summary.get("source_unchanged") is not True
        or not re.fullmatch("[0-9a-f]{40}", candidate)
    ):
        raise ValueError("Code-review receipt does not identify a reviewed candidate")

    directory = task_docs(run)
    pr = directory + "/pr.md"
    extensions = {
        ".md",
        ".json",
        ".csv",
        ".txt",
        ".html",
        ".png",
        ".jpg",
        ".jpeg",
        ".webp",
        ".pdf",
    }
    qa_steps = [
        step
        for step in steps[review_index + 1 :]
        if step.get("node_id", step.get("node")) == "qa"
    ]
    if (
        not qa_steps
        or qa_steps[-1].get("status") != "completed"
        or (qa_steps[-1].get("result") or {}).get("route") != "next"
    ):
        raise ValueError("Missing completed QA handoff after the current code review")
    latest_artifacts = set(qa_steps[-1]["result"].get("artifacts", []))
    evidence = set()
    # Report can request additional QA for the same candidate. Retain evidence
    # already handed off against that review; a new review resets the scope.
    for step in qa_steps:
        if step.get("status") != "completed":
            continue
        for name in (step.get("result") or {}).get("artifacts", []):
            path = Path(name)
            if (
                path.is_relative_to(directory)
                and ".." not in path.parts
                and path.as_posix() == name
                and path.suffix.lower() in extensions
            ):
                evidence.add(name)

    # Use Git's tree modes and blob IDs, so executable-bit changes, links,
    # staged content and deletions cannot hide behind equal file bytes.
    raw = git(
        workspace, "diff-tree", "-r", "-z", "--raw", "--no-renames", candidate, tree
    )
    fields = raw.rstrip("\0").split("\0") if raw else []
    changed = []
    added_evidence = set()
    for metadata, name in zip(fields[::2], fields[1::2], strict=True):
        _, new_mode, _, _, status = metadata.split()
        if new_mode == "100644" and name == pr and status in ("A", "M"):
            continue
        if new_mode == "100644" and status == "A" and name in evidence:
            added_evidence.add(name)
            continue
        changed.append(name)
    if changed:
        raise ReviewedSourceChanged(
            "Reviewed source changed. Return through development, fixed tests and architecture review: "
            + ", ".join(changed[:20])
        )
    if latest_artifacts.isdisjoint(added_evidence):
        raise ValueError(
            "Latest QA handoff has no new evidence for the reviewed candidate"
        )


def publish(workspace, run, scoped_docs=False, materials=False, reviewed_source=False):
    if materials:
        verify_material(workspace, run)
    branch = branch_for(run)
    if git(workspace, "branch", "--show-current") != branch:
        raise ValueError(
            "Current branch does not belong to this Run; no commit or push performed"
        )
    if scoped_docs:
        delivery_artifacts(workspace, run)
    else:  # Frozen installations keep their previous path contract until upgraded.
        for name in ("docs/workflow/qa.md", "docs/workflow/pr.md"):
            if not (workspace / name).is_file():
                raise ValueError("Missing delivery artifact: " + name)
    if git(workspace, "diff", "--name-only", "--diff-filter=U"):
        raise ValueError(
            "Base integration has unresolved conflicts; no publication attempted"
        )
    check_publish_files(workspace, exclude_runtime=reviewed_source)
    if reviewed_source:
        from architecture_review import load_candidate_runtime

        load_candidate_runtime().stage_candidate(workspace)
        verify_reviewed_tree(workspace, run, git(workspace, "write-tree"))
    else:
        git(workspace, "add", "--all")
    pending_merge = subprocess.run(
        ["git", "-C", str(workspace), "rev-parse", "--verify", "MERGE_HEAD"],
        capture_output=True,
        text=True,
        timeout=15,
    )
    if (
        git(workspace, "diff", "--cached", "--name-only")
        or pending_merge.returncode == 0
    ):
        title = run.get("input", "Workflow delivery").splitlines()[0][:120]
        git(workspace, "commit", "-m", title)
    head = git(workspace, "rev-parse", "HEAD")
    if reviewed_source:
        verify_reviewed_tree(workspace, run, head)
    git(workspace, "push", "origin", head + ":refs/heads/" + branch)
    return {
        "branch": branch,
        "head": head,
        "pushed": True,
    }


def regression_input(workspace, run, command):
    from architecture_review import load_candidate_runtime

    runtime = load_candidate_runtime()
    directory = task_docs(run)
    reports = {
        directory + "/development.md",
        directory + "/development-note.md",
        directory + "/implementation.md",
        directory + "/development/implementation.md",
    }
    candidate = runtime.candidate_commit(workspace)
    tree = runtime.git(workspace, "ls-tree", "-r", "-z", candidate)
    # Reuse applies only to this Run's implementation note. Preserve content,
    # executable bits, paths, config and all other inputs in the fingerprint.
    entries = [entry for entry in tree.split(b"\0") if entry]
    fingerprint = dict(
        commit=git(workspace, "rev-parse", "HEAD"),
        integrations=git(workspace, "rev-parse", "--revs-only", "MERGE_HEAD"),
        command=command,
        files=runtime.candidate_snapshot(workspace),
    )
    for entry in entries[:]:
        mode, _, raw_path = entry.partition(b"\t")
        path = os.fsdecode(raw_path)
        if path in reports and mode.startswith(b"100644 blob ") and not (workspace / path).is_symlink():
            fingerprint["files"].pop(path, None)
            entries.remove(entry)
    data = json.dumps(fingerprint, sort_keys=True).encode()
    return hashlib.sha256(data + b"\0" + b"\0".join(entries)).hexdigest()


def previous_regression(run, digest):
    for step in reversed(run.get("previous_results", [])):
        if step.get("node_id") != "tests":
            continue
        receipt = step.get("connector_receipt") or {}
        if (
            step.get("status") != "completed"
            or (step.get("result") or {}).get("route") != "next"
            or receipt.get("kind") != "command"
            or receipt.get("exit_code") != 0
            or (receipt.get("log") or {}).get("truncated") is not False
        ):
            break
        try:
            header = json.loads(receipt["output"].splitlines()[0])
            if header.get("fixed_test_input") == digest:
                seq = step["seq"]
                if header.get("mode") == "executed" and type(seq) is int and seq > 0:
                    return seq
                origin = header.get("original_seq")
                if header.get("mode") == "reused" and type(origin) is int and 0 < origin < seq:
                    return origin
        except (AttributeError, IndexError, KeyError, TypeError, ValueError):
            pass
        break
    return None


def run_tests(workspace, run, command, base="main", *, reuse_pass=False):
    verify_material(workspace, run)
    if (
        not isinstance(command, list)
        or not command
        or any(not isinstance(x, str) or not x or "\0" in x for x in command)
    ):
        raise ValueError("Fixed test command must be a non-empty argv array")
    pending = subprocess.run(
        ["git", "-C", str(workspace), "rev-parse", "--verify", "MERGE_HEAD"],
        capture_output=True,
        text=True,
        timeout=15,
    )
    conflicts = (
        [
            name
            for name in git(
                workspace, "diff", "--name-only", "--diff-filter=U", "-z"
            ).split("\0")
            if name
        ]
        if pending.returncode == 0
        else []
    )
    digest = regression_input(workspace, run, command) if reuse_pass and not conflicts else None
    if digest:
        original = previous_regression(run, digest)
        print(json.dumps({
            "fixed_test_input": digest,
            "mode": "reused" if original else "executed",
            **({"original_seq": original} if original else {}),
        }), flush=True)
        if original:
            return 0
    if conflicts:
        branch = branch_for(run)
        if (
            git(workspace, "branch", "--show-current") != branch
            or published_head(run, branch) != git(workspace, "rev-parse", "HEAD")
            or git(workspace, "rev-parse", "MERGE_HEAD")
            != git(workspace, "rev-parse", "origin/" + base)
        ):
            raise ValueError("Conflict resolution is not the prepared task integration")
        # Native Agents can edit files but cannot write .git. The configured
        # test Connector records their resolution; it never commits or pushes.
        git(workspace, "--literal-pathspecs", "diff", "--check", "--", *conflicts)
        git(workspace, "--literal-pathspecs", "add", "--", *conflicts)
        print(json.dumps({"integration_resolution_staged": conflicts}), flush=True)
    environment = dict(os.environ)
    environment.pop("GH_TOKEN", None)
    code = subprocess.run(
        command, cwd=workspace, stdin=subprocess.DEVNULL, env=environment
    ).returncode
    if code == 0 and digest and regression_input(workspace, run, command) != digest:
        raise ValueError("Fixed regression inputs changed during execution")
    return code


def prepare_material(workspace, run, repository):
    receipt = install_material(workspace, run, repository)
    if receipt:
        ignore = workspace / ".gitignore"
        if ignore.is_symlink():
            raise ValueError("Material ignore file must not be a symlink")
        original = ignore.read_text() if ignore.exists() else ""
        if "/.workflow-input/" not in original.splitlines():
            ignore.write_text(
                original.rstrip()
                + "\n\n# Private, pinned Workflow input materials\n/.workflow-input/\n"
            )
    return receipt


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "operation",
        choices=["prepare", "publish", "verify", "check-reviewed", "review-feedback"],
    )
    parser.add_argument("--repository")
    parser.add_argument("--reviewed-source", action="store_true")
    parser.add_argument("--base", default="main")
    parser.add_argument("--task-docs", action="store_true")
    parser.add_argument("--materials", action="store_true")
    parser.add_argument("--test-command")
    parser.add_argument("--reuse-passed-tests", action="store_true")
    parser.add_argument("--trellis-executable")
    parser.add_argument("--trellis-version")
    args = parser.parse_args()
    if args.operation == "review-feedback":
        from architecture_review import review_feedback

        run = json.load(sys.stdin)
        if Path(run["workspace_path"]).resolve() != Path.cwd():
            raise ValueError("Run workspace and process directory differ")
        feedback = review_feedback(run)
        print(json.dumps(feedback, ensure_ascii=False))
        raise SystemExit(0 if feedback["reason"] == "review_findings" else 20)
    if args.operation == "check-reviewed":
        from architecture_review import load_candidate_runtime

        run = json.load(sys.stdin)
        if Path(run["workspace_path"]).resolve() != Path.cwd():
            raise ValueError("Run workspace and process directory differ")
        try:
            verify_reviewed_tree(Path.cwd(), run, load_candidate_runtime().candidate_commit())
        except ReviewedSourceChanged as error:
            print(error)
            raise SystemExit(10)
        return
    if not args.repository:
        parser.error("--repository is required")
    run = json.load(sys.stdin)
    workspace = Path(run["workspace_path"]).resolve()
    if workspace != Path.cwd().resolve():
        raise ValueError("Run workspace and process directory differ")
    if (
        remote_repository(git(workspace, "remote", "get-url", "origin"))
        != args.repository
    ):
        raise ValueError("Workspace origin differs from configured repository")
    if args.operation == "verify":
        if not args.test_command:
            raise ValueError("Fixed test argv required")
        raise SystemExit(
            run_tests(
                workspace, run, json.loads(args.test_command), args.base,
                reuse_pass=args.reuse_passed_tests,
            )
        )
    result = (
        prepare(workspace, run, args.base, args.task_docs)
        if args.operation == "prepare"
        else publish(
            workspace, run, args.task_docs, args.materials, args.reviewed_source
        )
    )
    if args.operation == "prepare" and args.materials:
        material = prepare_material(workspace, run, args.repository)
        if material:
            result["material"] = material
    if args.operation == "prepare" and args.trellis_executable:
        result["trellis"] = prepare_trellis(
            workspace, args.trellis_executable, args.trellis_version
        )
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        # Keep actionable Git stderr; credentials are env references, never argv.
        print(error.stderr or str(error), file=sys.stderr)
        raise SystemExit(error.returncode)
