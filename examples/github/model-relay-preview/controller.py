#!/usr/bin/env python3
"""Trusted local release controller for the Model Relay CLI contract."""

import argparse
import fcntl
import hashlib
import json
import os
import plistlib
import shutil
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
from contextlib import contextmanager
from pathlib import Path


def run(*args, cwd=None, stdout=None, check=True):
    return subprocess.run(
        args,
        cwd=cwd,
        stdout=stdout,
        stderr=subprocess.STDOUT if stdout else None,
        check=check,
        text=stdout is None,
    )


def git(root, *args):
    return subprocess.check_output(
        ["git", "-C", str(root / "repository"), *args], text=True
    ).strip()


def digest(path):
    result = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            result.update(chunk)
    return result.hexdigest()


def read_json(path):
    return json.loads(path.read_text())


def save_json(path, value):
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_text(json.dumps(value, sort_keys=True, indent=2) + "\n")
    os.replace(temporary, path)


@contextmanager
def locked(root):
    with (root / "deploy.lock").open("a") as stream:
        fcntl.flock(stream, fcntl.LOCK_EX)
        yield


def settings(root):
    if root.stat().st_mode & 0o077:
        raise ValueError("deployment root is not private to its owner")
    value = read_json(root / "preview.json")
    if set(value) != {"repository", "port", "label"}:
        raise ValueError("unexpected preview configuration")
    return value


def service_plist(root, config):
    return Path.home() / "Library/LaunchAgents" / (config["label"] + ".plist")


def service_plist_valid(root, config):
    path = service_plist(root, config)
    if not path.is_file():
        return False
    value = plistlib.loads(path.read_bytes())
    return value.get("Label") == config["label"] and value.get("ProgramArguments") == [
        str(root / "current/source/bin/model-relay"),
        "serve",
        "--data-dir",
        str(root / "data"),
        "--key-file",
        str(root / "master.key"),
        "--listen",
        "127.0.0.1:" + str(config["port"]),
    ]


def launchctl(root, config, action):
    domain = "gui/" + str(os.getuid())
    if action == "stop":
        run("launchctl", "bootout", domain + "/" + config["label"], check=False)
    else:
        run("launchctl", "bootstrap", domain, str(service_plist(root, config)))


def healthy(port, version=None):
    try:
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open(f"http://127.0.0.1:{port}/healthz", timeout=2) as response:
            result = json.load(response)
            return (
                response.status == 200
                and result.get("status") == "ok"
                and (version is None or result.get("version") == version)
            )
    except (OSError, ValueError, urllib.error.URLError):
        return False


def port_open(port):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as connection:
        connection.settimeout(2)
        return connection.connect_ex(("127.0.0.1", port)) == 0


def await_health(port, expected, version=None, timeout=20):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if healthy(port, version) if expected else not port_open(port):
            return
        time.sleep(0.2)
    raise RuntimeError("service health did not reach expected state")


def release_source(root, sha):
    return root / "releases" / sha / "source"


def release_binary(root, sha):
    return release_source(root, sha) / "bin/model-relay"


def current_sha(root):
    record = root / "deployed.json"
    return read_json(record)["sha"] if record.exists() else None


def latest_main(root):
    run("git", "-C", str(root / "repository"), "fetch", "origin", "main")
    return git(root, "rev-parse", "FETCH_HEAD")


def valid_sha(sha):
    return (
        isinstance(sha, str)
        and len(sha) == 40
        and all(character in "0123456789abcdef" for character in sha)
    )


def stamp_binary(source, sha, log):
    with log.open("a") as stream:
        run(
            "go",
            "build",
            "-trimpath",
            "-ldflags",
            "-X main.buildVersion=" + sha,
            "-o",
            "bin/model-relay",
            "./cmd/model-relay",
            cwd=source,
            stdout=stream,
        )


def prepare(root, sha, plan_path, output):
    if not valid_sha(sha):
        raise ValueError("target must be a full lowercase commit SHA")
    config = settings(root)
    with locked(root):
        latest = latest_main(root)
        if latest != sha:
            raise ValueError("requested commit is no longer the current main head")
        old = current_sha(root)
        if old == sha:
            if output:
                output.write_text("deploy=false\n")
            print("Already deployed:", sha)
            return
        source = release_source(root, sha)
        ready = source.parent / "ready.json"
        if ready.exists():
            candidate = read_json(ready)
            if candidate != {
                "sha": sha,
                "binary_sha256": digest(release_binary(root, sha)),
            }:
                raise ValueError("prepared release has changed")
        else:
            if source.exists():
                run(
                    "git",
                    "-C",
                    str(root / "repository"),
                    "worktree",
                    "remove",
                    "--force",
                    str(source),
                )
                shutil.rmtree(source.parent)
            source.parent.mkdir(parents=True, exist_ok=True)
            run(
                "git",
                "-C",
                str(root / "repository"),
                "worktree",
                "add",
                "--detach",
                str(source),
                sha,
            )
            log = root / "logs" / ("verify-" + sha + ".log")
            with log.open("w") as stream:
                result = run("make", "verify", cwd=source, stdout=stream, check=False)
            if result.returncode:
                raise RuntimeError(
                    "project verification failed; see private log " + str(log)
                )
            stamp_binary(source, sha, log)
            binary = release_binary(root, sha)
            if not binary.is_file():
                raise RuntimeError("make verify did not build the service")
            candidate = {"sha": sha, "binary_sha256": digest(binary)}
            save_json(ready, candidate)
        plan = {
            **candidate,
            "previous_sha": old,
            "repository": config["repository"],
            "port": config["port"],
        }
        save_json(plan_path, plan)
        if output:
            output.write_text("deploy=true\n")
        print("Verified main:", sha)
        print("Current deployment:", old or "none")
        print("Review target: http://127.0.0.1:" + str(config["port"]) + "/admin/")


