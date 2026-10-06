import fcntl
import importlib.util
import json
import os
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / "controller.py"
SPEC = importlib.util.spec_from_file_location("model_relay_preview", SOURCE)
controller = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(controller)


def command(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


SCRIPT = """#!/usr/bin/env python3
import pathlib, sys, os, json, subprocess
schema = 1
if os.environ.get('MODEL_RELAY_REPO_TOKEN'):
    sys.exit(31)
args = sys.argv
sha = subprocess.check_output(['git', '-C', str(pathlib.Path(__file__).resolve().parent.parent), 'rev-parse', 'HEAD'], text=True).strip()
if args[1] == 'backup':
    pathlib.Path(args[args.index('--output') + 1]).write_text((pathlib.Path(args[args.index('--data-dir') + 1]) / 'state').read_text())
elif args[1] == 'restore':
    directory = pathlib.Path(args[args.index('--data-dir') + 1])
    directory.mkdir()
    (directory / 'state').write_text(pathlib.Path(args[args.index('--input') + 1]).read_text())
elif args[1] == 'init':
    directory = pathlib.Path(args[args.index('--data-dir') + 1])
    directory.mkdir()
    (directory / 'state').write_text('new')
    pathlib.Path(args[args.index('--key-file') + 1]).write_text('key')
    print('Administrator password (shown once): test-only')
elif args[1] == 'deployment-contract':
    print(json.dumps({'contract_version': 1, 'binary_version': sha, 'storage': {'init_schema': schema, 'serve_schemas': [schema], 'upgrade_from': [1] if schema == 2 else [], 'explicit_upgrade': schema == 2, 'backup_schemas': list(range(1, schema + 1)), 'restore_schemas': list(range(1, schema + 1))}}))
elif args[1] == 'upgrade':
    directory = pathlib.Path(args[args.index('--data-dir') + 1])
    state = directory / 'state'
    before = state.read_text()
    if before == 'schema2':
        print(json.dumps({'binary_version': sha, 'result': 'already_current', 'from_schema': 2, 'schema': 2}))
    else:
        output = pathlib.Path(args[args.index('--output') + 1])
        with output.open('x') as stream:
            stream.write(before)
        state.write_text('schema2')
        if (pathlib.Path(__file__).resolve().parent.parent / 'VERSION').read_text() == 'migration fails':
            sys.exit(7)
        print(json.dumps({'binary_version': sha, 'result': 'migrated', 'from_schema': 1, 'schema': 2}))
"""


class DeploymentControllerTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.remote = self.base / "remote.git"
        self.work = self.base / "work"
        self.root = self.base / "preview"
        command("git", "init", "--bare", str(self.remote))
        command("git", "init", "-b", "main", str(self.work))
        command(
            "git", "-C", str(self.work), "config", "user.email", "test@example.invalid"
        )
        command("git", "-C", str(self.work), "config", "user.name", "Test")
        command(
            "git", "-C", str(self.work), "remote", "add", "origin", str(self.remote)
        )
        (self.work / "Makefile").write_text(
            "verify:\n\tmkdir -p bin\n\tcp service.py bin/model-relay\n\tchmod +x bin/model-relay\n"
        )
        (self.work / "service.py").write_text(SCRIPT)
        self.old = self.commit("old")
        self.root.mkdir(mode=0o700)
        for name in ("logs", "plans", "releases", "backups", "failed-data", "private"):
            (self.root / name).mkdir()
        command(
            "git",
            "clone",
            "--no-checkout",
            str(self.remote),
            str(self.root / "repository"),
        )
        (self.root / "preview.json").write_text(
            json.dumps(
                {
                    "repository": "example/model-relay",
                    "port": 5545,
                    "label": "example.preview",
                }
            )
        )
        self.plan = self.root / "plans/one.json"

    def commit(self, message):
        (self.work / "VERSION").write_text(message)
        command("git", "-C", str(self.work), "add", "Makefile", "service.py", "VERSION")
        command("git", "-C", str(self.work), "commit", "-m", message)
        command("git", "-C", str(self.work), "push", "origin", "main")
        return command("git", "-C", str(self.work), "rev-parse", "HEAD")

    def prepare(self, sha, plan=None):
        with patch.object(controller, "stamp_binary"):
            controller.prepare(self.root, sha, plan or self.plan, None)

    def fake_git_requiring_job_token(self):
        fakebin = self.base / "fakebin"
        fakebin.mkdir()
        fake_git = fakebin / "git"
        fake_git.write_text(
            "#!/usr/bin/env python3\n"
            "import os, sys\n"
            "if 'fetch' in sys.argv:\n"
            "    if os.environ.get('GIT_CONFIG_VALUE_0') != "
            "'AUTHORIZATION: basic eC1hY2Nlc3MtdG9rZW46dGVzdC10b2tlbg==':\n"
            "        sys.exit(23)\n"
            "    if 'MODEL_RELAY_REPO_TOKEN' in os.environ:\n"
            "        sys.exit(24)\n"
            "os.execv('/usr/bin/git', ['/usr/bin/git', *sys.argv[1:]])\n"
        )
        fake_git.chmod(0o700)
        return fakebin

    def test_prepare_authenticates_fetch_with_job_token(self):
        fakebin = self.fake_git_requiring_job_token()
        with patch.dict(
            os.environ,
            {
                "MODEL_RELAY_REPO_TOKEN": "test-token",
                "PATH": str(fakebin) + os.pathsep + os.environ["PATH"],
            },
        ):
            self.prepare(self.old)
        self.assertEqual(json.loads(self.plan.read_text())["sha"], self.old)

    def test_activate_authenticates_recheck_without_exposing_token_to_service(self):
        self.prepare(self.old)
        fakebin = self.fake_git_requiring_job_token()
        with (
            patch.dict(
                os.environ,
                {
                    "MODEL_RELAY_REPO_TOKEN": "test-token",
                    "PATH": str(fakebin) + os.pathsep + os.environ["PATH"],
                },
            ),
            patch.object(controller, "service_plist_valid", return_value=True),
            patch.object(controller, "launchctl"),
            patch.object(controller, "await_health"),
        ):
            controller.activate(self.root, self.plan)
        self.assertEqual(controller.current_sha(self.root), self.old)

    def test_prepare_does_not_expose_job_token_to_project_verification(self):
        makefile = self.work / "Makefile"
        makefile.write_text(
            makefile.read_text().replace(
                "verify:\n", 'verify:\n\ttest -z "$$MODEL_RELAY_REPO_TOKEN"\n'
            )
        )
        target = self.commit("verify cannot read job token")
        with patch.dict(os.environ, {"MODEL_RELAY_REPO_TOKEN": "test-token"}):
            self.prepare(target)
        self.assertEqual(json.loads(self.plan.read_text())["sha"], target)

    def test_exact_main_preparation_is_repeatable_and_rejects_stale_head(self):
        self.prepare(self.old)
        prepared = json.loads(self.plan.read_text())
        self.assertEqual(prepared["sha"], self.old)
        self.assertEqual(
            prepared["binary_sha256"],
            controller.digest(controller.release_binary(self.root, self.old)),
        )
        self.prepare(self.old)
        self.assertEqual(json.loads(self.plan.read_text()), prepared)
        self.commit("new")
        with self.assertRaisesRegex(ValueError, "no longer the current main"):
            self.prepare(self.old)

    def test_failed_activation_restores_data_and_previous_release(self):
        self.install_previous_release()
        (self.root / "data/state").write_text("valuable")
        new = self.commit("new")
        second = self.root / "plans/two.json"
        self.prepare(new, second)
        starts = 0

        def launch(root, config, action):
            nonlocal starts
            if action == "start":
                starts += 1
                if starts == 1:
                    (root / "data/state").write_text("candidate mutation")
                    raise RuntimeError("candidate health failed")

        with (
            patch.object(controller, "service_plist_valid", return_value=True),
            patch.object(controller, "launchctl", side_effect=launch),
            patch.object(controller, "await_health"),
            patch.object(controller, "healthy", return_value=True),
        ):
            with self.assertRaisesRegex(RuntimeError, "candidate health failed"):
                controller.activate(self.root, second)
        self.assertEqual(starts, 2)
        self.assertEqual(controller.current_sha(self.root), self.old)
        self.assertEqual(
            (self.root / "current").resolve(),
            (self.root / "releases" / self.old).resolve(),
        )
        self.assertEqual((self.root / "data/state").read_text(), "valuable")
        self.assertEqual(len(list((self.root / "backups").iterdir())), 1)
        self.assertEqual(len(list((self.root / "failed-data").iterdir())), 1)

    def install_previous_release(self):
        self.prepare(self.old)
        with (
            patch.object(controller, "service_plist_valid", return_value=True),
            patch.object(controller, "launchctl"),
            patch.object(controller, "await_health"),
        ):
            controller.activate(self.root, self.plan)
        (self.root / "data/state").write_text("valuable schema1 data")

    def prepare_schema_two(self, message):
        (self.work / "service.py").write_text(
            SCRIPT.replace("schema = 1", "schema = 2")
        )
        sha = self.commit(message)
        plan = self.root / "plans/schema-two.json"
        self.prepare(sha, plan)
        return sha, plan

    def test_migration_runs_before_candidate_start_and_preserves_old_snapshot(self):
        self.install_previous_release()
        sha, plan = self.prepare_schema_two("schema2")
        starts = []

        def launch(root, config, action):
            if action == "start":
                starts.append((root / "data/state").read_text())
                self.assertEqual(starts[-1], "schema2")

        with (
            patch.object(controller, "service_plist_valid", return_value=True),
            patch.object(controller, "launchctl", side_effect=launch),
            patch.object(controller, "await_health"),
            patch.object(controller, "healthy", return_value=True),
        ):
            controller.activate(self.root, plan)
        self.assertEqual(starts, ["schema2"])
        self.assertEqual(controller.current_sha(self.root), sha)
        backups = list((self.root / "backups").iterdir())
        self.assertGreaterEqual(len(backups), 2)
        self.assertTrue(all(p.read_text() == "valuable schema1 data" for p in backups))

    def test_failed_upgrade_restores_old_data_before_restarting_old_binary(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("migration fails")
        starts = []

        def launch(root, config, action):
            if action == "start":
                starts.append(
                    (
                        (root / "current").resolve().name,
                        (root / "data/state").read_text(),
                    )
                )

        with (
            patch.object(controller, "service_plist_valid", return_value=True),
            patch.object(controller, "launchctl", side_effect=launch),
            patch.object(controller, "await_health"),
            patch.object(controller, "healthy", return_value=True),
        ):
            with self.assertRaises(Exception):
                controller.activate(self.root, plan)
        self.assertEqual(starts, [(self.old, "valuable schema1 data")])
        self.assertEqual(controller.current_sha(self.root), self.old)
        failed_states = list((self.root / "failed-data").glob("*/data/state"))
        self.assertEqual([p.read_text() for p in failed_states], ["schema2"])

    def test_prepared_release_binds_candidate_storage_contract(self):
        _, plan = self.prepare_schema_two("schema2")
        recorded = json.loads(plan.read_text())
        self.assertEqual(
            recorded["deployment_contract"]["storage"]["serve_schemas"], [2]
        )
        self.assertTrue(recorded["deployment_contract"]["storage"]["explicit_upgrade"])

    def runtime(self):
        from contextlib import ExitStack

        stack = ExitStack()
        for name in ("launchctl", "await_health"):
            stack.enter_context(patch.object(controller, name))
        for name in ("service_plist_valid", "healthy"):
            stack.enter_context(patch.object(controller, name, return_value=True))
        return stack

    def test_unknown_existing_installation_is_preserved_without_starting(self):
        self.prepare(self.old)
        (self.root / "data").mkdir()
        (self.root / "data/state").write_text("unknown installation")
        (self.root / "master.key").write_text("unknown key")
        with self.runtime(), patch.object(controller, "launchctl") as launch:
            with self.assertRaisesRegex(RuntimeError, "unrecorded"):
                controller.activate(self.root, self.plan)
        launch.assert_not_called()
        self.assertEqual((self.root / "data/state").read_text(), "unknown installation")

    def test_prepare_same_sha_checks_health_instead_of_claiming_success(self):
        self.install_previous_release()
        with patch.object(controller, "healthy", return_value=False):
            with self.assertRaisesRegex(RuntimeError, "not healthy"):
                self.prepare(self.old)

    def test_crash_after_upgrade_recovers_original_snapshot_on_next_activation(self):
        self.install_previous_release()
        sha, plan = self.prepare_schema_two("schema2")

        class Crash(BaseException):
            pass

        real = controller.command_json

        def crash(*args, **kwargs):
            result = real(*args, **kwargs)
            if "upgrade" in args:
                raise Crash()
            return result

        with (
            self.runtime(),
            patch.object(controller, "command_json", side_effect=crash),
        ):
            with self.assertRaises(Crash):
                controller.activate(self.root, plan)
        self.assertEqual((self.root / "data/state").read_text(), "schema2")
        backups = {p.name: p.read_bytes() for p in (self.root / "backups").iterdir()}
        with self.runtime():
            with self.assertRaisesRegex(RuntimeError, "interrupted.*recovered"):
                controller.activate(self.root, plan)
        self.assertEqual(
            (self.root / "data/state").read_text(), "valuable schema1 data"
        )
        self.assertEqual(controller.current_sha(self.root), self.old)
        self.assertEqual(
            backups, {p.name: p.read_bytes() for p in (self.root / "backups").iterdir()}
        )
        with self.runtime():
            controller.activate(self.root, plan)
        self.assertEqual(controller.current_sha(self.root), sha)

    def test_crash_during_restore_resumes_without_overwriting_restored_data(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("migration fails")

        class Crash(BaseException):
            pass

        original = controller.save_json

        def crash(path, value):
            original(path, value)
            if path.name == "activation.json" and value.get("stage") == "restore_ready":
                raise Crash()

        with self.runtime(), patch.object(controller, "save_json", side_effect=crash):
            with self.assertRaises(Crash):
                controller.activate(self.root, plan)
        with self.runtime():
            with self.assertRaisesRegex(RuntimeError, "interrupted.*recovered"):
                controller.activate(self.root, plan)
        self.assertEqual(
            (self.root / "data/state").read_text(), "valuable schema1 data"
        )
        self.assertEqual(controller.current_sha(self.root), self.old)
        failed = list((self.root / "failed-data").glob("*/data/state"))
        self.assertEqual([p.read_text() for p in failed], ["schema2"])

    def test_bounded_product_timeout_terminates_descendants(self):
        marker = self.base / "late-write"
        script = self.base / "sleeping-command.py"
        script.write_text(
            "import subprocess,sys,time\nsubprocess.Popen([sys.executable,'-c',"
            + repr(
                "import time,pathlib;time.sleep(0.8);pathlib.Path("
                + repr(str(marker))
                + ").write_text('late')"
            )
            + "])\ntime.sleep(20)\n"
        )
        with self.assertRaises(RuntimeError):
            controller.command_json(sys.executable, str(script), timeout=0.1)
        time.sleep(1)
        self.assertFalse(marker.exists())

    def test_contract_or_binary_drift_rejected_before_stop(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("schema2")
        value = json.loads(plan.read_text())
        value["deployment_contract"]["storage"]["init_schema"] = 1
        plan.write_text(json.dumps(value))
        with self.runtime(), patch.object(controller, "launchctl") as launch:
            with self.assertRaises(ValueError):
                controller.activate(self.root, plan)
        launch.assert_not_called()

    def test_new_contractless_release_is_rejected(self):
        (self.work / "service.py").write_text(
            SCRIPT.replace(
                "elif args[1] == 'deployment-contract':",
                "elif args[1] == 'not-supported':",
            )
        )
        sha = self.commit("contract missing")
        with self.assertRaises(ValueError):
            self.prepare(sha)

    def test_parent_death_keeps_deployment_locked_until_child_timeout(self):
        pidfile = self.base / "product-started"
        marker = self.base / "late-write"
        script = self.base / "long-product.py"
        script.write_text(
            "import pathlib,time\npathlib.Path("
            + repr(str(pidfile))
            + ").touch()\ntime.sleep(5)\npathlib.Path("
            + repr(str(marker))
            + ").touch()\n"
        )
        parent = self.base / "controller-parent.py"
        parent.write_text(
            "import importlib.util,pathlib,sys\ns=importlib.util.spec_from_file_location('c',"
            + repr(str(SOURCE))
            + ")\nc=importlib.util.module_from_spec(s);s.loader.exec_module(c)\nwith c.locked(pathlib.Path("
            + repr(str(self.root))
            + ")):\n    c.command_json(sys.executable,"
            + repr(str(script))
            + ",timeout=1)\n"
        )
        process = subprocess.Popen(
            [sys.executable, str(parent)],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        self.addCleanup(lambda: process.poll() is None and process.kill())
        deadline = time.monotonic() + 5
        while not pidfile.exists() and time.monotonic() < deadline:
            time.sleep(0.01)
        self.assertTrue(pidfile.exists())
        process.kill()
        process.wait(timeout=3)
        with (self.root / "deploy.lock").open("a") as stream:
            with self.assertRaises(BlockingIOError):
                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
            deadline = time.monotonic() + 6
            while True:
                try:
                    fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
                    break
                except BlockingIOError:
                    if time.monotonic() >= deadline:
                        self.fail("supervisor did not release lock after deadline")
                    time.sleep(0.02)
        self.assertFalse(marker.exists())

    def test_partial_failed_restore_is_preserved_and_retry_recovers_same_snapshot(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("migration fails")
        real = controller.product_command

        def fail_restore(*args, **kwargs):
            if "restore" in args:
                target = Path(args[args.index("--data-dir") + 1])
                target.mkdir()
                (target / "partial").write_text("not validated")
                raise RuntimeError("restore failed")
            return real(*args, **kwargs)

        with (
            self.runtime(),
            patch.object(controller, "product_command", side_effect=fail_restore),
        ):
            with self.assertRaisesRegex(RuntimeError, "recovery incomplete"):
                controller.activate(self.root, plan)
        self.assertFalse((self.root / "current").exists())
        self.assertFalse((self.root / "data").exists())
        with self.runtime():
            with self.assertRaisesRegex(RuntimeError, "interrupted.*recovered"):
                controller.activate(self.root, plan)
        self.assertEqual(
            (self.root / "data/state").read_text(), "valuable schema1 data"
        )
        partials = list((self.root / "failed-data").glob("*/partial-restore-*/partial"))
        self.assertEqual([p.read_text() for p in partials], ["not validated"])

    def test_damaged_rollback_snapshot_stops_recovery_before_touching_data(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("schema2")

        class Crash(BaseException):
            pass

        real = controller.command_json

        def crash(*args, **kwargs):
            result = real(*args, **kwargs)
            if "upgrade" in args:
                raise Crash()
            return result

        with (
            self.runtime(),
            patch.object(controller, "command_json", side_effect=crash),
        ):
            with self.assertRaises(Crash):
                controller.activate(self.root, plan)
        next((self.root / "backups").glob("*-rollback.backup")).write_text("damaged")
        with self.runtime(), patch.object(controller, "launchctl") as launch:
            with self.assertRaisesRegex(RuntimeError, "recovery incomplete"):
                controller.activate(self.root, plan)
        launch.assert_not_called()
        self.assertEqual((self.root / "data/state").read_text(), "schema2")

    def test_invalid_upgrade_result_restores_original_before_old_start(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("schema2")
        real = controller.command_json

        def bad_result(*args, **kwargs):
            value = real(*args, **kwargs)
            if "upgrade" in args:
                value["schema"] = 1
            return value

        with (
            self.runtime(),
            patch.object(controller, "command_json", side_effect=bad_result),
        ):
            with self.assertRaisesRegex(ValueError, "invalid upgrade result"):
                controller.activate(self.root, plan)
        self.assertEqual(
            (self.root / "data/state").read_text(), "valuable schema1 data"
        )
        self.assertEqual(controller.current_sha(self.root), self.old)

    def test_health_failure_after_schema_upgrade_recovers_old_schema(self):
        self.install_previous_release()
        sha, plan = self.prepare_schema_two("schema2")

        def health(port, expected, version=None, **kwargs):
            if expected and version == sha:
                self.assertEqual(kwargs["schema"], 2)
                raise RuntimeError("wrong schema in actual health")

        with (
            self.runtime(),
            patch.object(controller, "await_health", side_effect=health),
        ):
            with self.assertRaisesRegex(RuntimeError, "wrong schema"):
                controller.activate(self.root, plan)
        self.assertEqual(
            (self.root / "data/state").read_text(), "valuable schema1 data"
        )
        self.assertEqual(controller.current_sha(self.root), self.old)

    def test_schema_downgrade_is_rejected_before_stop(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("schema2")
        with self.runtime():
            controller.activate(self.root, plan)
        (self.work / "service.py").write_text(SCRIPT)
        downgrade = self.commit("unsupported downgrade")
        self.prepare(downgrade)
        with self.runtime(), patch.object(controller, "launchctl") as launch:
            with self.assertRaisesRegex(ValueError, "cannot serve or upgrade"):
                controller.activate(self.root, self.plan)
        launch.assert_not_called()
        self.assertEqual((self.root / "data/state").read_text(), "schema2")

    def test_unchanged_legacy_deployment_can_upgrade_without_a_legacy_contract(self):
        self.install_previous_release()
        binary = controller.release_binary(self.root, self.old)
        binary.write_text(
            binary.read_text().replace(
                "elif args[1] == 'deployment-contract':",
                "elif args[1] == 'not-supported':",
            )
        )
        record = controller.read_json(self.root / "deployed.json")
        del record["deployment_contract"]
        del record["storage_schema"]
        record["binary_sha256"] = controller.digest(binary)
        controller.save_json(self.root / "deployed.json", record)
        sha, plan = self.prepare_schema_two("schema2")
        with self.runtime():
            controller.activate(self.root, plan)
        self.assertEqual(controller.current_sha(self.root), sha)
        self.assertEqual((self.root / "data/state").read_text(), "schema2")

    def test_interrupted_prepare_schedules_recovery_without_touching_data(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("schema2")

        class Crash(BaseException):
            pass

        real = controller.command_json

        def crash(*args, **kwargs):
            result = real(*args, **kwargs)
            if "upgrade" in args:
                raise Crash()
            return result

        with (
            self.runtime(),
            patch.object(controller, "command_json", side_effect=crash),
        ):
            with self.assertRaises(Crash):
                controller.activate(self.root, plan)
        newer = self.commit("new main while interrupted")
        output = self.base / "output"
        with (
            patch.object(controller, "stamp_binary") as build,
            patch.object(controller, "launchctl") as launch,
        ):
            controller.prepare(self.root, newer, self.plan, output)
        build.assert_not_called()
        launch.assert_not_called()
        self.assertEqual(output.read_text(), "deploy=true\n")
        self.assertEqual((self.root / "data/state").read_text(), "schema2")

    def test_first_install_failure_preserves_data_key_and_allows_fresh_retry(self):
        self.prepare(self.old)

        def unhealthy(port, expected, version=None, **kwargs):
            if expected:
                raise RuntimeError("initial start failed")

        with (
            self.runtime(),
            patch.object(controller, "await_health", side_effect=unhealthy),
        ):
            with self.assertRaisesRegex(RuntimeError, "initial start failed"):
                controller.activate(self.root, self.plan)
        self.assertFalse((self.root / "data").exists())
        self.assertFalse((self.root / "master.key").exists())
        failed = next((self.root / "failed-data").iterdir())
        self.assertEqual((failed / "master.key").read_text(), "key")
        self.assertEqual((failed / "data/state").read_text(), "new")
        with self.runtime():
            controller.activate(self.root, self.plan)
        self.assertEqual(controller.current_sha(self.root), self.old)

    def test_backup_interruption_restarts_original_without_trying_restore(self):
        self.install_previous_release()
        _, plan = self.prepare_schema_two("schema2")

        class Crash(BaseException):
            pass

        real = controller.product_command

        def crash(*args, **kwargs):
            if "backup" in args:
                raise Crash()
            return real(*args, **kwargs)

        with (
            self.runtime(),
            patch.object(controller, "product_command", side_effect=crash),
        ):
            with self.assertRaises(Crash):
                controller.activate(self.root, plan)
        with self.runtime(), patch.object(controller, "product_command") as product:
            with self.assertRaisesRegex(RuntimeError, "interrupted.*recovered"):
                controller.activate(self.root, plan)
        product.assert_not_called()
        self.assertEqual(
            (self.root / "data/state").read_text(), "valuable schema1 data"
        )
        self.assertEqual(controller.current_sha(self.root), self.old)

    def interrupted_recorded_candidate(self):
        self.install_previous_release()
        sha, plan = self.prepare_schema_two("schema2")

        class Crash(BaseException):
            pass

        real = controller.save_json

        def crash(path, value):
            real(path, value)
            if path.name == "deployed.json" and value.get("sha") == sha:
                raise Crash()

        with self.runtime(), patch.object(controller, "save_json", side_effect=crash):
            with self.assertRaises(Crash):
                controller.activate(self.root, plan)
        return sha, plan

    def test_crash_after_deployment_record_reconciles_without_rolling_back(self):
        sha, plan = self.interrupted_recorded_candidate()
        self.assertEqual(controller.current_sha(self.root), sha)
        self.assertEqual(
            controller.read_json(self.root / "activation.json")["stage"],
            "candidate_healthy",
        )
        (self.root / "data/state").write_text("new schema2 requests after publication")
        before = {p.name: p.read_bytes() for p in (self.root / "backups").iterdir()}
        with (
            self.runtime(),
            patch.object(controller, "launchctl") as launch,
            patch.object(controller, "product_command") as product,
        ):
            controller.activate(self.root, plan)
        launch.assert_not_called()
        product.assert_not_called()
        self.assertEqual(controller.current_sha(self.root), sha)
        self.assertEqual(
            (self.root / "data/state").read_text(),
            "new schema2 requests after publication",
        )
        self.assertEqual(
            controller.read_json(self.root / "activation.json")["stage"], "committed"
        )
        self.assertEqual(
            before, {p.name: p.read_bytes() for p in (self.root / "backups").iterdir()}
        )

    def test_supervisor_death_is_uncertain_and_never_safe_to_restore(self):
        pidfile = self.base / "running-product.json"
        script = self.base / "running-product.py"
        script.write_text(
            "import os,pathlib,json,time\npathlib.Path("
            + repr(str(pidfile))
            + ").write_text(json.dumps([os.getppid(),os.getpid()]))\ntime.sleep(30)\n"
        )
        captured = []

        def terminate_supervisor():
            deadline = time.monotonic() + 5
            while not pidfile.exists() and time.monotonic() < deadline:
                time.sleep(0.01)
            if pidfile.exists():
                supervisor, product = json.loads(pidfile.read_text())
                captured.append(product)
                os.kill(supervisor, signal.SIGKILL)

        killer = threading.Thread(target=terminate_supervisor)
        killer.start()
        try:
            with controller.locked(self.root):
                with self.assertRaises(controller.ProductStillRunning):
                    controller.product_command(sys.executable, str(script), timeout=5)
            killer.join(timeout=6)
            self.assertTrue(captured)
            # The product is still alive; ordinary recovery would be unsafe.
            os.kill(captured[0], 0)
        finally:
            killer.join(timeout=6)
            if captured:
                try:
                    os.killpg(captured[0], signal.SIGKILL)
                except ProcessLookupError:
                    pass

    def test_prepare_reconciles_healthy_recorded_candidate_without_activation(self):
        sha, plan = self.interrupted_recorded_candidate()
        output = self.base / "prepare-output"
        with (
            self.runtime(),
            patch.object(controller, "launchctl") as launch,
            patch.object(controller, "product_command") as product,
        ):
            controller.prepare(self.root, sha, plan, output)
        launch.assert_not_called()
        product.assert_not_called()
        self.assertEqual(output.read_text(), "deploy=false\n")
        self.assertEqual(
            controller.read_json(self.root / "activation.json")["stage"], "committed"
        )

    def test_unhealthy_recorded_candidate_is_recovered_instead_of_reconciled(self):
        _, plan = self.interrupted_recorded_candidate()
        with self.runtime(), patch.object(controller, "healthy", return_value=False):
            with self.assertRaisesRegex(RuntimeError, "interrupted.*recovered"):
                controller.activate(self.root, plan)
        self.assertEqual(controller.current_sha(self.root), self.old)
        self.assertEqual(
            (self.root / "data/state").read_text(), "valuable schema1 data"
        )


class ProductProtocolTest(unittest.TestCase):
    def test_product_output_is_bounded_before_json_decode(self):
        with self.assertRaisesRegex(RuntimeError, "failed"):
            controller.command_json(sys.executable, "-c", "print('x'*4097)")

    def test_health_requires_actual_version_and_integer_schema(self):
        payload = {"status": "ok", "version": "release", "storage_schema": 2}

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                raw = json.dumps(payload).encode()
                self.send_response(200)
                self.end_headers()
                self.wfile.write(raw)

            def log_message(self, *_):
                pass

        with HTTPServer(("127.0.0.1", 0), Handler) as server:
            thread = threading.Thread(target=server.serve_forever)
            thread.start()
            try:
                port = server.server_port
                self.assertTrue(controller.healthy(port, "release", 2))
                self.assertFalse(controller.healthy(port, "other", 2))
                self.assertFalse(controller.healthy(port, "release", 1))
                payload["storage_schema"] = True
                self.assertFalse(controller.healthy(port, "release", 1))
            finally:
                server.shutdown()
                thread.join()


if __name__ == "__main__":
    unittest.main()
