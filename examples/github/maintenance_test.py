import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import maintenance


class MaintenanceTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.work = self.root / "work"
        self.work.mkdir()
        self.git("init", "-b", "main")
        self.git("config", "user.name", "Test")
        self.git("config", "user.email", "test@example.invalid")
        (self.work / "app.txt").write_text("product")
        self.git("add", ".")
        self.git("commit", "-m", "base")
        self.base = self.git("rev-parse", "HEAD")
        (self.work / "scripts").mkdir()
        (self.work / "scripts/check.py").write_text("print('checked')")
        self.git("add", ".")
        self.git("commit", "-m", "maintenance")
        self.head = self.git("rev-parse", "HEAD")
        self.settings = {
            "repository": "owner/repo",
            "registry": str(self.root / "registry"),
            "maintenance": {
                "paths": ["scripts/**"],
                "checks": [[sys.executable, "scripts/check.py"]],
            },
        }
        self.pr = {
            "number": 2,
            "state": "open",
            "draft": False,
            "base": {"ref": "main"},
            "head": {"sha": self.head, "repo": {"full_name": "owner/repo"}},
        }

    def git(self, *args):
        return subprocess.check_output(
            ["git", "-C", str(self.work), *args], text=True, stderr=subprocess.DEVNULL
        ).strip()

    def api(self, path, body=None):
        if path.endswith("/pulls/2"):
            return self.pr
        if path.endswith("/commits/main"):
            return {"sha": self.base}
        if "/statuses/" in path:
            self.statuses.append(body)
            return {}
        raise AssertionError(path)

    def verify(self):
        self.statuses = []
        with patch.object(maintenance, "github", side_effect=self.api):
            return maintenance.verify(self.settings, 2, self.work)

    def test_real_check_produces_receipt_and_is_invalidated_by_base_or_head_change(
        self,
    ):
        self.verify()
        self.assertEqual(self.statuses[-1]["state"], "success")
        self.assertTrue(maintenance.verified(self.settings, self.pr, self.base))
        self.assertFalse(maintenance.verified(self.settings, self.pr, "different"))
        self.pr["head"]["sha"] = "different"
        self.assertFalse(maintenance.verified(self.settings, self.pr, self.base))

    def test_failed_command_never_passes_and_has_durable_log(self):
        self.settings["maintenance"]["checks"] = [
            [sys.executable, "-c", "raise SystemExit(7)"]
        ]
        with self.assertRaises(subprocess.CalledProcessError):
            self.verify()
        self.assertEqual(self.statuses[-1]["state"], "failure")
        self.assertFalse(maintenance.verified(self.settings, self.pr, self.base))
        self.assertTrue(list((self.root / "registry/maintenance").rglob("*.log")))

    def test_product_edit_and_dirty_worktree_are_rejected(self):
        (self.work / "app.txt").write_text("changed")
        with self.assertRaises(ValueError):
            self.verify()
        self.git("add", ".")
        self.git("commit", "-m", "product")
        self.pr["head"]["sha"] = self.git("rev-parse", "HEAD")
        with self.assertRaises(ValueError):
            self.verify()
        self.assertFalse(maintenance.verified(self.settings, self.pr, self.base))

    def test_main_moves_while_tests_run_no_success(self):
        self.statuses = []
        calls = 0

        def api(path, body=None):
            nonlocal calls
            if path.endswith("/commits/main"):
                calls += 1
                return {"sha": self.base if calls == 1 else "new-main"}
            return self.api(path, body)

        with patch.object(maintenance, "github", side_effect=api):
            with self.assertRaises(ValueError):
                maintenance.verify(self.settings, 2, self.work)
        self.assertEqual(self.statuses[-1]["state"], "failure")

    def test_no_checks_or_outdated_main_cannot_pass(self):
        self.settings["maintenance"]["checks"] = []
        with self.assertRaises(ValueError):
            self.verify()
        self.settings["maintenance"]["checks"] = [[sys.executable, "scripts/check.py"]]
        self.base = "0" * 40
        with self.assertRaises((ValueError, subprocess.CalledProcessError)):
            self.verify()

    def test_changed_policy_invalidates_success(self):
        self.verify()
        self.settings["maintenance"]["paths"].append("tests/**")
        self.assertFalse(maintenance.verified(self.settings, self.pr, self.base))
