#!/usr/bin/env python3
"""Install a verified platform binary in one isolated Linux user service."""

import argparse
import fcntl
import hashlib
import ipaddress
import json
import os
import re
import secrets
import socket
import stat
import subprocess
import time
import urllib.error
import urllib.request
import uuid
from contextlib import contextmanager
from pathlib import Path
from urllib.parse import urlparse


def private_file(path: Path, limit: int = 65536) -> bytes:
    before = path.lstat()
    if (
        not stat.S_ISREG(before.st_mode)
        or before.st_uid != os.getuid()
        or before.st_mode & 0o077
    ):
        raise ValueError("configuration must be a private, owned regular file")
    with path.open("rb") as stream:
        actual = os.fstat(stream.fileno())
        if (before.st_dev, before.st_ino) != (actual.st_dev, actual.st_ino):
            raise ValueError("private configuration changed while opening")
        value = stream.read(limit + 1)
    if len(value) > limit:
        raise ValueError("private configuration exceeds its size limit")
    return value


def write_private(path: Path, value: bytes) -> None:
    with path.open("xb") as stream:
        path.chmod(0o600)
        stream.write(value)
        stream.flush()
        os.fsync(stream.fileno())


@contextmanager
def installation_lock(root):
    descriptor = os.open(
        root / ".install.lock", os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600
    )
    try:
        info = os.fstat(descriptor)
        if (
            not stat.S_ISREG(info.st_mode)
            or info.st_uid != os.getuid()
            or info.st_mode & 0o077
            or info.st_nlink != 1
        ):
            raise ValueError("installation lock must be private and owned")
        try:
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise ValueError("another installation is in progress") from error
        yield
    finally:
        os.close(descriptor)


def service_unit(root, settings, *, memory=True, runtime=True):
    port = int(settings["listen"].rpartition(":")[2])
    command = [
        str(root / "agent-platform"),
        "--data",
        str(root / "data"),
        "--listen",
        settings["listen"],
        "--base-url",
        settings["base_url"],
        "--sandbox-config",
        settings["sandbox_config"],
        "--sandbox-image",
        settings["image"],
        "--sandbox-ttl",
        str(settings["ttl"]),
        "--auth-home",
        settings["auth_home"],
    ]
    if memory:
        command.extend(["--sandbox-memory-mib", str(settings.get("memory_mib", 2048))])
    optional = f"EnvironmentFile=-{root / 'runtime.env'}\n" if runtime else ""
    return f"""[Unit]
Description=Isolated Agent Platform {port}
After=network-online.target

[Service]
Type=simple
WorkingDirectory={root}
EnvironmentFile={root / "credentials.env"}
{optional}ExecStart={" ".join(json.dumps(value) for value in command)}
Restart=on-failure
RestartSec=5
UMask=0077

[Install]
WantedBy=default.target
""".encode()


def runtime_environment(path: Path) -> None:
    if not path.exists() and not path.is_symlink():
        return
    raw = private_file(path)
    try:
        lines = raw.decode("utf-8").splitlines()
    except UnicodeDecodeError as error:
        raise ValueError("runtime environment must be UTF-8") from error
    seen = set()
    for line in lines:
        if not line.strip() or line.startswith("#"):
            continue
        name, sep, value = line.partition("=")
        if (
            not sep
            or not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]{0,127}", name)
            or name in seen
            or name
            in {
                "HOME",
                "PATH",
                "LD_PRELOAD",
                "LD_LIBRARY_PATH",
                "NODE_OPTIONS",
                "PYTHONPATH",
                "PYTHONHOME",
                "BASH_ENV",
                "ENV",
            }
            or name.startswith(("AGENT_PLATFORM_", "CODEX_", "WORKFLOW_"))
            or not value
            or "\x00" in value
            or any(c in value for c in "\r\n")
        ):
            raise ValueError(
                "runtime environment contains an invalid or reserved entry"
            )
        seen.add(name)


def unit_path(path: Path) -> str:
    value = str(path)
    if any(c.isspace() or c in '\x00"%$\\' for c in value):
        raise ValueError("service paths contain unsupported characters")
    return value


