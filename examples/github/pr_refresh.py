"""Automatic refresh of registered, ready PRs; final merge remains human."""

import argparse
import json
import os
from pathlib import Path

from product_scope import allowed_product_path
from requirements import github


def dispatch(settings, number, after):
    return github(
        f"repos/{settings['repository']}/actions/workflows/agent-platform.yml/dispatches",
        {"ref": "main", "inputs": {"issue": str(number), "after": after}},
    )


def registered_prs(settings, prs):
    from pipeline import task_directory

    ready = {
        p["html_url"]: p
        for p in prs
        if p["state"] == "open"
        and not p["draft"]
        and p["base"]["ref"] == "main"
        and (p["head"].get("repo") or {}).get("full_name") == settings["repository"]
    }
    result = {}
    for path in (Path(settings["registry"]) / "issues").glob("*.json"):
        task = json.loads(path.read_text())
        receipt = task_directory(settings, Path(task["workspace"])) / "delivery.json"
        if not receipt.exists():
            continue
        pr = ready.get(json.loads(receipt.read_text())["pr_url"])
        if pr and pr["head"]["ref"] == task["branch"]:
            result[pr["number"]] = int(path.stem)
    return result


def ready_issues(settings, prs):
    return sorted(registered_prs(settings, prs).values())


def refresh(settings, prs):
    import maintenance

    registered = registered_prs(settings, prs)
    for number in sorted(registered.values()):
        dispatch(settings, number, "integrate")
        print(f"Requested refresh for Issue #{number}", flush=True)
    unregistered = [
        pr
        for pr in prs
        if maintenance.eligible(settings, pr) and pr["number"] not in registered
    ]
    if not unregistered:
        return
    base = github(f"repos/{settings['repository']}/commits/main")["sha"]
    for pr in unregistered:
        passed = maintenance.verified(settings, pr, base)
        status(
            settings,
            pr["head"]["sha"],
            "success" if passed else "failure",
            "Maintainer checks passed against main " + base[:12]
            if passed
            else "Maintainer verification required; run pr_refresh.py --maintenance-pr "
            + str(pr["number"]),
        )
        print(
            f"PR #{pr['number']}: {'maintainer checks current' if passed else 'needs maintainer verification; no Agent dispatched'}",
            flush=True,
        )


def matches_review(task, pr_head, main_head, inspected_head):
    integration = task.get("integration", {})
    return bool(
        integration.get("state") == "review"
        and integration.get("base_sha") == main_head
        and integration.get("published_head") == pr_head == inspected_head
    )


def current_pr(settings, task):
    from pipeline import task_directory

    delivery = json.loads(
        (
            task_directory(settings, Path(task["workspace"])) / "delivery.json"
        ).read_text()
    )
    number = int(delivery["pr_url"].rstrip("/").split("/")[-1])
    pr = github(f"repos/{settings['repository']}/pulls/{number}")
    if (
        pr["state"] != "open"
        or pr["draft"]
        or pr["base"]["ref"] != "main"
        or (pr["head"].get("repo") or {}).get("full_name") != settings["repository"]
        or pr["head"]["ref"] != task["branch"]
    ):
        raise ValueError("Refresh requires a registered, ready, same-repository PR")
    return pr


def status(settings, sha, state, description):
    github(
        f"repos/{settings['repository']}/statuses/{sha}",
        {"state": state, "context": "pipeline/refresh", "description": description},
    )


def _merge_in_progress(workspace):
    from pipeline import run

    path = Path(run(["git", "rev-parse", "--git-path", "MERGE_HEAD"], workspace))
    return (path if path.is_absolute() else workspace / path).exists()


def _matches_commit(workspace, parents, message, tree=None):
    from pipeline import run

    if run(["git", "show", "-s", "--format=%P", "HEAD"], workspace).split() != parents:
        return False
    if run(["git", "show", "-s", "--format=%s", "HEAD"], workspace) != message:
        return False
    return tree is None or run(["git", "rev-parse", "HEAD^{tree}"], workspace) == tree


