"""Single-user Agent Team prototype. Starts empty; only users submit requirements."""

import argparse
import json
import secrets
import signal
from pathlib import Path

from git_workspace import Workspace, git
from team_store import TeamStore


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", required=True)
    sub = parser.add_subparsers(dest="command", required=True)
    bootstrap = sub.add_parser(
        "bootstrap", help="Initialize only a genuinely empty test repository"
    )
    bootstrap.add_argument("--repository", required=True)
    bootstrap.add_argument("--branch", default="agent-team/playground")
    init = sub.add_parser("init")
    init.add_argument("--repository", required=True)
    init.add_argument("--codex", required=True)
    init.add_argument("--model", default="gpt-5.6-sol")
    init.add_argument("--workers", type=int, default=3)
    init.add_argument("--branch", default="agent-team/playground")
    init.add_argument(
        "--env-file", required=True, help="Non-secret execution environment JSON"
    )
    start = sub.add_parser("serve")
    start.add_argument("--port", type=int, default=8991)
    sub.add_parser("status")
    args = parser.parse_args()
    store = TeamStore(args.root)
    if args.command == "bootstrap":
        import subprocess

        from git_workspace import validate_experiment_branch

        repo = Path(args.repository).resolve()
        validate_experiment_branch(args.branch)
        exists = (
            subprocess.run(
                ["git", "rev-parse", "--verify", "HEAD"], cwd=repo, capture_output=True
            ).returncode
            == 0
        )
        if exists or git(repo, "status", "--porcelain", "--untracked-files=all"):
            raise ValueError("只允许初始化没有提交、没有已有文件的空仓库")
        git(repo, "switch", "--orphan", args.branch)
        for name in ["README.md", "AGENTS.md", ".gitignore"]:
            (repo / name).write_bytes(
                (Path(__file__).parent / "team_seed" / name).read_bytes()
            )
        git(repo, "config", "user.name", "Agent Team Prototype")
        git(repo, "config", "user.email", "agent-team@example.invalid")
        git(repo, "add", "README.md", "AGENTS.md", ".gitignore")
        git(
            repo, "commit", "-m", "chore: initialize independent development playground"
        )
        print(
            json.dumps(
                {
                    "bootstrap_commit": git(repo, "rev-parse", "HEAD"),
                    "branch": args.branch,
                }
            )
        )
    elif args.command == "init":
        repo = Path(args.repository).resolve()
        if store.root.is_relative_to(repo) or repo.is_relative_to(store.root):
            raise ValueError("运行库与产品仓库必须为独立目录")
        Workspace(repo, store.root, args.branch).check_target()
        store.initialize(
            {
                "repository": str(repo),
                "integration_branch": args.branch,
                "codex": str(Path(args.codex).resolve()),
                "model": args.model,
                "concurrency": args.workers,
                "timeout_seconds": 1800,
                "network_access": True,
                "execution_env": json.loads(Path(args.env_file).read_text()),
            }
        )
        key = store.root / "http-key"
        key.write_text(secrets.token_urlsafe(32))
        key.chmod(0o600)
        print(
            json.dumps(
                {"initialized": True, "requirements": 0, "workers": args.workers}
            )
        )
    elif args.command == "status":
        print(json.dumps(store.snapshot(), ensure_ascii=False))
    else:
        from team_http import serve
        from team_runtime import Team

        team = Team(args.root)
        instance = serve(
            team, "127.0.0.1", args.port, (store.root / "http-key").read_text().strip()
        )
        team.start()

        def stop(*_):
            # shutdown must run outside the serve_forever thread.
            import threading

            threading.Thread(target=instance.shutdown, daemon=True).start()

        signal.signal(signal.SIGTERM, stop)
        signal.signal(signal.SIGINT, stop)
        print(
            f"Agent Team prototype API listening on 127.0.0.1:{args.port}", flush=True
        )
        try:
            instance.serve_forever()
        finally:
            team.close()
            instance.server_close()


if __name__ == "__main__":
    main()
