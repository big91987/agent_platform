"""Real Git transport fixtures; no model calls or product acceptance."""

import hashlib
import json
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch

from git_workspace import Workspace, git
from publish import publish


class PublishTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "source"
        self.repo.mkdir()
        self.branch = "agent-team/experiment"
        git(self.repo, "init", "-b", self.branch)
        git(self.repo, "config", "user.name", "Transport Fixture")
        git(self.repo, "config", "user.email", "fixture@example.invalid")
        (self.repo / "base.txt").write_text("transport baseline")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-m", "baseline")
        self.base = git(self.repo, "rev-parse", "HEAD")
        git(self.repo, "branch", "main")
        self.remote = self.root / "remote.git"
        subprocess.run(
            ["git", "clone", "--bare", str(self.repo), str(self.remote)],
            check=True,
            capture_output=True,
        )
        git(self.repo, "remote", "add", "origin", str(self.remote))
        git(self.repo, "fetch", "origin")
        (self.repo / "work.txt").write_text("real transport fixture change")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-m", "candidate")
        self.head = git(self.repo, "rev-parse", "HEAD")
        self.workspace = Workspace(self.repo, self.root / "runtime", self.branch)

    def test_incremental_bundle_publishes_exact_commit_and_reconciles_repeat(self):
        bundle = self.workspace.bundle(self.head)
        self.assertEqual(
            bundle["sha256"],
            hashlib.sha256(Path(bundle["bundle"]).read_bytes()).hexdigest(),
        )
        result = publish(
            bundle["bundle"], bundle["sha256"], str(self.remote), self.branch, self.head
        )
        self.assertEqual(result["status"], "published")
        self.assertEqual(git(self.remote, "rev-parse", self.branch), self.head)
        self.assertEqual(git(self.remote, "rev-parse", "main"), self.base)
        again = publish(
            bundle["bundle"], bundle["sha256"], str(self.remote), self.branch, self.head
        )
        self.assertEqual(again["status"], "already_published")
        self.assertNotEqual(result["receipt"], again["receipt"])

    def test_wrong_digest_or_head_or_main_target_cannot_change_remote(self):
        bundle = self.workspace.bundle(self.head)
        with self.assertRaises(ValueError):
            publish(
                bundle["bundle"], "0" * 64, str(self.remote), self.branch, self.head
            )
        with self.assertRaises(ValueError):
            publish(
                bundle["bundle"],
                bundle["sha256"],
                str(self.remote),
                self.branch,
                self.base,
            )
        with self.assertRaises(ValueError):
            publish(
                bundle["bundle"], bundle["sha256"], str(self.remote), "main", self.head
            )
        self.assertEqual(git(self.remote, "rev-parse", self.branch), self.base)

    def test_remote_divergence_is_preserved_without_force(self):
        bundle = self.workspace.bundle(self.head)
        other = self.root / "other"
        subprocess.run(
            ["git", "clone", str(self.remote), str(other)],
            check=True,
            capture_output=True,
        )
        git(other, "config", "user.name", "Other Fixture")
        git(other, "config", "user.email", "other@example.invalid")
        (other / "other.txt").write_text("independent remote update")
        git(other, "add", ".")
        git(other, "commit", "-m", "remote change")
        git(other, "push", "origin", self.branch)
        remote_head = git(other, "rev-parse", "HEAD")
        with self.assertRaises(RuntimeError):
            publish(
                bundle["bundle"],
                bundle["sha256"],
                str(self.remote),
                self.branch,
                self.head,
            )
        self.assertEqual(git(self.remote, "rev-parse", self.branch), remote_head)

    def test_replacing_input_after_validation_cannot_change_pushed_commit(self):
        bundle = self.workspace.bundle(self.head)
        (self.repo / "later.txt").write_text("not authorized by this publish request")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-m", "later candidate")
        later = self.workspace.bundle(git(self.repo, "rev-parse", "HEAD"))
        actual_run = subprocess.run

        def replace_after_check(argv, *args, **kwargs):
            result = actual_run(argv, *args, **kwargs)
            if "list-heads" in argv:
                Path(bundle["bundle"]).write_bytes(Path(later["bundle"]).read_bytes())
            return result

        with patch("publish.subprocess.run", side_effect=replace_after_check):
            result = publish(
                bundle["bundle"],
                bundle["sha256"],
                str(self.remote),
                self.branch,
                self.head,
            )
        self.assertEqual(result["status"], "published")
        self.assertEqual(git(self.remote, "rev-parse", self.branch), self.head)

    def test_lost_response_after_real_push_stays_unknown_and_reconciles(self):
        bundle = self.workspace.bundle(self.head)
        actual_run = subprocess.run

        def interrupt_after_push(argv, *args, **kwargs):
            result = actual_run(argv, *args, **kwargs)
            if "push" in argv:
                raise KeyboardInterrupt("response lost after remote accepted commit")
            return result

        with patch("publish.subprocess.run", side_effect=interrupt_after_push):
            with self.assertRaises(KeyboardInterrupt):
                publish(
                    bundle["bundle"],
                    bundle["sha256"],
                    str(self.remote),
                    self.branch,
                    self.head,
                )
        receipt = sorted(
            (Path(bundle["bundle"]).parent / "publish-receipts").glob("*.json")
        )[-1]
        self.assertEqual(json.loads(receipt.read_text())["status"], "unknown")
        self.assertEqual(git(self.remote, "rev-parse", self.branch), self.head)
        again = publish(
            bundle["bundle"], bundle["sha256"], str(self.remote), self.branch, self.head
        )
        self.assertEqual(again["status"], "already_published")
        self.assertFalse(any("push" in item["args"] for item in again["commands"]))

    def test_sigterm_after_remote_acceptance_keeps_pre_push_unknown_receipt(self):
        bundle = self.workspace.bundle(self.head)
        marker = self.root / "remote-accepted"
        script = """import pathlib,subprocess,time
from publish import publish
actual_run=subprocess.run
def observed(argv,*args,**kwargs):
 result=actual_run(argv,*args,**kwargs)
 if 'push' in argv:
  pathlib.Path(MARKER).write_text('remote accepted')
  time.sleep(30)
 return result
subprocess.run=observed
publish(BUNDLE,DIGEST,REMOTE,BRANCH,HEAD)
"""
        for key, value in {
            "MARKER": str(marker),
            "BUNDLE": bundle["bundle"],
            "DIGEST": bundle["sha256"],
            "REMOTE": str(self.remote),
            "BRANCH": self.branch,
            "HEAD": self.head,
        }.items():
            script = script.replace(key, repr(value))
        child = subprocess.Popen(
            [sys.executable, "-c", script],
            cwd=Path(__file__).parent,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        try:
            deadline = time.monotonic() + 8
            while (
                not marker.exists()
                and child.poll() is None
                and time.monotonic() < deadline
            ):
                time.sleep(0.02)
            self.assertTrue(marker.exists(), "real push did not reach acceptance")
            child.terminate()
            child.communicate(timeout=3)
            receipt = sorted(
                (Path(bundle["bundle"]).parent / "publish-receipts").glob("*.json")
            )[-1]
            self.assertEqual(json.loads(receipt.read_text())["status"], "unknown")
            self.assertEqual(git(self.remote, "rev-parse", self.branch), self.head)
        finally:
            if child.poll() is None:
                child.kill()
                child.communicate(timeout=3)


if __name__ == "__main__":
    unittest.main()
