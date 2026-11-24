#!/usr/bin/env python3
"""Install one private OpenSandbox instance; never change the system Python."""

import argparse
import json
import os
import re
import secrets
import stat
import subprocess
import sys
import uuid
from pathlib import Path

VERSION = "1.1.1"


def prepare(root: Path, port: int) -> Path:
    root = root.expanduser().absolute()
    if any(c.isspace() or c in '\x00"%$\\' for c in str(root)):
        raise ValueError("installation path contains unsupported characters")
    if not 1024 <= port <= 65535:
        raise ValueError("port must be between 1024 and 65535")
    if root.is_symlink():
        raise ValueError("installation directory cannot be a symlink")
    root.mkdir(parents=True, exist_ok=True, mode=0o700)
    if root.stat().st_uid != os.getuid() or root.stat().st_mode & 0o077:
        raise ValueError(
            "installation directory must be owned by this user and mode 0700"
        )
    config = root / "sandbox.toml"
    if config.is_symlink():
        raise ValueError("configuration cannot be a symlink")
    if config.exists():
        if config.stat().st_uid != os.getuid() or config.stat().st_mode & 0o077:
            raise ValueError(
                "existing configuration must be owned by this user and mode 0600"
            )
        # Existing operational settings and credentials belong to the operator.
        # Upgrade only the package; do not silently replace their configuration.
        return root
    body = f"""[server]
host = "127.0.0.1"
port = {port}
eip = "127.0.0.1"
api_key = {json.dumps(secrets.token_urlsafe(48))}
max_sandbox_timeout_seconds = 86400

[runtime]
type = "docker"
execd_image = "opensandbox/execd:v1.1.0"

[docker]
network_mode = "bridge"
publish_host = "127.0.0.1"
host_ip = "127.0.0.1"
port_range_min = 45000
port_range_max = 46000
no_new_privileges = true
pids_limit = 4096

[storage]
allowed_host_paths = []

[store]
type = "sqlite"
path = {json.dumps(str(root / "metadata.db"))}

[log]
level = "INFO"
"""
    with config.open("x", encoding="utf-8") as output:
        os.chmod(config, 0o600)
        output.write(body)
    return root


def install(root: Path) -> None:
    subprocess.run([sys.executable, "-m", "venv", str(root / "venv")], check=True)
    python = str(root / "venv/bin/python")
    # Ubuntu 22.04 seeds pip 22.0, which has no installation report support.
    # Bootstrap only our venv, so every new host uses the same supported entry.
    subprocess.run(
        [
            python,
            "-m",
            "pip",
            "install",
            "--disable-pip-version-check",
            "pip==25.3",
        ],
        check=True,
    )
    subprocess.run(
        [
            python,
            "-m",
            "pip",
            "install",
            "--disable-pip-version-check",
            "--report",
            str(root / f"install-report-{uuid.uuid4().hex}.json"),
            f"opensandbox-server=={VERSION}",
        ],
        check=True,
    )
    subprocess.run([python, "-c", "import opensandbox_server.cli"], check=True)
    with (root / "installed-packages.txt").open("w", encoding="utf-8") as output:
        subprocess.run([python, "-m", "pip", "freeze"], stdout=output, check=True)
    (root / "agent-platform-opensandbox.service").write_text(
        service_unit(root), encoding="utf-8"
    )


def docker_environment(root: Path, endpoint: str | None) -> None:
    path = root / "docker.env"

    def encoded(value):
        if not re.fullmatch(r"unix:///[A-Za-z0-9_./-]+", value):
            raise ValueError("choose an explicit local Docker Unix socket")
        return f"DOCKER_HOST={value}\n"

    requested = encoded(endpoint) if endpoint is not None else None
    if path.exists() or path.is_symlink():
        if path.is_symlink():
            raise ValueError("Docker environment must not be a symlink")
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(descriptor) as source:
            info = os.fstat(source.fileno())
            if (
                not stat.S_ISREG(info.st_mode)
                or info.st_uid != os.getuid()
                or info.st_mode & 0o077
            ):
                raise ValueError("Docker environment must be private and owned")
            previous = source.read(4097)
        if (
            not previous.startswith("DOCKER_HOST=")
            or encoded(previous.removeprefix("DOCKER_HOST=").removesuffix("\n"))
            != previous
        ):
            raise ValueError("invalid Docker environment")
        if requested is not None and requested != previous:
            raise ValueError(
                "Docker backend changes require a separate installation; retain existing environments"
            )
        return
    if requested is None:
        return
    if (root / "installed-packages.txt").exists():
        raise ValueError(
            "Docker backend changes require a separate installation; retain existing environments"
        )
    with path.open("x", encoding="utf-8") as output:
        os.chmod(path, 0o600)
        output.write(requested)


def service_unit(root: Path) -> str:
    docker_environment(root, None)
    docker_line = (
        f"EnvironmentFile={root / 'docker.env'}\n"
        if (root / "docker.env").exists()
        else ""
    )
    return f'''[Unit]
Description=Agent Platform isolated OpenSandbox
After=network-online.target

[Service]
Type=simple
WorkingDirectory={root}
{docker_line}ExecStart="{root}/venv/bin/opensandbox-server" --config "{root}/sandbox.toml"
Restart=on-failure
RestartSec=5
UMask=0077

[Install]
WantedBy=default.target
'''


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--port", type=int, default=8095)
    parser.add_argument(
        "--docker-host",
        help="explicit local Docker socket for a new installation, including rootless Docker",
    )
    parser.add_argument("--prepare-only", action="store_true")
    args = parser.parse_args()
    os.umask(0o077)
    root = prepare(args.root, args.port)
    docker_environment(root, args.docker_host)
    if not args.prepare_only:
        install(root)
    print(f"Prepared {root}; existing configuration and API key are preserved.")
    if not args.prepare_only:
        print(
            "Link the generated unit with systemctl --user link, then enable --now it."
        )


if __name__ == "__main__":
    main()
