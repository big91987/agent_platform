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

from service import read_private


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


def digest(data):
    return hashlib.sha256(data).hexdigest()


def boot(plist):
    subprocess.run(
        ["launchctl", "bootstrap", f"gui/{os.getuid()}", str(plist)],
        check=True,
        timeout=30,
    )


def upgrade(root, source, version, port, activate, confirm_idle, plist):
    config_path = root / "instance.json"
    config = read_private(config_path)
    binary = root / "agent-platform"
    if (
        config.get("root") != str(root)
        or config.get("port") != port
        or config.get("argv", [None])[0] != str(binary)
        or binary.is_symlink()
        or plist.is_symlink()
    ):
        raise ValueError(
            "Existing service does not identify the requested private instance"
        )
    old_binary, old_config = binary.read_bytes(), config_path.read_bytes()
    old_digest = digest(old_binary)
    if config["build"]["sha256"] != old_digest:
        raise ValueError(
            "Installed binary differs from its recorded digest; inspect drift before upgrading"
        )
    new_binary = source.read_bytes()
    new_digest = digest(new_binary)
    if new_digest == old_digest:
        return False
    launch = plistlib.loads(plist.read_bytes())
    label = f"com.agent-platform.local.{port}"
    program = launch.get("ProgramArguments", [])
    if (
        launch.get("Label") != label
        or launch.get("WorkingDirectory") != str(root)
        or program[1:] != [str(root / "service.py"), str(root)]
    ):
        raise ValueError("LaunchAgent belongs to another installation or has drifted")
    target = f"gui/{os.getuid()}/{label}"
    loaded = (
        subprocess.run(
            ["launchctl", "print", target], capture_output=True, timeout=15
        ).returncode
        == 0
    )
    if loaded and (not activate or not confirm_idle):
        raise ValueError(
            "Confirm that the instance has no active work, then use --activate --confirm-idle"
        )
    backup = root / "previous-builds" / old_digest
    if backup.is_symlink() or backup.parent.is_symlink():
        raise ValueError("Previous build storage must be inside the private instance")
    backup.mkdir(parents=True, exist_ok=True, mode=0o700)
    for path, data, mode in (
        (backup / binary.name, old_binary, 0o700),
        (backup / config_path.name, old_config, 0o600),
    ):
        if path.is_symlink() or (path.exists() and path.read_bytes() != data):
            raise ValueError("Previous build storage has drifted")
        if not path.exists():
            write(path, data, mode)
    config["build"] = {**version, "sha256": new_digest}
    if loaded:
        subprocess.run(["launchctl", "bootout", target], check=True, timeout=30)
    try:
        write(binary, new_binary, 0o700)
        write(config_path, (json.dumps(config, indent=2) + "\n").encode())
        if activate:
            boot(plist)
    except (OSError, ValueError, subprocess.SubprocessError):
        write(binary, old_binary, 0o700)
        write(config_path, old_config)
        if loaded:
            boot(plist)
        raise
    return True


def main():
    os.umask(0o077)
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--root", type=Path, required=True)
    p.add_argument("--binary", type=Path, required=True)
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--claude", type=Path)
    p.add_argument("--claude-config-home", type=Path, default=Path.home() / ".claude")
    p.add_argument("--concurrency", type=int, default=3)
    p.add_argument("--activate", action="store_true")
    p.add_argument("--upgrade", action="store_true")
    p.add_argument("--confirm-idle", action="store_true")

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
    source = args.binary.resolve(strict=True)
    version = json.loads(
        subprocess.check_output([str(source), "--version"], text=True, timeout=10)
    )
    label = f"com.agent-platform.local.{args.port}"
    plist = Path.home() / "Library/LaunchAgents" / (label + ".plist")
    if args.upgrade:
        changed = upgrade(
            root, source, version, args.port, args.activate, args.confirm_idle, plist
        )
        print(
            json.dumps(
                {
                    "url": f"http://127.0.0.1:{args.port}",
                    "revision": version.get("revision"),
                    "upgraded": changed,
                }
            )
        )
        return
    if not args.claude:
        p.error("--claude is required for a new instance")
    if (root / "instance.json").exists():
        raise ValueError("instance already installed; use --upgrade to preserve it")
    if plist.exists() or plist.is_symlink():
        raise ValueError("existing LaunchAgent must not be replaced")
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", args.port))
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
        "sha256": digest(binary.read_bytes()),
    }
    write(root / "instance.json", (json.dumps(config, indent=2) + "\n").encode())
    if not (root / "credentials.json").exists():
        write(root / "credentials.json", b"{}\n")
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
        boot(plist)
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
