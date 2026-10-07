import json
import subprocess
import tempfile
import unittest
from pathlib import Path

from install_github_entry import install


class GitHubInstallTest(unittest.TestCase):
    def test_install_reentry_and_drift_rejection(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(root),
                    "remote",
                    "add",
                    "origin",
                    "https://github.com/owner/repo.git",
                ],
                check=True,
            )
            self.assertTrue(install(root, "owner/repo"))
            self.assertFalse(install(root, "owner/repo"))
            manifest = json.loads(
                (root / ".github/agent-platform-entry-source.json").read_text()
            )
            target = root / manifest["path"]
            target.write_text(target.read_text() + "# local change\n")
            with self.assertRaisesRegex(ValueError, "drift"):
                install(root, "owner/repo", upgrade=True)
            with self.assertRaises(ValueError):
                install(root, "other/repo")

    def test_interrupted_first_install_resumes_without_manual_manifest(self):
        from unittest.mock import patch

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(root),
                    "remote",
                    "add",
                    "origin",
                    "https://github.com/owner/repo.git",
                ],
                check=True,
            )
            from install_github_entry import atomic_write

            original = atomic_write

            def broken(path, *args, **kwargs):
                if path.name == "agent-platform-entry-source.json":
                    raise OSError("interrupted manifest write")
                return original(path, *args, **kwargs)

            with (
                patch("install_github_entry.atomic_write", broken),
                self.assertRaises(OSError),
            ):
                install(root, "owner/repo")
            install(root, "owner/repo")
            self.assertTrue(
                (root / ".github/agent-platform-entry-source.json").exists()
            )

    def test_interrupted_upgrade_resumes_and_preserves_unrelated_files(self):
        from unittest.mock import patch

        from install_github_entry import atomic_write, digest

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(root),
                    "remote",
                    "add",
                    "origin",
                    "https://github.com/owner/repo.git",
                ],
                check=True,
            )
            install(root, "owner/repo")
            target = root / ".github/workflows/agent-platform-entry.yml"
            manifest = root / ".github/agent-platform-entry-source.json"
            old = b"name: Previous installed source\n"
            target.write_bytes(old)
            manifest.write_text(
                json.dumps(
                    {
                        "path": ".github/workflows/agent-platform-entry.yml",
                        "sha256": digest(old),
                    }
                )
            )
            (root / "keep.txt").write_text("keep")

            def broken(path, content):
                if path == manifest:
                    raise OSError("interrupted")
                atomic_write(path, content)

            with (
                patch("install_github_entry.atomic_write", broken),
                self.assertRaises(OSError),
            ):
                install(root, "owner/repo", True)
            self.assertTrue(install(root, "owner/repo", True))
            self.assertFalse(install(root, "owner/repo", True))
            self.assertEqual((root / "keep.txt").read_text(), "keep")