def prepare(
    root,
    binary,
    revision,
    listen,
    base_url,
    sandbox_config,
    image,
    ttl,
    auth_home,
    *,
    memory_mib=None,
    update_environment=False,
    activate_service=False,
):
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("choose an exact source revision")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", image):
        raise ValueError("choose an immutable sandbox image")
    host, _, port_text = listen.rpartition(":")
    host = host.strip("[]")
    ip = ipaddress.ip_address(host)
    port = int(port_text)
    if (
        not (ip.is_loopback or ip.is_private)
        or ip.is_unspecified
        or not 1024 <= port <= 65535
    ):
        raise ValueError("choose an explicit private or loopback listener")
    url = urlparse(base_url)
    if (
        url.scheme != "http"
        or url.hostname != host
        or url.port != port
        or url.username
        or url.password
        or url.query
        or url.fragment
        or url.path not in ("", "/")
    ):
        raise ValueError("base URL must match this private listener")
    root = Path(root).expanduser().absolute()
    if root.is_symlink():
        raise ValueError("instance directory cannot be a symlink")
    root = root.resolve()
    configuration = root / "instance.json"
    original_config = None
    old_settings = None
    if configuration.exists() or configuration.is_symlink():
        original_config = private_file(configuration)
        old_settings = json.loads(original_config)
        if not isinstance(old_settings, dict):
            raise ValueError("invalid instance configuration")
        old_memory = old_settings.get("memory_mib", 2048)
        old_ttl = old_settings.get("ttl")
        if (
            type(old_memory) is not int
            or not 256 <= old_memory <= 65536
            or type(old_ttl) is not int
            or not 60 <= old_ttl <= 86400
        ):
            raise ValueError("invalid existing environment configuration")
    if ttl is None:
        ttl = old_settings.get("ttl", 3600) if old_settings else 3600
    if memory_mib is None:
        memory_mib = old_settings.get("memory_mib", 2048) if old_settings else 2048
    if type(ttl) is not int or not 60 <= ttl <= 86400:
        raise ValueError("choose a bounded TTL between 60 and 86400 seconds")
    if type(memory_mib) is not int or not 256 <= memory_mib <= 65536:
        raise ValueError("memory must be between 256 and 65536 MiB")
    sandbox_config = Path(sandbox_config).expanduser().absolute()
    private_file(sandbox_config)
    sandbox_config = sandbox_config.resolve()
    auth_home = Path(auth_home).expanduser().resolve()
    for path in (root, sandbox_config, auth_home):
        unit_path(path)
    binary = Path(binary).resolve(strict=True)
    if not binary.is_file():
        raise ValueError("binary must be a regular file")
    version = json.loads(subprocess.check_output([str(binary), "--version"], timeout=5))
    if version.get("revision") != revision:
        raise ValueError("binary revision differs from the requested source revision")
    with binary.open("rb") as source:
        raw = source.read(256 * 1024 * 1024 + 1)
    if len(raw) > 256 * 1024 * 1024:
        raise ValueError("binary exceeds its size limit")
    digest = hashlib.sha256(raw).hexdigest()
    settings = dict(
        listen=listen,
        base_url=base_url.rstrip("/"),
        sandbox_config=str(sandbox_config),
        image=image,
        ttl=ttl,
        auth_home=str(auth_home),
        memory_mib=memory_mib,
    )
    change_config = original_config is None
    if old_settings is not None:
        normalized = dict(old_settings, memory_mib=old_settings.get("memory_mib", 2048))
        if normalized != settings:
            changed = {
                key
                for key in normalized.keys() | settings.keys()
                if normalized.get(key) != settings.get(key)
            }
            if not update_environment or not changed.issubset({"ttl", "memory_mib"}):
                raise ValueError(
                    "existing instance configuration differs; inspect before upgrading"
                )
            change_config = True
    root.mkdir(mode=0o700, parents=True, exist_ok=True)
    if root.stat().st_uid != os.getuid() or root.stat().st_mode & 0o077:
        raise ValueError("instance directory must be private and owned")
    with installation_lock(root):
        runtime = root / "runtime.env"
        runtime_environment(runtime)
        credentials = root / "credentials.env"
        if credentials.exists() or credentials.is_symlink():
            private_file(credentials)
        else:
            write_private(
                credentials,
                (
                    "AGENT_PLATFORM_PASSWORD=" + secrets.token_urlsafe(32) + "\n"
                ).encode(),
            )
        data = root / "data"
        if data.is_symlink():
            raise ValueError("instance data cannot be a symlink")
        data.mkdir(mode=0o700, exist_ok=True)
        if data.stat().st_uid != os.getuid() or data.stat().st_mode & 0o077:
            raise ValueError("instance data must be private and owned")
        releases = root / "releases"
        if releases.is_symlink():
            raise ValueError("release directory cannot be a symlink")
        releases.mkdir(mode=0o700, exist_ok=True)
        release = releases / revision
        if release.is_symlink():
            raise ValueError("revision directory cannot be a symlink")
        release.mkdir(mode=0o700, exist_ok=True)
        installed = release / "agent-platform"
        manifest = dict(
            revision=revision,
            version=version["version"],
            binary_sha256=digest,
            bytes=len(raw),
        )
        if installed.exists() or installed.is_symlink():
            if (
                hashlib.sha256(private_file(installed, 256 * 1024 * 1024)).hexdigest()
                != digest
            ):
                raise ValueError(
                    "same revision has different binary bytes; original retained"
                )
            if json.loads(private_file(release / "manifest.json")) != manifest:
                raise ValueError("existing binary manifest differs")
        else:
            write_private(installed, raw)
            installed.chmod(0o700)
            write_private(
                release / "manifest.json", json.dumps(manifest, indent=2).encode()
            )
        service_name = f"agent-platform-{port}.service"
        unit = service_unit(root, settings)
        service = root / service_name
        replacement_needed = True
        if service.exists() or service.is_symlink():
            previous = private_file(service)
            replacement_needed = previous != unit
            if replacement_needed:
                prior = old_settings or settings
                accepted = {
                    service_unit(root, prior, runtime=enabled)
                    for enabled in (True, False)
                }
                if "memory_mib" not in prior:
                    accepted.update(
                        service_unit(root, prior, memory=False, runtime=enabled)
                        for enabled in (True, False)
                    )
                if previous not in accepted:
                    raise ValueError("existing service configuration differs")
        if original_config is not None:
            if private_file(configuration) != original_config:
                raise ValueError("instance configuration changed while preparing")
        elif configuration.exists() or configuration.is_symlink():
            raise ValueError("instance configuration appeared while preparing")
        active = root / "agent-platform"
        if active.exists() and not active.is_symlink():
            raise ValueError("active binary is not a managed symlink")
        if active.is_symlink() and active.resolve().parent.parent != releases:
            raise ValueError("active binary points outside this instance")
        if replacement_needed:
            replacement = root / (".unit-" + uuid.uuid4().hex)
            write_private(replacement, unit)
            os.replace(replacement, service)
        if change_config:
            replacement = root / (".config-" + uuid.uuid4().hex)
            write_private(replacement, json.dumps(settings, indent=2).encode())
            os.replace(replacement, configuration)
        link = root / (".activate-" + uuid.uuid4().hex)
        link.symlink_to(Path("releases") / revision / "agent-platform")
        os.replace(link, active)
        result = dict(
            manifest,
            root=str(root),
            service=service_name,
            url=settings["base_url"],
            listen=listen,
        )
        if activate_service:
            result["health"] = activate(result)
        return result