def _changed_paths(workspace):
    from pipeline import run

    paths = run(["git", "diff", "--name-only", "-z", "HEAD"], workspace).split("\0")
    paths += run(
        ["git", "ls-files", "--others", "--exclude-standard", "-z"], workspace
    ).split("\0")
    return set(filter(None, paths))


def start(settings, task, task_path, client, run_id):
    """Prepare a saved refresh round; the caller holds the registered issue lock."""
    from pipeline import delivery_token

    if not run_id.isdigit():
        raise ValueError("Refresh requires a workflow run ID")
    os.environ["GH_TOKEN"] = delivery_token(settings)
    pr = current_pr(settings, task)
    try:
        return _start(settings, task, task_path, client, run_id, pr)
    except Exception:
        status(
            settings,
            pr["head"]["sha"],
            "failure",
            "Refresh failed; inspect the integrate Action logs and retry",
        )
        raise


def _sync_remote_head(settings, task, pr, workspace):
    """Accept only an exact remote fast-forward; preserve all local work."""
    from pipeline import run

    if run(["git", "status", "--porcelain", "--untracked-files=all"], workspace):
        raise ValueError(
            "PR advanced but workspace has local changes; preserve and reconcile them first"
        )
    for name in (
        "MERGE_HEAD",
        "CHERRY_PICK_HEAD",
        "REVERT_HEAD",
        "rebase-merge",
        "rebase-apply",
        "sequencer",
    ):
        path = Path(run(["git", "rev-parse", "--git-path", name], workspace))
        if (path if path.is_absolute() else workspace / path).exists():
            raise ValueError("Finish the existing Git operation before syncing the PR")
    run(["git", "fetch", "origin", "refs/heads/" + task["branch"]], workspace)
    expected = pr["head"]["sha"]
    if run(["git", "rev-parse", "FETCH_HEAD"], workspace) != expected:
        raise ValueError(
            "Remote PR changed during fetch; retry against its latest head"
        )
    if run(["git", "merge-base", "HEAD", expected], workspace) != run(
        ["git", "rev-parse", "HEAD"], workspace
    ):
        raise ValueError(
            "Workspace diverged from PR; local commits require maintainer reconciliation"
        )
    if current_pr(settings, task)["head"]["sha"] != expected:
        raise ValueError("PR changed during sync; retry against its latest head")
    run(
        [
            "git",
            "-c",
            "core.hooksPath=/dev/null",
            "merge",
            "--ff-only",
            "--no-edit",
            "--no-overwrite-ignore",
            expected,
        ],
        workspace,
    )


