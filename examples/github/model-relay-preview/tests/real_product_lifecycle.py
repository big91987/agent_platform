#!/usr/bin/env python3
"""Opt-in controller/real-Go regression. Not launchd, UI or GitHub evidence.

Builds both exact product commits through their unchanged make verify entry.
Only the service manager is replaced; real Go CLI, HTTP, data and controller
state transitions run in a fresh private fixture. No existing installation is read.
"""

import argparse
import contextlib
import http.cookiejar
import importlib.util
import json
import os
import re
import socket
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path
from unittest.mock import patch


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def git(*args):
    return subprocess.check_output(["git", *map(str, args)], text=True).strip()


class InterruptedAfterUpgrade(BaseException):
    pass


class ProcessService:
    """Real product serve; no launchd installation or fake healthy response."""

    def __init__(self, controller):
        self.controller = controller
        self.process = None
        self.log = None
        self.trace = []
        self.block_candidate_once = None
        self.blocker = None

    def stop(self):
        if self.process is not None:
            if self.process.poll() is None:
                self.process.terminate()
            try:
                self.process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=5)
            self.process = None
        if self.log is not None:
            self.log.close()
            self.log = None
        if self.blocker is not None:
            self.blocker.close()
            self.blocker = None

    def __call__(self, root, config, action):
        self.trace.append(action)
        if action == "stop":
            self.stop()
            return
        assert action == "start"
        assert self.process is None
        sha = (root / "current").resolve(strict=True).name
        self.trace.append("start:" + sha)
        if self.block_candidate_once == sha:
            self.block_candidate_once = None
            self.blocker = socket.socket()
            self.blocker.bind(("127.0.0.1", config["port"]))
            self.blocker.listen(4)
        self.log = (root / "logs" / ("serve-" + str(time.time_ns()) + ".log")).open(
            "wb"
        )
        self.process = subprocess.Popen(
            [
                str(self.controller.release_binary(root, sha)),
                "serve",
                "--data-dir",
                str(root / "data"),
                "--key-file",
                str(root / "master.key"),
                "--listen",
                "127.0.0.1:" + str(config["port"]),
            ],
            stdout=self.log,
            stderr=self.log,
        )


class Admin:
    def __init__(self, root, port):
        self.base = "http://127.0.0.1:" + str(port)
        self.http = urllib.request.build_opener(
            urllib.request.ProxyHandler({}),
            urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()),
        )
        self.csrf = ""
        password = re.search(
            r"Administrator password \(shown once\):\s*(\S+)",
            (root / "private/initial-admin-password.txt").read_text(),
        )[1]
        self.csrf = self.call("login", "POST", {"password": password})["csrf"]

    def call(self, path, method="GET", body=None):
        request = urllib.request.Request(
            self.base + "/admin/api/" + path,
            method=method,
            data=None if body is None else json.dumps(body).encode(),
            headers={
                "Origin": self.base,
                "Content-Type": "application/json",
                "X-CSRF-Token": self.csrf,
            },
        )
        with self.http.open(request, timeout=10) as response:
            data = response.read()
            return json.loads(data) if data else None

    def close(self):
        self.call("logout", "POST", {})