def activate(result):
    root = Path(result["root"])
    service = result["service"]
    loaded = subprocess.check_output(
        ["systemctl", "--user", "show", service, "--property=FragmentPath", "--value"],
        text=True,
        timeout=10,
    ).strip()
    if loaded and Path(loaded).resolve() != (root / service).resolve():
        raise ValueError("another installation already owns this service name")
    state = subprocess.run(
        ["systemctl", "--user", "is-active", "--quiet", service], timeout=10
    )
    if state.returncode != 0:
        host, _, port = result["listen"].rpartition(":")
        family = socket.AF_INET6 if ":" in host.strip("[]") else socket.AF_INET
        with socket.socket(family) as check:
            check.bind((host.strip("[]"), int(port)))
    subprocess.run(
        ["systemd-analyze", "--user", "verify", str(root / service)],
        check=True,
        timeout=15,
    )
    subprocess.run(
        ["systemctl", "--user", "link", str(root / service)], check=True, timeout=15
    )
    subprocess.run(["systemctl", "--user", "daemon-reload"], check=True, timeout=15)
    subprocess.run(["systemctl", "--user", "enable", service], check=True, timeout=15)
    subprocess.run(["systemctl", "--user", "restart", service], check=True, timeout=15)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    last = "health unavailable"
    for _ in range(60):
        try:
            with opener.open(result["url"] + "/api/health", timeout=2) as response:
                health = json.loads(response.read(65537))
            if health.get("ok") and health.get("revision") == result["revision"]:
                return health
            last = "health revision differs from the installed binary"
        except (OSError, urllib.error.URLError, json.JSONDecodeError) as error:
            last = type(error).__name__
        time.sleep(0.25)
    raise RuntimeError(
        "activation was not verified: "
        + last
        + "; inspect the original service log, do not restart blindly"
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--revision", required=True)
    parser.add_argument("--listen", default="127.0.0.1:8795")
    parser.add_argument("--base-url", default="http://127.0.0.1:8795")
    parser.add_argument("--sandbox-config", type=Path, required=True)
    parser.add_argument("--sandbox-image", required=True)
    parser.add_argument("--sandbox-ttl", type=int)
    parser.add_argument("--sandbox-memory-mib", type=int)
    parser.add_argument(
        "--update-environment",
        action="store_true",
        help="explicitly update lifetime and memory defaults for new Runs",
    )
    parser.add_argument("--auth-home", type=Path, default=Path.home() / ".codex")
    parser.add_argument("--activate", action="store_true")
    args = parser.parse_args()
    os.umask(0o077)
    result = prepare(
        args.root,
        args.binary,
        args.revision,
        args.listen,
        args.base_url,
        args.sandbox_config,
        args.sandbox_image,
        args.sandbox_ttl,
        args.auth_home,
        memory_mib=args.sandbox_memory_mib,
        update_environment=args.update_environment,
        activate_service=args.activate,
    )
    print(json.dumps(result))


if __name__ == "__main__":
    main()
