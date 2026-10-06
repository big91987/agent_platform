import hashlib
import json
import os
import plistlib
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SOURCE = Path(__file__).resolve().parents[1]


class ControllerInstallTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.root = self.base / "preview"
        self.root.mkdir(mode=0o700)
        self.home = self.base / "home"
        self.home.mkdir()
        subprocess.run(
            ["git", "init", str(self.root / "repository")],
            check=True,
            capture_output=True,
        )
        subprocess.run(
            [
                "git",
                "-C",
                str(self.root / "repository"),
                "remote",
                "add",
                "origin",
                "https://github.com/example/model-relay.git",
            ],
            check=True,
        )
        self.plist = (
            self.home
            / "Library/LaunchAgents/com.agent-platform.model-relay.example.model-relay.5678.plist"
        )

    def install(self):
        return subprocess.run(
            [
                sys.executable,
                str(SOURCE / "install.py"),
                "--repository",
                "example/model-relay",
                "--root",
                str(self.root),
                "--port",
                "5678",
            ],
            env={**os.environ, "HOME": str(self.home)},
            capture_output=True,
            text=True,
        )

    def test_new_and_existing_install_share_same_controller_and_preserve_product(self):
        result = self.install()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            (self.root / "controller.py").read_bytes(),
            (SOURCE / "controller.py").read_bytes(),
        )
        self.assertFalse((self.root / "data").exists())
        (self.root / "data").mkdir()
        (self.root / "data/state").write_text("existing installation")
        (self.root / "master.key").write_text("test-only-key")
        (self.root / "deployed.json").write_text('{"sha":"test release identity"}')
        before = {
            name: (self.root / name).read_bytes()
            for name in ("preview.json", "data/state", "master.key", "deployed.json")
        }
        plist = self.plist.read_bytes()
        result = self.install()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            before, {name: (self.root / name).read_bytes() for name in before}
        )
        self.assertEqual(self.plist.read_bytes(), plist)
        installed = json.loads((self.root / "controller-install.json").read_text())
        self.assertEqual(
            installed["controller_sha256"],
            hashlib.sha256((SOURCE / "controller.py").read_bytes()).hexdigest(),
        )

    def test_changed_launch_agent_is_rejected_before_overwriting_controller(self):
        (self.root / "controller.py").write_text("previous controller")
        self.plist.parent.mkdir(parents=True)
        self.plist.write_bytes(plistlib.dumps({"Label": "operator changed"}))
        result = self.install()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(
            (self.root / "controller.py").read_text(), "previous controller"
        )

    def test_pending_activation_requires_recovery_before_controller_replacement(self):
        (self.root / "controller.py").write_text("previous controller")
        (self.root / "activation.json").write_text('{"stage":"backup_complete"}')
        result = self.install()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("recover", result.stderr)
        self.assertEqual(
            (self.root / "controller.py").read_text(), "previous controller"
        )


if __name__ == "__main__":
    unittest.main()
