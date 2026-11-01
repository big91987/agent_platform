#!/usr/bin/env python3
"""Supported local entry point for the isolated Agent Team factory experiment."""

import argparse
import json
import sys
from pathlib import Path

from factory_store import Store


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--root",
        required=True,
        help="Private runtime directory, outside product worktrees",
    )
    sub = parser.add_subparsers(dest="command", required=True)
    init = sub.add_parser("init")
    init.add_argument("--repository", required=True)
    init.add_argument("--integration-branch", default="agent-team/experiment")
    init.add_argument("--concurrency", type=int, default=3)
    init.add_argument("--timeout-seconds", type=int, default=1800)
    init.add_argument("--codex", default="codex")
    init.add_argument("--model", default="")
    plan = sub.add_parser("plan")
    plan.add_argument("--request-file", required=True)
    plan.add_argument("--task-count", type=int, default=4)
    sub.add_parser("run")
    sub.add_parser("status")
    execution = sub.add_parser("configure-execution")
    execution.add_argument("--env-json", required=True)
    execution.add_argument("--network", choices=["enabled", "disabled"], required=True)
    concurrency = sub.add_parser("set-concurrency")
    concurrency.add_argument("--limit", type=int, required=True)
    dependencies = sub.add_parser("set-dependencies")
    dependencies.add_argument("--task", required=True)
    dependencies.add_argument("--depends-on-json", required=True)
    dependencies.add_argument("--reason", required=True)
    message = sub.add_parser("message")
    message.add_argument("--to", required=True)
    message.add_argument("--text", required=True)
    resume = sub.add_parser("resume")
    resume.add_argument("--task", required=True)
    resume.add_argument("--message", required=True)
    integrate = sub.add_parser("integrate")
    integrate.add_argument("--task", required=True)
    bundle = sub.add_parser("bundle")
    bundle.add_argument("--expected-head", required=True)
    serve = sub.add_parser("serve")
    serve.add_argument("--port", type=int, default=8977)
    args = parser.parse_args()
    if args.command == "init":
        from git_workspace import Workspace

        repo = Path(args.repository).resolve()
        root = Path(args.root).resolve()
        if root.is_relative_to(repo) or repo.is_relative_to(root):
            raise ValueError(
                "runtime directory and integration checkout must be separate siblings"
            )
        if not 30 <= args.timeout_seconds <= 14400:
            raise ValueError("timeout must be between 30 and 14400 seconds")
        Workspace(repo, root, args.integration_branch).check_target()
        store = Store(root)
        store.initialize(
            {
                "repository": str(repo),
                "integration_branch": args.integration_branch,
                "concurrency": args.concurrency,
                "timeout_seconds": args.timeout_seconds,
                "codex": args.codex,
                "model": args.model,
            }
        )
        result = {"initialized": True, "root": str(root)}
    elif args.command == "status":
        result = Store(args.root).snapshot()
    elif args.command == "configure-execution":
        Store(args.root).configure_execution(
            json.loads(args.env_json), args.network == "enabled"
        )
        result = {"configured": True}
    elif args.command == "set-concurrency":
        store = Store(args.root)
        store.set_concurrency(args.limit)
        result = {"concurrency": store.settings()["concurrency"]}
    elif args.command == "message":
        Store(args.root).message("user", args.to, args.text)
        result = {"accepted": True}
    elif args.command == "set-dependencies":
        Store(args.root).set_dependencies(
            args.task, json.loads(args.depends_on_json), args.reason
        )
        result = {"accepted": True, "task": args.task}
    elif args.command == "resume":
        Store(args.root).requeue(args.task, args.message)
        result = {"accepted": True, "task": args.task, "next": "run"}
    elif args.command == "serve":
        from board import serve

        serve(Store(args.root), args.port)
        return
    else:
        from runner import Factory

        factory = Factory(args.root)
        if args.command == "plan":
            result = factory.plan(Path(args.request_file).read_text(), args.task_count)
        elif args.command == "run":
            result = factory.run()
        elif args.command == "bundle":
            result = factory.bundle(args.expected_head)
        else:
            result = factory.integrate(args.task)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
