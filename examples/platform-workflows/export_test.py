"""A copied Harness runs without the maintenance checkout or ambient imports."""

import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from export import export
from repository import check_publish_files


class ExportTest(unittest.TestCase):
    def test_fixed_version_is_self_contained_and_preserves_project_customizations(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "source"
            source.mkdir()
            dest = root / "project/.harness/agent-platform"
            real = Path(__file__).resolve().parents[2]
            paths = [
                "go.mod",
                "go.sum",
                "examples/pipeline-tool/main.go",
                "sdk/python/agent_platform_client/__init__.py",
                "examples/platform-workflows/run_entry.py",
                "examples/platform-workflows/sandbox_entry.py",
                "examples/platform-workflows/github_entry.py",
                "examples/platform-workflows/materials.py",
                "examples/platform-workflows/report_publication.py",
                "examples/platform-workflows/report_publication_test.py",
                "examples/github/requirements.py",
            ]
            for name in paths:
                target = source / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(real / name, target)
            subprocess.run(["git", "init", "-q", str(source)], check=True)
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(source),
                    "add",
                    "sdk",
                    "examples",
                    "go.mod",
                    "go.sum",
                ],
                check=True,
            )
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(source),
                    "-c",
                    "user.name=Fixture",
                    "-c",
                    "user.email=fixture@example.invalid",
                    "commit",
                    "-qm",
                    "source version",
                ],
                check=True,
            )
            (source / "private-auth.json").write_text("fixture private data")
            commit = export(source, dest)
            inventory = json.loads((dest / "SOURCE.json").read_text())
            self.assertEqual(inventory["commit"], commit)
            self.assertTrue(
                (dest / "go.mod").is_file(),
                "copied Go MCP example lacks its dependency contract",
            )
            self.assertFalse((dest / "private-auth.json").exists())
            for name, record in inventory["files"].items():
                self.assertEqual(
                    hashlib.sha256((dest / name).read_bytes()).hexdigest(),
                    record["sha256"],
                )
            # Installing the supplied template must not block the project's own
            # publication scanner; an actual private artifact still must block it.
            project = root / "project"
            subprocess.run(["git", "init", "-q", str(project)], check=True)
            subprocess.run(["git", "-C", str(project), "add", ".harness"], check=True)
            check_publish_files(project)
            (project / ".env").write_text("private fixture")
            with self.assertRaisesRegex(ValueError, "credentials/cache"):
                check_publish_files(project)
            (project / ".env").unlink()
            shutil.rmtree(source)
            env = {
                k: v
                for k, v in os.environ.items()
                if not k.startswith(("PYTHON", "AGENT_PLATFORM"))
            }
            for mode in ["sandbox", "legacy"]:
                result = subprocess.run(
                    [
                        sys.executable,
                        "-I",
                        str(dest / "examples/platform-workflows/run_entry.py"),
                        mode,
                        "--help",
                    ],
                    env=env,
                    text=True,
                    capture_output=True,
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("--config", result.stdout)
            result = subprocess.run(
                [
                    "go",
                    "build",
                    "-o",
                    str(root / "copied-mcp-tool"),
                    "./examples/pipeline-tool",
                ],
                cwd=dest,
                env=env,
                text=True,
                capture_output=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            (dest / "project-customization.txt").write_text("keep this")
            with self.assertRaises(ValueError):
                export(real, dest)
            self.assertEqual(
                (dest / "project-customization.txt").read_text(), "keep this"
            )


if __name__ == "__main__":
    unittest.main()
