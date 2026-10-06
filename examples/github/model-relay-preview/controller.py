#!/usr/bin/env python3
"""Trusted local release controller for the Model Relay CLI contract."""

import argparse
import base64
import fcntl
import hashlib
import json
import os
import plistlib
import selectors
import shutil
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
from contextlib import contextmanager
from pathlib import Path


def run(*args, cwd=None, stdout=None, check=True, env=None, timeout=None):
    return subprocess.run(
        args,
        cwd=cwd,
        stdout=stdout,
        stderr=subprocess.STDOUT if stdout else None,
        check=check,
        env=env,
        text=stdout is None,
        timeout=timeout,
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
    with temporary.open("w") as stream:
        stream.write(json.dumps(value, sort_keys=True, indent=2) + "\n")
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temporary, path)
    sync_path(path.parent)


def sync_path(path):
    descriptor = os.open(path, os.O_RDONLY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


_LOCK_FD = None


@contextmanager
def locked(root):
    global _LOCK_FD
    with (root / "deploy.lock").open("a") as stream:
        deadline = time.monotonic() + 10
        while True:
            try:
                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
                break
            except BlockingIOError:
                if time.monotonic() >= deadline:
                    raise RuntimeError(
                        "deployment operation still owns the lock; retry later"
                    )
                time.sleep(0.1)
        _LOCK_FD = stream.fileno()
        try:
            yield
        finally:
            # Closing, rather than LOCK_UN, preserves the inherited lock while a
            # surviving supervised product command still owns this open file.
            _LOCK_FD = None


def settings(root):
    if root.stat().st_mode & 0o077:
        raise ValueError("deployment root is not private to its owner")
    value = read_json(root / "preview.json")
    if set(value) != {"repository", "port", "label"}:
        raise ValueError("unexpected preview configuration")
    return value


def require_isolation(root, protected):
    """Refuse an operator-selected drill target that aliases the preview."""
    root, protected = root.resolve(), protected.expanduser().resolve()
    if root.is_relative_to(protected) or protected.is_relative_to(root):
        raise ValueError("deployment roots overlap")
    config, other = settings(root), settings(protected)
    if config["repository"] != other["repository"]:
        raise ValueError("isolated installation belongs to a different repository")
    if config["port"] == other["port"] or config["label"] == other["label"]:
        raise ValueError("deployment installations share a service")
    for installation in (root, protected):
        paths = [installation / name for name in ("current", "repository", "releases")]
        releases = installation / "releases"
        if releases.exists():
            for release in releases.iterdir():
                paths.extend(
                    [
                        release,
                        release / "source",
                        release / "source/bin",
                        release / "source/bin/model-relay",
                    ]
                )
        for path in paths:
            if not path.resolve().is_relative_to(installation):
                raise ValueError(
                    "deployment paths overlap or escape their installation"
                )
    state_names = (
        "data",
        "master.key",
        "logs",
        "plans",
        "backups",
        "failed-data",
        "private",
        "activation.json",
        "deployed.json",
        "deploy.lock",
    )

    def state_files(installation):
        identities = set()
        pending = [installation / name for name in state_names]
        while pending:
            path = pending.pop()
            if path.is_symlink():
                raise ValueError(
                    "deployment state has a link that could overlap installations"
                )
            try:
                metadata = path.stat()
            except FileNotFoundError:
                continue
            if path.is_dir():
                pending.extend(path.iterdir())
            else:
                identities.add((metadata.st_dev, metadata.st_ino))
        return identities

    if state_files(root) & state_files(protected):
        raise ValueError("deployment installations have shared state")


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
        run(
            "launchctl",
            "bootout",
            domain + "/" + config["label"],
            check=False,
            timeout=10,
        )
    else:
        run(
            "launchctl",
            "bootstrap",
            domain,
            str(service_plist(root, config)),
            timeout=10,
        )


def healthy(port, version=None, schema=None):
    try:
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open(f"http://127.0.0.1:{port}/healthz", timeout=2) as response:
            raw = response.read(4097)
            if len(raw) > 4096:
                return False
            result = json.loads(raw)
            if not isinstance(result, dict):
                return False
            return (
                response.status == 200
                and result.get("status") == "ok"
                and (version is None or result.get("version") == version)
                and (
                    schema is None
                    or (
                        type(result.get("storage_schema")) is int
                        and result["storage_schema"] == schema
                    )
                )
            )
    except (OSError, ValueError, urllib.error.URLError):
        return False


def port_open(port):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as connection:
        connection.settimeout(2)
        return connection.connect_ex(("127.0.0.1", port)) == 0


def await_health(port, expected, version=None, timeout=20, schema=None):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if healthy(port, version, schema) if expected else not port_open(port):
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


def latest_main(root, token=None):
    environment = os.environ.copy()
    environment["GIT_TERMINAL_PROMPT"] = "0"
    if token:
        credentials = base64.b64encode(("x-access-token:" + token).encode()).decode()
        environment["GIT_CONFIG_COUNT"] = "1"
        environment["GIT_CONFIG_KEY_0"] = "http.https://github.com/.extraheader"
        environment["GIT_CONFIG_VALUE_0"] = "AUTHORIZATION: basic " + credentials
    run(
        "git",
        "-C",
        str(root / "repository"),
        "fetch",
        "origin",
        "main",
        env=environment,
        timeout=30,
    )
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


CONTROLLER_CONTRACT = 1


class ProductStillRunning(RuntimeError):
    pass


class ProductCommandFailed(RuntimeError):
    """The worker positively confirmed cleanup after a command failure."""


def capture_command(args, timeout, lock_fd=None):
    """Worker owns the deadline and lock even if its controller parent exits."""
    process = subprocess.Popen(
        args,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        start_new_session=True,
        pass_fds=(() if lock_fd is None else (lock_fd,)),
    )
    streams = {process.stdout: bytearray(), process.stderr: bytearray()}
    deadline = time.monotonic() + timeout
    failure = None
    output = b""
    try:
        with selectors.DefaultSelector() as selector:
            for stream in streams:
                selector.register(stream, selectors.EVENT_READ)
            while selector.get_map():
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise RuntimeError("product command timed out")
                for key, _ in selector.select(remaining):
                    chunk = os.read(
                        key.fileobj.fileno(), 4097 - len(streams[key.fileobj])
                    )
                    if not chunk:
                        selector.unregister(key.fileobj)
                    else:
                        streams[key.fileobj].extend(chunk)
                        if len(streams[key.fileobj]) > 4096:
                            raise ValueError("product command result exceeds limit")
            if process.wait(timeout=max(0.01, deadline - time.monotonic())):
                raise RuntimeError("product command failed")
        output = bytes(streams[process.stdout])
    except Exception as error:
        failure = error
    finally:
        # Every cleanup failure is uncertainty, never an ordinary CLI failure.
        try:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait(timeout=3)
            deadline = time.monotonic() + 3
            while True:
                try:
                    os.killpg(process.pid, 0)
                except ProcessLookupError:
                    break
                if time.monotonic() >= deadline:
                    raise ProductStillRunning("product process group has not exited")
                time.sleep(0.02)
        except Exception as error:
            raise ProductStillRunning(
                "product cleanup could not be confirmed"
            ) from error
        finally:
            for stream in streams:
                stream.close()
    if failure:
        raise ProductCommandFailed("product command failed or timed out") from failure
    return output


def product_command(*args, timeout=60):
    inherited = () if _LOCK_FD is None else (_LOCK_FD,)
    worker = subprocess.Popen(
        [
            sys.executable,
            str(Path(__file__).resolve()),
            "_product-command",
            str(timeout),
            str(_LOCK_FD if _LOCK_FD is not None else -1),
            *args,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        pass_fds=inherited,
        start_new_session=True,
    )
    try:
        stdout, _ = worker.communicate(timeout=timeout + 8)
    except Exception as error:
        # A surviving product keeps the lock. This controller must not attempt
        # recovery under its own lock after uncertain supervisor communication.
        try:
            worker.kill()
            worker.wait(timeout=3)
        finally:
            raise ProductStillRunning(
                "product supervisor completion is unknown; preserve installation"
            ) from error
    finally:
        worker.stdout.close()
        worker.stderr.close()
    if worker.returncode == 65:
        raise RuntimeError("product command failed or timed out")
    if worker.returncode:
        raise ProductStillRunning("product supervisor exited without confirmed cleanup")
    return stdout


def command_json(*args, timeout=15):
    return json.loads(product_command(*args, timeout=timeout))


def validate_contract(value, sha):
    if not isinstance(value, dict) or set(value) != {
        "contract_version",
        "binary_version",
        "storage",
    }:
        raise ValueError("invalid deployment contract")
    if (
        type(value["contract_version"]) is not int
        or value["contract_version"] != 1
        or value["binary_version"] != sha
    ):
        raise ValueError("unsupported deployment contract or binary version")
    storage = value["storage"]
    fields = {
        "init_schema",
        "serve_schemas",
        "upgrade_from",
        "explicit_upgrade",
        "backup_schemas",
        "restore_schemas",
    }
    if not isinstance(storage, dict) or set(storage) != fields:
        raise ValueError("invalid storage contract")
    if (
        type(storage["init_schema"]) is not int
        or storage["init_schema"] not in (1, 2)
        or type(storage["explicit_upgrade"]) is not bool
    ):
        raise ValueError("unsupported storage contract")
    for name in ("serve_schemas", "upgrade_from", "backup_schemas", "restore_schemas"):
        values = storage[name]
        if (
            not isinstance(values, list)
            or any(type(v) is not int or v not in (1, 2) for v in values)
            or values != sorted(set(values))
        ):
            raise ValueError("invalid storage schema set")
    target = storage["init_schema"]
    if (
        storage["serve_schemas"] != [target]
        or not set(storage["serve_schemas"] + storage["upgrade_from"]).issubset(
            storage["backup_schemas"]
        )
        or not set(storage["backup_schemas"]).issubset(storage["restore_schemas"])
    ):
        raise ValueError("incompatible storage capabilities")
    if storage["explicit_upgrade"] != (target == 2) or storage["upgrade_from"] != (
        [1] if target == 2 else []
    ):
        raise ValueError("unsupported upgrade contract")
    return value


def product_contract(root, sha):
    binary = release_binary(root, sha)
    try:
        return validate_contract(command_json(str(binary), "deployment-contract"), sha)
    except ProductStillRunning:
        raise
    except (RuntimeError, ValueError):
        # Only the already-installed, unchanged legacy release is recognized.
        # A new binary without a valid contract is never assumed to use schema1.
        record = root / "deployed.json"
        if record.exists():
            previous = read_json(record)
            if (
                "deployment_contract" not in previous
                and previous.get("sha") == sha
                and previous.get("binary_sha256") == digest(binary)
                and (root / "current").resolve() == (root / "releases" / sha).resolve()
            ):
                return {
                    "contract_version": 0,
                    "binary_version": sha,
                    "storage": {
                        "init_schema": 1,
                        "serve_schemas": [1],
                        "upgrade_from": [],
                        "explicit_upgrade": False,
                        "backup_schemas": [1],
                        "restore_schemas": [1],
                    },
                }
        raise


def validate_upgrade(result, contract):
    target = contract["storage"]["init_schema"]
    if (
        not isinstance(result, dict)
        or result.get("binary_version") != contract["binary_version"]
        or type(result.get("from_schema")) is not int
        or type(result.get("schema")) is not int
        or result["schema"] != target
    ):
        raise ValueError("invalid upgrade result")
    previous = result["from_schema"]
    if not (
        (result.get("result") == "already_current" and previous == target)
        or (
            result.get("result") == "migrated"
            and previous in contract["storage"]["upgrade_from"]
            and previous != target
        )
    ):
        raise ValueError("incompatible upgrade result")


def prepare(root, sha, plan_path, output):
    if not valid_sha(sha):
        raise ValueError("target must be a full lowercase commit SHA")
    token = os.environ.pop("MODEL_RELAY_REPO_TOKEN", None)
    config = settings(root)
    with locked(root):
        latest = latest_main(root, token)
        if latest != sha:
            raise ValueError("requested commit is no longer the current main head")
        old = current_sha(root)
        pending = pending_activation(root)
        if pending and reconcile_committed(root, config, pending):
            pending = None
        if pending:
            save_json(plan_path, pending["plan"])
            if output:
                output.write_text("deploy=true\n")
            print(
                "Interrupted activation requires explicit activate recovery; no deployment performed."
            )
            return
        if old == sha:
            verify_deployed(root, config)
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
        contract = product_contract(root, sha)
        plan = {
            **candidate,
            "controller_contract": CONTROLLER_CONTRACT,
            "deployment_contract": contract,
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
    sync_path(root)


def pending_activation(root):
    path = root / "activation.json"
    if not path.exists():
        return None
    value = read_json(path)
    if value["stage"] in ("committed", "rolled_back"):
        return None
    return value


def verify_deployed(root, config):
    recorded = read_json(root / "deployed.json")
    sha = recorded["sha"]
    if (
        not valid_sha(sha)
        or recorded.get("binary_sha256") != digest(release_binary(root, sha))
        or (root / "current").resolve() != (root / "releases" / sha).resolve()
    ):
        raise ValueError("current deployed binary differs from recorded release")
    if not healthy(config["port"], sha, recorded.get("storage_schema")):
        raise RuntimeError("recorded deployment is not healthy")
    return recorded


def deployment_record(journal):
    plan = journal["plan"]
    return {
        "sha": plan["sha"],
        "previous_sha": plan["previous_sha"],
        "binary_sha256": plan["binary_sha256"],
        "deployment_contract": plan["deployment_contract"],
        "storage_schema": plan["deployment_contract"]["storage"]["init_schema"],
    }


def reconcile_committed(root, config, journal):
    record = root / "deployed.json"
    if (
        journal["stage"] != "candidate_healthy"
        or not record.exists()
        or read_json(record) != deployment_record(journal)
    ):
        return False
    try:
        verify_deployed(root, config)
    except RuntimeError:
        # A recorded candidate which is no longer healthy still needs recovery.
        return False
    if (
        journal["key_sha256"] is not None
        and digest(root / "master.key") != journal["key_sha256"]
    ):
        raise ValueError("master key changed; preserve interrupted deployment")
    stage(root, journal, "committed")
    return True


def stage(root, journal, name):
    journal["stage"] = name
    save_json(root / "activation.json", journal)


def recovery(root, config, journal):
    """Restore the pinned original snapshot; never back up uncertain new data."""
    old = journal["previous"]
    name = journal["stage"]
    allowed = {
        "stopping",
        "initializing",
        "backup_complete",
        "upgrade_confirmed",
        "candidate_healthy",
        "recovery_pending",
        "restore_ready",
        "restored",
    }
    if name not in allowed or not str(journal["attempt"]).isdigit():
        raise ValueError("unknown activation stage; preserve installation")
    if not service_plist_valid(root, config):
        raise ValueError("service installation changed during recovery")
    if old:
        sha = old["sha"]
        if (
            not valid_sha(sha)
            or digest(release_binary(root, sha)) != old["binary_sha256"]
        ):
            raise ValueError("previous binary changed; recovery incomplete")
        if digest(root / "master.key") != journal["key_sha256"]:
            raise ValueError("master key changed; recovery incomplete")
        backup = root / "backups" / (journal["attempt"] + "-rollback.backup")
        if journal["backup_sha256"] is not None:
            if digest(backup) != journal["backup_sha256"]:
                raise ValueError("rollback backup changed; recovery incomplete")
        elif name not in ("stopping", "restored"):
            raise ValueError("rollback backup is unconfirmed; recovery incomplete")
    launchctl(root, config, "stop")
    await_health(config["port"], False)
    if old and name == "stopping":
        # No mutating command is allowed before durable backup_complete.
        stage(root, journal, "restored")
    else:
        # Prevent automatic launchd restart against partly migrated data.
        switch_current(root, None)
        failed = root / "failed-data" / journal["attempt"]
        failed.mkdir(exist_ok=True)
        if name not in ("restore_ready", "restored"):
            stage(root, journal, "recovery_pending")
            pairs = [(root / "data", failed / "data")]
            if not old:
                pairs += [
                    (root / "master.key", failed / "master.key"),
                    (
                        root / "private/initial-admin-password.txt",
                        failed / "initial-admin-password.txt",
                    ),
                ]
            for source, target in pairs:
                if source.exists():
                    if target.exists():
                        raise ValueError(
                            "ambiguous recovery data; preserve both directories"
                        )
                    source.rename(target)
                    sync_path(source.parent)
                    sync_path(target.parent)
            if old:
                restored = failed / "restored"
                if restored.exists():
                    # A failed/uncertain restore is evidence, never a usable copy.
                    restored.rename(failed / ("partial-restore-" + str(time.time_ns())))
                product_command(
                    str(release_binary(root, old["sha"])),
                    "restore",
                    "--input",
                    str(backup),
                    "--data-dir",
                    str(restored),
                    "--key-file",
                    str(root / "master.key"),
                    timeout=120,
                )
                if not restored.is_dir():
                    raise RuntimeError("restore did not produce an installation")
                sync_path(restored)
                stage(root, journal, "restore_ready")
            else:
                stage(root, journal, "restored")
        if journal["stage"] == "restore_ready":
            restored, data = failed / "restored", root / "data"
            if restored.is_dir() and not data.exists():
                restored.rename(data)
                sync_path(failed)
                sync_path(root)
            elif restored.exists() or not data.is_dir():
                raise ValueError(
                    "ambiguous restored installation; preserve directories"
                )
            # If the move completed before a crash, this only records completion.
            stage(root, journal, "restored")
    switch_current(root, old["sha"] if old else None)
    if old:
        launchctl(root, config, "start")
        await_health(config["port"], True, old["sha"], schema=old.get("storage_schema"))
        save_json(root / "deployed.json", old)
    else:
        (root / "deployed.json").unlink(missing_ok=True)
        sync_path(root)
    stage(root, journal, "rolled_back")


def activate(root, plan_path):
    token = os.environ.pop("MODEL_RELAY_REPO_TOKEN", None)
    config = settings(root)
    with locked(root):
        interrupted = pending_activation(root)
        if interrupted and reconcile_committed(root, config, interrupted):
            print("Already deployed:", interrupted["plan"]["sha"])
            return
        if interrupted:
            try:
                recovery(root, config, interrupted)
            except Exception as error:
                raise RuntimeError(
                    "interrupted activation recovery incomplete; preserve installation"
                ) from error
            raise RuntimeError(
                "interrupted activation recovered; explicitly deploy again for a new attempt"
            )
        plan = read_json(plan_path)
        sha = plan["sha"]
        if not valid_sha(sha):
            raise ValueError("deployment plan has invalid commit SHA")
        old = current_sha(root)
        if old == sha:
            verify_deployed(root, config)
            print("Already deployed:", sha)
            return
        contract = product_contract(root, sha)
        if plan != {
            "sha": sha,
            "binary_sha256": digest(release_binary(root, sha)),
            "controller_contract": CONTROLLER_CONTRACT,
            "deployment_contract": contract,
            "previous_sha": old,
            "repository": config["repository"],
            "port": config["port"],
        }:
            raise ValueError("deployment plan or release changed; prepare again")
        if latest_main(root, token) != sha:
            raise ValueError("newer main exists; old deployment plan rejected")
        recorded = verify_deployed(root, config) if old else None
        if old:
            previous_contract = product_contract(root, old)
            if previous_contract["storage"]["init_schema"] not in (
                contract["storage"]["serve_schemas"]
                + contract["storage"]["upgrade_from"]
            ):
                raise ValueError(
                    "candidate cannot serve or upgrade the deployed schema"
                )
        elif any(
            (root / p).exists()
            for p in ("data", "master.key", "private/initial-admin-password.txt")
        ):
            raise RuntimeError(
                "unrecorded installation exists; preserve it for diagnosis"
            )
        if not service_plist_valid(root, config):
            raise RuntimeError("service installation is missing or changed")
        journal = {
            "attempt": str(time.time_ns()),
            "plan": plan,
            "previous": recorded,
            "key_sha256": digest(root / "master.key") if old else None,
            "backup_sha256": None,
        }
        stage(root, journal, "stopping" if old else "initializing")
        try:
            launchctl(root, config, "stop")
            await_health(config["port"], False)
            if old:
                backup = root / "backups" / (journal["attempt"] + "-rollback.backup")
                product_command(
                    str(release_binary(root, old)),
                    "backup",
                    "--data-dir",
                    str(root / "data"),
                    "--output",
                    str(backup),
                    timeout=60,
                )
                if not backup.is_file() or backup.stat().st_size == 0:
                    raise ValueError("backup did not produce a nonempty archive")
                sync_path(backup)
                sync_path(backup.parent)
                journal["backup_sha256"] = digest(backup)
                stage(root, journal, "backup_complete")
                if contract["storage"]["explicit_upgrade"]:
                    result = command_json(
                        str(release_binary(root, sha)),
                        "upgrade",
                        "--data-dir",
                        str(root / "data"),
                        "--key-file",
                        str(root / "master.key"),
                        "--output",
                        str(
                            root / "backups" / (journal["attempt"] + "-upgrade.backup")
                        ),
                        timeout=120,
                    )
                    validate_upgrade(result, contract)
            else:
                result = product_command(
                    str(release_binary(root, sha)),
                    "init",
                    "--data-dir",
                    str(root / "data"),
                    "--key-file",
                    str(root / "master.key"),
                )
                password = root / "private/initial-admin-password.txt"
                with password.open("xb") as stream:
                    stream.write(result)
                    stream.flush()
                    os.fsync(stream.fileno())
                password.chmod(0o600)
            stage(root, journal, "upgrade_confirmed")
            switch_current(root, sha)
            launchctl(root, config, "start")
            await_health(
                config["port"], True, sha, schema=contract["storage"]["init_schema"]
            )
            stage(root, journal, "candidate_healthy")
            save_json(root / "deployed.json", deployment_record(journal))
            stage(root, journal, "committed")
        except ProductStillRunning:
            # Do not overlap recovery with an uncertain mutating process.
            raise
        except Exception:
            try:
                recovery(root, config, journal)
            except Exception as error:
                raise RuntimeError(
                    "deployment failed; recovery incomplete; preserve installation"
                ) from error
            raise
        print("Deployed:", sha)
        print("URL: http://127.0.0.1:" + str(config["port"]) + "/admin/")


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["prepare", "activate"])
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--sha")
    parser.add_argument("--protected-root", type=Path)
    parser.add_argument("--plan", type=Path, required=True)
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args()
    root = args.root.expanduser().resolve()
    if args.protected_root is not None:
        require_isolation(root, args.protected_root)
    if args.action == "prepare":
        if not args.sha:
            parser.error("prepare requires --sha")
        prepare(root, args.sha, args.plan, args.github_output)
    else:
        activate(root, args.plan)


if __name__ == "__main__":
    try:
        if len(sys.argv) > 1 and sys.argv[1] == "_product-command":
            # Parent cancellation does not abandon a mutating CLI operation.
            # The worker's own deadline kills its process group and closes lock.
            signal.signal(signal.SIGTERM, signal.SIG_IGN)
            signal.signal(signal.SIGINT, signal.SIG_IGN)
            descriptor = int(sys.argv[3])
            sys.stdout.buffer.write(
                capture_command(
                    sys.argv[4:],
                    float(sys.argv[2]),
                    None if descriptor < 0 else descriptor,
                )
            )
        else:
            main()
    except Exception as error:
        print("Deployment failed:", error, file=sys.stderr)
        if len(sys.argv) > 1 and sys.argv[1] == "_product-command":
            # Only status 65 proves failed CLI cleanup completed. Signals and all
            # other failures are uncertain to the parent, even ordinary exit 1.
            sys.exit(65 if isinstance(error, ProductCommandFailed) else 75)
        sys.exit(1)