def run_case(args, controller, legacy):
    root = args.work / "installation"
    root.mkdir()
    for name in ("logs", "plans", "releases", "backups", "failed-data", "private"):
        (root / name).mkdir()
    remote = args.work / "remote.git"
    git("clone", "--bare", "--shared", args.project, remote)
    git("--git-dir", remote, "update-ref", "refs/heads/main", args.old_sha)
    git("clone", "--no-checkout", remote, root / "repository")
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    controller.save_json(
        root / "preview.json",
        {
            "repository": "fixture/model-relay",
            "port": port,
            "label": "fixture.real-go",
        },
    )
    service = ProcessService(controller)
    old_plan = root / "plans/legacy.json"
    plan = root / "plans/candidate.json"
    report = {
        "evidence_kind": "controller-real-go-isolated",
        "scope_exclusions": ["launchd", "GitHub Actions", "UI", "supplier"],
        "old_sha": args.old_sha,
        "candidate_sha": args.candidate_sha,
        "controller_sha256": controller.digest(Path(controller.__file__)),
        "legacy_controller_sha256": controller.digest(args.legacy_controller),
        "harness_sha256": controller.digest(Path(__file__)),
        "result": "running",
        "checks": [],
    }

    def passed(name):
        report["checks"].append(name)
        controller.save_json(args.work / "evidence.json", report)
        print("PASS " + name, flush=True)

    def expect_failure(call, pattern):
        try:
            call()
        except (RuntimeError, ValueError) as error:
            assert re.search(pattern, str(error)), (
                type(error).__name__ + ": " + str(error)
            )
        else:
            raise AssertionError("expected failure: " + pattern)

    original_command = controller.product_command
    commands = []

    def observed_command(*command, **kwargs):
        commands.append((Path(command[0]).parents[2].name, command[1]))
        return original_command(*command, **kwargs)

    def capture_old():
        admin = Admin(root, port)
        try:
            return {
                key: admin.call(key)
                for key in ("upstream", "models", "keys", "requests")
            }
        finally:
            admin.close()

    def check_old():
        assert controller.healthy(port, args.old_sha)
        assert controller.read_json(root / "deployed.json") == old_deployed
        assert controller.digest(root / "master.key") == key_digest
        assert capture_old() == baseline

    # Service-manager replacement is explicit; health and product commands are real.
    try:
        with contextlib.ExitStack() as stack:
            for module in (controller, legacy):
                stack.enter_context(
                    patch.object(module, "service_plist_valid", return_value=True)
                )
                stack.enter_context(
                    patch.object(module, "launchctl", side_effect=service)
                )
            stack.enter_context(
                patch.object(
                    controller, "product_command", side_effect=observed_command
                )
            )
            print("Preparing exact legacy commit through make verify", flush=True)
            legacy.prepare(root, args.old_sha, old_plan, None)
            legacy.activate(root, old_plan)
            admin = Admin(root, port)
            try:
                admin.call(
                    "upstream",
                    "PUT",
                    {
                        "url": "https://example.com/v1",
                        "token": "explicit-test-placeholder",
                        "enabled": False,
                    },
                )
                admin.call(
                    "models",
                    "POST",
                    {"alias": "kept-enabled", "model": "remote-a", "enabled": True},
                )
                admin.call(
                    "models",
                    "POST",
                    {"alias": "kept-disabled", "model": "remote-b", "enabled": False},
                )
                admin.call("keys", "POST", {"name": "kept-key"})
                # A real unauthorized public request supplies history without calling any upstream.
                request = urllib.request.Request(
                    admin.base + "/v1/chat/completions",
                    data=b'{"model":"kept-enabled","messages":[]}',
                    headers={"Content-Type": "application/json"},
                    method="POST",
                )
                try:
                    admin.http.open(request, timeout=10)
                except urllib.error.HTTPError as error:
                    assert error.code == 401
                    error.close()
                else:
                    raise AssertionError(
                        "unauthorized history fixture unexpectedly accepted"
                    )
            finally:
                admin.close()
            baseline = capture_old()
            assert len(baseline["models"]) == 2 and len(baseline["keys"]) == 1
            assert len(baseline["requests"]["data"]) == 1
            old_deployed = controller.read_json(root / "deployed.json")
            key_digest = controller.digest(root / "master.key")
            legacy_contract = controller.product_contract(root, args.old_sha)
            assert legacy_contract["contract_version"] == 0
            controller.prepare(root, args.old_sha, root / "plans/noop.json", None)
            check_old()
            passed("known unchanged legacy and nonempty API fixture")

            git(
                "--git-dir", remote, "update-ref", "refs/heads/main", args.candidate_sha
            )
            print("Preparing exact candidate through make verify", flush=True)
            controller.prepare(root, args.candidate_sha, plan, None)
            candidate = controller.read_json(plan)
            assert candidate["deployment_contract"]["storage"]["init_schema"] == 2
            report["old_binary_sha256"] = old_deployed["binary_sha256"]
            report["candidate_binary_sha256"] = candidate["binary_sha256"]

            def obstruct_output(verb):
                def call(*command, **kwargs):
                    if command[1] == verb:
                        Path(command[command.index("--output") + 1]).write_bytes(
                            b"explicit output collision fixture"
                        )
                    return observed_command(*command, **kwargs)

                return call

            for verb in ("backup", "upgrade"):
                commands.clear()
                with patch.object(
                    controller, "product_command", side_effect=obstruct_output(verb)
                ):
                    expect_failure(
                        lambda: controller.activate(root, plan),
                        "product command failed",
                    )
                check_old()
                assert (
                    controller.read_json(root / "activation.json")["stage"]
                    == "rolled_back"
                )
                assert (
                    (args.candidate_sha, "upgrade") not in commands
                    if verb == "backup"
                    else (args.old_sha, "restore") in commands
                )
                passed("real Go " + verb + " destination refusal and old data recovery")

            def obstruct_restore(*command, **kwargs):
                if command[1] == "restore":
                    target = Path(command[command.index("--data-dir") + 1])
                    target.mkdir()
                    (target / "collision-fixture").write_text(
                        "explicit nonempty restore target"
                    )
                return observed_command(*command, **kwargs)

            service.block_candidate_once = args.candidate_sha
            with patch.object(
                controller, "product_command", side_effect=obstruct_restore
            ):
                expect_failure(
                    lambda: controller.activate(root, plan), "recovery incomplete"
                )
            journal = controller.read_json(root / "activation.json")
            assert journal["stage"] == "recovery_pending"
            assert not controller.healthy(port)
            snapshot_digest = journal["backup_sha256"]
            expect_failure(
                lambda: controller.activate(root, plan),
                "interrupted activation recovered",
            )
            check_old()
            assert (
                controller.read_json(root / "activation.json")["backup_sha256"]
                == snapshot_digest
            )
            assert list(
                (root / "failed-data" / journal["attempt"]).glob("partial-restore-*")
            )
            passed(
                "real bind failure and real restore refusal preserve evidence; explicit retry restores old"
            )

            real_stage = controller.stage

            def interrupt(root, journal, name):
                real_stage(root, journal, name)
                if name == "upgrade_confirmed":
                    raise InterruptedAfterUpgrade()

            try:
                with patch.object(controller, "stage", side_effect=interrupt):
                    controller.activate(root, plan)
            except InterruptedAfterUpgrade:
                pass
            else:
                raise AssertionError("interruption fixture did not execute")
            journal = controller.read_json(root / "activation.json")
            assert journal["stage"] == "upgrade_confirmed"
            commands.clear()
            controller.prepare(root, args.candidate_sha, plan, None)
            assert not commands
            expect_failure(
                lambda: controller.activate(root, plan),
                "interrupted activation recovered",
            )
            check_old()
            assert (args.old_sha, "backup") not in commands
            passed(
                "durable post-upgrade interruption recovers original without old backup of new schema"
            )

            commands.clear()
            controller.activate(root, plan)
            assert controller.healthy(port, args.candidate_sha, 2)
            assert [
                (sha, verb)
                for sha, verb in commands
                if verb in ("backup", "upgrade", "restore")
            ] == [(args.old_sha, "backup"), (args.candidate_sha, "upgrade")]
            admin = Admin(root, port)
            try:
                assert admin.call("keys") == baseline["keys"]
                models = {m["alias"]: m for m in admin.call("models")["data"]}
                for old in baseline["models"]:
                    new = models[old["alias"]]
                    assert new["enabled"] == old["enabled"]
                    assert new["candidates"][0]["remote_model"] == old["model"]
                history = admin.call("requests")["data"]
                assert len(history) == 1
                assert all(
                    history[0].get(k) == v
                    for k, v in baseline["requests"]["data"][0].items()
                )
            finally:
                admin.close()
            assert controller.digest(root / "master.key") == key_digest
            passed(
                "real legacy backup candidate upgrade and schema2 service preserve identities and history"
            )
            commands.clear()
            service.trace.clear()
            controller.activate(root, plan)
            assert not commands and not service.trace
            passed("healthy same-SHA activation has no service or product side effects")
            git("--git-dir", remote, "update-ref", "refs/heads/main", args.old_sha)
            service.trace.clear()
            expect_failure(
                lambda: controller.prepare(root, args.old_sha, old_plan, None),
                "product command failed",
            )
            assert not service.trace and controller.healthy(port, args.candidate_sha, 2)
            passed(
                "contractless legacy candidate cannot downgrade schema2 or stop current service"
            )
            report["result"] = "passed"
    except BaseException:
        report["result"] = "failed"
        raise
    finally:
        service.stop()
        controller.save_json(args.work / "evidence.json", report)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", type=Path, required=True)
    parser.add_argument("--old-sha", required=True)
    parser.add_argument("--candidate-sha", required=True)
    parser.add_argument("--legacy-controller", type=Path, required=True)
    parser.add_argument("--work", type=Path, required=True)
    args = parser.parse_args()
    args.project = args.project.resolve()
    args.legacy_controller = args.legacy_controller.resolve()
    args.work = args.work.resolve()
    if args.work.exists():
        parser.error(
            "work must be a new private directory; existing evidence is never overwritten"
        )
    controller = load(
        "real_current_controller", Path(__file__).resolve().parents[1] / "controller.py"
    )
    assert controller.valid_sha(args.old_sha) and controller.valid_sha(
        args.candidate_sha
    )
    assert args.old_sha != args.candidate_sha
    for sha in (args.old_sha, args.candidate_sha):
        assert git("-C", args.project, "rev-parse", sha + "^{commit}") == sha
    os.umask(0o077)
    args.work.mkdir(parents=True, mode=0o700)
    legacy = load("real_legacy_controller", args.legacy_controller)
    run_case(args, controller, legacy)


if __name__ == "__main__":
    main()
