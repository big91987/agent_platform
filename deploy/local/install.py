#!/usr/bin/env python3
"""Install a distinct loopback Agent Platform instance on macOS."""

import argparse
import hashlib
import json
import os
import plistlib
import socket
import subprocess
import sys
from pathlib import Path


def write(path, data, mode=0o600):
    if path.is_symlink():
        raise ValueError("refusing a symlink destination")
    temp = path.with_name(path.name + ".next")
    fd = os.open(temp, os.O_CREAT | os.O_EXCL | os.O_WRONLY, mode)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
        os.replace(temp, path)
    finally:
        temp.unlink(missing_ok=True)


def specification(root, binary, port, claude, claude_home, concurrency):
    return {
        "root": str(root),
        "port": port,
        "path": "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
        "argv": [
            str(binary),
            "-listen",
            f"127.0.0.1:{port}",
            "-base-url",
            f"http://127.0.0.1:{port}",
            "-data",
            str(root / "data"),
            "-concurrency",
            str(concurrency),
            "-claude",
            str(claude),
            "-claude-config-home",
            str(claude_home),
        ],
    }


def main():
    os.umask(0o077)
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--root", type=Path, required=True)
    p.add_argument("--binary", type=Path, required=True)
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--claude", type=Path, required=True)
    p.add_argument("--claude-config-home", type=Path, default=Path.home() / ".claude")
    p.add_argument("--concurrency", type=int, default=3)
    p.add_argument("--activate", action="store_true")
    args = p.parse_args()
    if (
        sys.platform != "darwin"
        or not 1024 <= args.port <= 65535
        or not 1 <= args.concurrency <= 32
    ):
        p.error("requires macOS, port1024..65535 and concurrency1..32")
    root = args.root.expanduser().resolve()
    root.mkdir(parents=True, exist_ok=True, mode=0o700)
    if root.stat().st_uid != os.getuid() or root.stat().st_mode & 0o077:
        raise ValueError("instance root must be private and owned")
    if (root / "instance.json").exists():
        raise ValueError(
            "instance already installed; preserve its data and explicitly manage its existing service"
        )
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", args.port))
    source = args.binary.resolve(strict=True)
    version = json.loads(
        subprocess.check_output([str(source), "--version"], text=True, timeout=10)
    )
    binary = root / "agent-platform"
    write(binary, source.read_bytes(), 0o700)
    write(
        root / "service.py", Path(__file__).with_name("service.py").read_bytes(), 0o700
    )
    config = specification(
        root,
        binary,
        args.port,
        args.claude.resolve(strict=True),
        args.claude_config_home.resolve(strict=True),
        args.concurrency,
    )
    config["build"] = {
        **version,
        "sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
    }
    write(root / "instance.json", (json.dumps(config, indent=2) + "\n").encode())
    if not (root / "credentials.json").exists():
        write(root / "credentials.json", b"{}\n")
    label = f"com.agent-platform.local.{args.port}"
    plist = Path.home() / "Library/LaunchAgents" / (label + ".plist")
    if plist.exists() or plist.is_symlink():
        raise ValueError("existing LaunchAgent must not be replaced")
    plist.parent.mkdir(parents=True, exist_ok=True)
    document = {
        "Label": label,
        "ProgramArguments": [sys.executable, str(root / "service.py"), str(root)],
        "WorkingDirectory": str(root),
        "RunAtLoad": True,
        "KeepAlive": True,
        "StandardOutPath": str(root / "server.log"),
        "StandardErrorPath": str(root / "server-error.log"),
    }
    write(plist, plistlib.dumps(document))
    if args.activate:
        subprocess.run(
            ["launchctl", "bootstrap", f"gui/{os.getuid()}", str(plist)], check=True
        )
    print(
        json.dumps(
            {
                "url": f"http://127.0.0.1:{args.port}",
                "revision": version.get("revision"),
                "activated": args.activate,
            }
        )
    )


if __name__ == "__main__":
    main()
