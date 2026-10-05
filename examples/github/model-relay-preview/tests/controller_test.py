import importlib.util
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

SOURCE = Path(__file__).resolve().parents[1] / "controller.py"
SPEC = importlib.util.spec_from_file_location("model_relay_preview", SOURCE)
controller = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(controller)


def command(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


SCRIPT = """#!/usr/bin/env python3
import pathlib, sys
args = sys.argv
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

    def test_prepare_authenticates_fetch_with_job_token(self):
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
        with patch.dict(
            os.environ,
            {
                "MODEL_RELAY_REPO_TOKEN": "test-token",
                "PATH": str(fakebin) + os.pathsep + os.environ["PATH"],
            },
        ):
            self.prepare(self.old)
        self.assertEqual(json.loads(self.plan.read_text())["sha"], self.old)

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
        self.prepare(self.old)
        (self.root / "data").mkdir()
        (self.root / "data/state").write_text("valuable")
        (self.root / "master.key").write_text("key")
        with (
            patch.object(controller, "service_plist_valid", return_value=True),
            patch.object(controller, "launchctl"),
            patch.object(controller, "await_health"),
            patch.object(controller, "healthy", return_value=True),
        ):
            (self.root / "service.plist").write_text("installed")
            controller.activate(self.root, self.plan)
        self.assertEqual(controller.current_sha(self.root), self.old)
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


if __name__ == "__main__":
    unittest.main()
