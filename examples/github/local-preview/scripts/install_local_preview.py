#!/usr/bin/env python3
"""Install the trusted deployment controller and launchd service once on this Mac."""

import argparse
import json
import os
import plistlib
import re
import shutil
import subprocess
import sys
from pathlib import Path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument(
        "--git-proxy", help="Optional persistent HTTP proxy for repository fetches"
    )
    parser.add_argument("--repository", required=True, help="GitHub owner/repository")
    parser.add_argument("--port", type=int, default=5533)
    args = parser.parse_args()
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", args.repository):
        parser.error("repository must be owner/repository")
    if not 1024 <= args.port <= 65535:
        parser.error("port must be 1024..65535")
    root = args.root.expanduser().resolve()
    settings = root / "preview.json"
    expected = {"repository": args.repository, "port": args.port}
    if settings.exists() and json.loads(settings.read_text()) != expected:
        raise ValueError(
            "Existing deployment root has different repository/port settings"
        )
    for name in ("controller", "logs", "plans", "releases", "data"):
        (root / name).mkdir(parents=True, exist_ok=True)
    repository = root / "repository"
    if not repository.exists():
        subprocess.run(
            [
                "git",
                *(["-c", "http.proxy=" + args.git_proxy] if args.git_proxy else []),
                "clone",
                "--no-checkout",
                f"https://github.com/{args.repository}.git",
                str(repository),
            ],
            check=True,
        )
    if (
        subprocess.check_output(
            ["git", "-C", str(repository), "remote", "get-url", "origin"], text=True
        ).strip()
        != f"https://github.com/{args.repository}.git"
    ):
        raise ValueError("Existing deployment root belongs to another repository")
    (root / "preview.json").write_text(
        json.dumps({"repository": args.repository, "port": args.port})
    )
    if args.git_proxy:
        subprocess.run(
            ["git", "-C", str(repository), "config", "http.proxy", args.git_proxy],
            check=True,
        )
    controller = root / "controller/local_deploy.py"
    shutil.copyfile(Path(__file__).with_name("local_deploy.py"), controller)
    label = (
        "com.agent-platform.preview."
        + args.repository.replace("/", ".")
        + "."
        + str(args.port)
    )
    plist = Path.home() / "Library/LaunchAgents" / (label + ".plist")
    plist.parent.mkdir(parents=True, exist_ok=True)
    config = {
        "Label": label,
        "ProgramArguments": [
            sys.executable,
            str(controller),
            "serve",
            "--root",
            str(root),
        ],
        "RunAtLoad": True,
        "KeepAlive": True,
        "StandardOutPath": str(root / "logs/server.log"),
        "StandardErrorPath": str(root / "logs/server-error.log"),
        "WorkingDirectory": str(root),
    }
    plist.write_bytes(plistlib.dumps(config))
    domain = f"gui/{os.getuid()}"
    subprocess.run(["launchctl", "bootout", domain + "/" + label], capture_output=True)
    subprocess.run(["launchctl", "bootstrap", domain, str(plist)], check=True)
    print(
        f"Service installed on http://127.0.0.1:{args.port}; no release is changed by installation."
    )
    print("Set GitHub variable LOCAL_DEPLOY_ROOT to " + str(root))


if __name__ == "__main__":
    main()