def switch_current(root, sha):
    pointer = root / "current"
    temporary = root / ".current-next"
    temporary.unlink(missing_ok=True)
    if sha is None:
        pointer.unlink(missing_ok=True)
    else:
        temporary.symlink_to(root / "releases" / sha)
        os.replace(temporary, pointer)


def activate(root, plan_path):
    config = settings(root)
    with locked(root):
        plan = read_json(plan_path)
        sha = plan["sha"]
        if not valid_sha(sha):
            raise ValueError("deployment plan has invalid commit SHA")
        old = current_sha(root)
        if old == sha:
            print("Already deployed:", sha)
            return
        if plan != {
            "sha": sha,
            "binary_sha256": digest(release_binary(root, sha)),
            "previous_sha": old,
            "repository": config["repository"],
            "port": config["port"],
        }:
            raise ValueError("deployment plan or release changed; prepare again")
        if latest_main(root) != sha:
            raise ValueError("newer main exists; old deployment plan rejected")
        if old:
            recorded = read_json(root / "deployed.json")
            if (
                recorded.get("binary_sha256") != digest(release_binary(root, old))
                or (root / "current").resolve() != (root / "releases" / old).resolve()
            ):
                raise ValueError(
                    "current deployed binary differs from recorded release"
                )
        if old and not healthy(config["port"], old):
            raise RuntimeError(
                "current service is unhealthy; preserve it for diagnosis"
            )
        if not old and (root / "data").exists() != (root / "master.key").exists():
            raise RuntimeError("initial data and key are inconsistent; preserve them")
        if not service_plist_valid(root, config):
            raise RuntimeError("service installation is missing or changed")
        launchctl(root, config, "stop")
        await_health(config["port"], False)
        backup = None
        if old:
            backup = root / "backups" / (str(time.time_ns()) + "-" + old + ".backup")
            try:
                run(
                    str(release_binary(root, old)),
                    "backup",
                    "--data-dir",
                    str(root / "data"),
                    "--output",
                    str(backup),
                )
            except Exception:
                launchctl(root, config, "start")
                await_health(config["port"], True, old)
                raise
        elif not (root / "data").exists():
            password = root / "private" / "initial-admin-password.txt"
            with password.open("x") as stream:
                run(
                    str(release_binary(root, sha)),
                    "init",
                    "--data-dir",
                    str(root / "data"),
                    "--key-file",
                    str(root / "master.key"),
                    stdout=stream,
                )
            password.chmod(0o600)
        try:
            switch_current(root, sha)
            launchctl(root, config, "start")
            await_health(config["port"], True, sha)
            save_json(
                root / "deployed.json",
                {
                    "sha": sha,
                    "previous_sha": old,
                    "binary_sha256": plan["binary_sha256"],
                },
            )
        except Exception:
            launchctl(root, config, "stop")
            await_health(config["port"], False)
            if backup:
                failed = root / "failed-data" / (str(time.time_ns()) + "-" + sha)
                (root / "data").rename(failed)
                run(
                    str(release_binary(root, old)),
                    "restore",
                    "--input",
                    str(backup),
                    "--data-dir",
                    str(root / "data"),
                    "--key-file",
                    str(root / "master.key"),
                )
            switch_current(root, old)
            if old:
                launchctl(root, config, "start")
                await_health(config["port"], True, old)
            raise
        print("Deployed:", sha)
        print("URL: http://127.0.0.1:" + str(config["port"]) + "/admin/")


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["prepare", "activate"])
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--sha")
    parser.add_argument("--plan", type=Path, required=True)
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args()
    root = args.root.expanduser().resolve()
    if args.action == "prepare":
        if not args.sha:
            parser.error("prepare requires --sha")
        prepare(root, args.sha, args.plan, args.github_output)
    else:
        activate(root, args.plan)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("Deployment failed:", error, file=sys.stderr)
        sys.exit(1)
