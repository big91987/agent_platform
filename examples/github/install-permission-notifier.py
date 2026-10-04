"""Install GitHub permission notifications as a macOS user service."""

import argparse
import hashlib
import json
import os
import plistlib
import subprocess
import sys
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True, type=Path)
    args = parser.parse_args()
    if sys.platform != "darwin":
        parser.error(
            "On other hosts run permission_notifications.py with your service manager"
        )
    config = args.config.resolve(strict=True)
    settings = json.loads(config.read_text())["pipeline"]
    registry = Path(settings["registry"])
    if not registry.is_dir():
        parser.error("Run pipeline setup first")
    root = Path(__file__).resolve().parents[2]
    label = (
        "agent-platform.permissions."
        + hashlib.sha256(str(config).encode()).hexdigest()[:12]
    )
    target = Path.home() / "Library/LaunchAgents" / (label + ".plist")
    target.parent.mkdir(parents=True, exist_ok=True)
    environment = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "PYTHONPATH": str(root / "sdk/python"),
    }
    for key in (
        "HTTPS_PROXY",
        "HTTP_PROXY",
        "ALL_PROXY",
        "NO_PROXY",
        "https_proxy",
        "http_proxy",
        "all_proxy",
        "no_proxy",
    ):
        if key in os.environ:
            environment[key] = os.environ[key]
    value = {
        "Label": label,
        "ProgramArguments": [
            sys.executable,
            str(root / "examples/github/permission_notifications.py"),
            "--config",
            str(config),
        ],
        "WorkingDirectory": str(root),
        "EnvironmentVariables": environment,
        "RunAtLoad": True,
        "KeepAlive": True,
        "ThrottleInterval": 10,
        "StandardOutPath": str(registry / "permission-notifications.log"),
        "StandardErrorPath": str(registry / "permission-notifications.log"),
    }
    target.write_bytes(plistlib.dumps(value))
    target.chmod(0o600)
    domain = f"gui/{os.getuid()}"
    subprocess.run(
        ["launchctl", "bootout", domain + "/" + label], capture_output=True, check=False
    )
    subprocess.run(["launchctl", "bootstrap", domain, str(target)], check=True)
    print(
        f"Installed {label}; notifications cover this configuration's registered Issue conversations only."
    )


if __name__ == "__main__":
    main()