def _start(settings, task, task_path, client, run_id, pr):
    from pipeline import run, save
    from pr_merge import MERGE_MESSAGE, prepare_merge

    workspace = Path(task["workspace"])
    previous = task.get("integration", {})
    if previous.get("state") == "review":
        return None
    if previous.get("state") in ("development", "qa"):
        stage = previous["state"]
        if task.get("active_stage") != stage or task.get("round") != previous.get(
            "initial_round", task.get("round")
        ):
            return None
        receipt = task["stages"].get(stage, {})
        # A saved intention without an invocation still needs to be dispatched.
        # Same workflow retries reconnect using the existing idempotency key.
        if task.get("round") == run_id or receipt.get("round") != task.get("round"):
            return stage
        return None
    if previous.get("state") != "preparing":
        base = github(f"repos/{settings['repository']}/commits/main")["sha"]
        if (
            previous.get("state") == "complete"
            and previous.get("base_sha") == base
            and previous.get("published_head") == pr["head"]["sha"]
        ):
            return None
        for receipt in task["stages"].values():
            if (
                client.conversation(receipt["conversation_id"])["conversation"][
                    "status"
                ]
                != "idle"
            ):
                raise ValueError(
                    "Finish the active conversation before refreshing this PR"
                )
        for receipt in task["stages"].values():
            client.workspace_access(receipt["conversation_id"], read_only=True)
        if run(["git", "branch", "--show-current"], workspace) != task["branch"]:
            raise ValueError("Workspace left the registered PR branch")
        if run(["git", "rev-parse", "HEAD"], workspace) != pr["head"]["sha"]:
            _sync_remote_head(settings, task, pr, workspace)
        if _merge_in_progress(workspace):
            raise ValueError("An unregistered merge is already in progress")
        if any(not name.startswith("docs/") for name in _changed_paths(workspace)):
            raise ValueError("Workspace has uncommitted non-document changes")
        task["integration"] = {
            "state": "preparing",
            "base_sha": base,
            "expected_head": pr["head"]["sha"],
            "pre_merge_head": pr["head"]["sha"],
            "documents_saved": False,
            "initial_round": run_id,
        }
        task["round"] = run_id
        task.pop("transition", None)
        save(task_path, task)
    integration = task["integration"]
    if pr["head"]["sha"] != integration["expected_head"]:
        raise ValueError("PR changed during refresh preparation")
    if run(["git", "branch", "--show-current"], workspace) != task["branch"]:
        raise ValueError("Workspace left the registered PR branch")
    status(
        settings,
        pr["head"]["sha"],
        "pending",
        "Syncing main, then independent QA and review",
    )
    docs_message = "Preserve previous verification evidence"
    if not integration["documents_saved"]:
        head = run(["git", "rev-parse", "HEAD"], workspace)
        if head != integration["expected_head"]:
            if not integration.get("docs_tree") or not _matches_commit(
                workspace,
                [integration["expected_head"]],
                docs_message,
                integration["docs_tree"],
            ):
                raise ValueError("Unexpected HEAD while preserving review documents")
        else:
            changed = _changed_paths(workspace)
            if any(not name.startswith("docs/") for name in changed):
                raise ValueError("Workspace has uncommitted non-document changes")
            if changed:
                run(["git", "add", "--", "docs"], workspace)
                tree = run(["git", "write-tree"], workspace)
                if integration.get("docs_tree") not in (None, tree):
                    raise ValueError("Review documents changed during commit recovery")
                integration["docs_tree"] = tree
                save(task_path, task)
                run(
                    [
                        "git",
                        "-c",
                        "core.hooksPath=/dev/null",
                        "-c",
                        "commit.gpgsign=false",
                        "commit",
                        "-m",
                        docs_message,
                    ],
                    workspace,
                )
        integration["pre_merge_head"] = run(["git", "rev-parse", "HEAD"], workspace)
        integration["documents_saved"] = True
        save(task_path, task)
    run(["git", "fetch", "origin", "main"], workspace)
    head = run(["git", "rev-parse", "HEAD"], workspace)
    if head != integration["pre_merge_head"]:
        if _merge_in_progress(workspace) or not _matches_commit(
            workspace,
            [integration["pre_merge_head"], integration["base_sha"]],
            MERGE_MESSAGE,
        ):
            raise ValueError("Unexpected HEAD while recovering the main merge")
        if _changed_paths(workspace):
            raise ValueError("Workspace changed after the recovered merge")
        merged = {"state": "merged", "head": head, "conflicts": []}
    else:
        merged = prepare_merge(workspace, task["branch"], head, integration["base_sha"])
    stage = "development" if merged["state"] == "conflicts" else "qa"
    integration.update(
        state=stage, merge_head=merged["head"], conflicts=merged["conflicts"]
    )
    task["active_stage"] = stage
    save(task_path, task)
    return stage


