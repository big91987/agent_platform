#!/usr/bin/env python3
"""Install the trusted Model Relay preview controller without deploying code."""

import argparse
import fcntl
import hashlib
import json
import os
import plistlib
import re
import shutil
import subprocess
import sys
from pathlib import Path


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--port", type=int, default=5545)
    parser.add_argument("--git-proxy")
    args = parser.parse_args()
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", args.repository):
        parser.error("repository must be owner/repository")
    if not 1024 <= args.port <= 65535:
        parser.error("port must be 1024..65535")
    root = args.root.expanduser().resolve()
    root.mkdir(parents=True, exist_ok=True, mode=0o700)
    if root.stat().st_mode & 0o077:
        raise ValueError("deployment root must be private to its owner")
    label = (
        "com.agent-platform.model-relay."
        + args.repository.replace("/", ".")
        + "."
        + str(args.port)
    )
    expected = {"repository": args.repository, "port": args.port, "label": label}
    with (root / "deploy.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        settings = root / "preview.json"
        if settings.exists() and json.loads(settings.read_text()) != expected:
            raise ValueError("deployment root already belongs to different settings")
        for name in ("logs", "plans", "releases", "backups", "failed-data", "private"):
            (root / name).mkdir(exist_ok=True)
        repository = root / "repository"
        url = "https://github.com/" + args.repository + ".git"
        if not repository.exists():
            subprocess.run(
                [
                    "git",
                    *(["-c", "http.proxy=" + args.git_proxy] if args.git_proxy else []),
                    "clone",
                    "--no-checkout",
                    url,
                    str(repository),
                ],
                check=True,
            )
        actual = subprocess.check_output(
            ["git", "-C", str(repository), "remote", "get-url", "origin"], text=True
        ).strip()
        if actual != url:
            raise ValueError(
                "existing repository remote differs from requested repository"
            )
        if args.git_proxy:
            subprocess.run(
                ["git", "-C", str(repository), "config", "http.proxy", args.git_proxy],
                check=True,
            )
        plist = Path.home() / "Library/LaunchAgents" / (label + ".plist")
        plist.parent.mkdir(parents=True, exist_ok=True)
        content = {
            "Label": label,
            "ProgramArguments": [
                str(root / "current/source/bin/model-relay"),
                "serve",
                "--data-dir",
                str(root / "data"),
                "--key-file",
                str(root / "master.key"),
                "--listen",
                "127.0.0.1:" + str(args.port),
            ],
            "RunAtLoad": True,
            "KeepAlive": True,
            "WorkingDirectory": str(root),
            "StandardOutPath": str(root / "logs/server.log"),
            "StandardErrorPath": str(root / "logs/server-error.log"),
        }
        if plist.exists() and plistlib.loads(plist.read_bytes()) != content:
            raise ValueError("existing LaunchAgent differs; inspect before changing it")
        source = Path(__file__).with_name("controller.py")
        activation = root / "activation.json"
        controller = root / "controller.py"
        if (
            activation.exists()
            and json.loads(activation.read_text()).get("stage")
            not in ("committed", "rolled_back")
            and (
                not controller.exists()
                or controller.read_bytes() != source.read_bytes()
            )
        ):
            raise ValueError(
                "activation is unfinished; recover with the installed controller before upgrading"
            )
        temporary = root / ".controller-next.py"
        shutil.copyfile(source, temporary)
        temporary.chmod(0o700)
        os.replace(temporary, controller)
        settings.write_text(json.dumps(expected, sort_keys=True) + "\n")
        plist.write_bytes(plistlib.dumps(content))
        manifest = root / ".controller-install-next.json"
        manifest.write_text(
            json.dumps(
                {
                    "format": 1,
                    "controller_sha256": hashlib.sha256(
                        controller.read_bytes()
                    ).hexdigest(),
                },
                sort_keys=True,
            )
            + "\n"
        )
        os.replace(manifest, root / "controller-install.json")
    print("Installed trusted controller at", controller)
    print("No product release was deployed or started.")
    print("Set repository Actions variable MODEL_RELAY_PREVIEW_ROOT to", root)
    print(
        "Set repository Actions variable MODEL_RELAY_PREVIEW_URL to http://127.0.0.1:"
        + str(args.port)
        + "/admin/"
    )


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("Installation failed:", error, file=sys.stderr)
        sys.exit(1)