def finish_conflicts(task, workspace, task_path, settings=None):
    """Finish or recognize a saved merge under the issue lock, then persist QA."""
    from pipeline import run, save
    from pr_merge import MERGE_MESSAGE, _protected, finish_merge

    workspace = Path(workspace)
    integration = task.get("integration", {})
    if integration.get("state") != "development":
        return
    if run(["git", "branch", "--show-current"], workspace) != task["branch"]:
        raise ValueError("Workspace left the registered PR branch")
    head = run(["git", "rev-parse", "HEAD"], workspace)
    if not _merge_in_progress(workspace):
        if not _matches_commit(
            workspace,
            [integration["merge_head"], integration["base_sha"]],
            MERGE_MESSAGE,
        ):
            raise ValueError(
                "Expected merge is missing; refusing to reuse old verification"
            )
        if _changed_paths(workspace):
            raise ValueError("Workspace changed after the recovered merge")
    else:
        if (
            head != integration["merge_head"]
            or run(["git", "rev-parse", "MERGE_HEAD"], workspace)
            != integration["base_sha"]
        ):
            raise ValueError("Merge head or base changed while resolving conflicts")
        # Clean main changes are already staged. The Agent cannot stage or commit;
        # explicitly validate and stage only its additional product/document work.
        paths = set(
            filter(
                None, run(["git", "diff", "--name-only", "-z"], workspace).split("\0")
            )
        )
        paths.update(
            filter(
                None,
                run(
                    ["git", "ls-files", "--others", "--exclude-standard", "-z"],
                    workspace,
                ).split("\0"),
            )
        )
        if any(
            not allowed_product_path(name, settings) or _protected(name)
            for name in paths
        ):
            raise ValueError("Conflict repair modified protected files")
        # The persisted original merge_head/base/conflicts are also the finish
        # intent: if commit succeeds but save does not, verify its parents above.
        save(task_path, task)
        if paths:
            run(["git", "--literal-pathspecs", "add", "--", *sorted(paths)], workspace)
        head = finish_merge(workspace, integration["conflicts"])
    integration.update(merge_head=head, conflicts=[], state="qa")
    save(task_path, task)


def published(settings, task, path, number, head):
    from pipeline import save

    integration = task["integration"]
    if (
        integration.get("state") == "complete"
        and integration.get("published_head") == head
    ):
        return
    integration["published_head"] = head
    main = github(f"repos/{settings['repository']}/commits/main")["sha"]
    integration["state"] = "review" if main == integration["base_sha"] else "superseded"
    save(path, task)
    status(settings, head, "pending", "QA completed; independent review pending")
    dispatch(
        settings,
        number,
        "code_review" if integration["state"] == "review" else "integrate",
    )


def reviewed(settings, task, path, number, inspected_head):
    from pipeline import save

    pr = current_pr(settings, task)
    main = github(f"repos/{settings['repository']}/commits/main")["sha"]
    if (
        task["integration"].get("state") == "complete"
        and task["integration"].get("published_head")
        == pr["head"]["sha"]
        == inspected_head
        and task["integration"].get("base_sha") == main
    ):
        return
    if matches_review(task, pr["head"]["sha"], main, inspected_head):
        status(
            settings,
            inspected_head,
            "success",
            "QA and review completed; human merge decision required",
        )
        task["integration"]["state"] = "complete"
        save(path, task)
    else:
        task["integration"]["state"] = "superseded"
        save(path, task)
        status(
            settings,
            pr["head"]["sha"],
            "pending",
            "Base or PR changed; fresh verification required",
        )
        dispatch(settings, number, "integrate")


def main():
    from pipeline import delivery_token

    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument(
        "--maintenance-pr",
        type=int,
        help="Run maintainer-owned engineering checks; never dispatch an Agent",
    )
    parser.add_argument(
        "--workspace", type=Path, help="Clean checkout of the exact PR head"
    )
    args = parser.parse_args()
    settings = json.loads(Path(args.config).read_text())["pipeline"]
    if settings["repository"] != os.environ["GH_REPO"]:
        raise ValueError("Unregistered repository")
    os.environ["GH_TOKEN"] = delivery_token(settings)
    if args.maintenance_pr is not None:
        import maintenance

        if args.workspace is None:
            parser.error("--maintenance-pr requires --workspace")
        pr = github(f"repos/{settings['repository']}/pulls/{args.maintenance_pr}")
        if registered_prs(settings, [pr]):
            raise ValueError(
                "Registered product PRs must use Agent integration QA/review"
            )
        maintenance.verify(settings, args.maintenance_pr, args.workspace)
        return
    prs = github(
        f"repos/{settings['repository']}/pulls?state=open&per_page=100", paginate=True
    )
    refresh(settings, prs)


if __name__ == "__main__":
    main()
