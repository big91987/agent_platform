import importlib.util
import json
import os
import re
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path

SOURCE = Path(__file__).resolve().parents[1] / "install_workflow.py"
SPEC = importlib.util.spec_from_file_location("model_relay_workflow_install", SOURCE)
installer = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(installer)


class WorkflowInstallTest(unittest.TestCase):
    def test_deploy_requires_owner_manual_dispatch(self):
        workflow = Path(__file__).resolve().parents[1] / "deploy-local.yml"
        content = workflow.read_text()
        self.assertIn("push:\n    branches: [main]", content)
        self.assertIn("type: boolean\n        default: false", content)
        self.assertIn(
            "github.event_name == 'workflow_dispatch' && github.actor == github.repository_owner && inputs.deploy",
            content,
        )

    def test_actual_job_shell_keeps_target_and_isolation_arguments(self):
        content = SOURCE.with_name("deploy-local.yml").read_text()
        scripts = [
            textwrap.dedent(value)
            for value in re.findall(
                r"        run: \|\n((?:          .*\n)+)",
                content,
            )
        ]
        self.assertEqual(len(scripts), 2)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "validation root"
            root.mkdir()
            receipt = root / "args.json"
            (root / "controller.py").write_text(
                "import json, pathlib, sys\n"
                "pathlib.Path(__file__).with_name('args.json').write_text(json.dumps(sys.argv[1:]))\n"
            )
            base_env = {
                **os.environ,
                "DEPLOY_ROOT": str(root),
                "PREVIEW_ROOT": str(Path(temporary) / "preview root"),
                "TARGET_SHA": "a" * 40,
                "GITHUB_RUN_ID": "123",
                "GITHUB_OUTPUT": str(root / "output"),
            }
            for script in scripts:
                for target in ("preview", "validation"):
                    result = subprocess.run(
                        ["bash", "-e", "-c", script],
                        env={**base_env, "DEPLOY_TARGET": target},
                        capture_output=True,
                    )
                    self.assertEqual(result.returncode, 0, result.stderr)
                    args = json.loads(receipt.read_text())
                    self.assertEqual(args[args.index("--root") + 1], str(root))
                    self.assertEqual("--protected-root" in args, target == "validation")
                    if target == "validation":
                        self.assertEqual(
                            args[args.index("--protected-root") + 1],
                            base_env["PREVIEW_ROOT"],
                        )
                    receipt.unlink()
                for overrides in (
                    {"DEPLOY_TARGET": "unknown"},
                    {"DEPLOY_TARGET": "validation", "PREVIEW_ROOT": ""},
                    {"DEPLOY_TARGET": "validation", "DEPLOY_ROOT": ""},
                ):
                    result = subprocess.run(
                        ["bash", "-e", "-c", script],
                        env={**base_env, **overrides},
                        capture_output=True,
                    )
                    self.assertNotEqual(result.returncode, 0)
                    self.assertFalse(receipt.exists())

    def project(self, temporary, remote="https://github.com/example/model-relay.git"):
        project = Path(temporary)
        subprocess.run(["git", "init", str(project)], check=True, capture_output=True)
        subprocess.run(
            ["git", "-C", str(project), "remote", "add", "origin", remote],
            check=True,
        )
        return project

    def test_new_install_reentry_and_drift_rejection(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = self.project(temporary)
            self.assertTrue(installer.install(project))
            installed = (project / installer.WORKFLOW).read_bytes()
            manifest = json.loads((project / installer.MANIFEST).read_text())
            self.assertEqual(manifest["sha256"], installer.digest(installed))
            self.assertFalse(installer.install(project))
            (project / installer.WORKFLOW).write_bytes(installed + b"# local edit\n")
            with self.assertRaisesRegex(ValueError, "outside installer"):
                installer.install(project, upgrade=True)

    def test_rejects_wrong_project_and_path_escape(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = self.project(temporary, "https://github.com/example/other.git")
            with self.assertRaisesRegex(ValueError, "Model Relay repository"):
                installer.install(project)
        with tempfile.TemporaryDirectory() as temporary:
            project = self.project(Path(temporary) / "project")
            outside = Path(temporary) / "outside-workflows"
            outside.mkdir()
            (project / ".github").mkdir()
            (project / ".github/workflows").symlink_to(outside)
            with self.assertRaisesRegex(ValueError, "escapes project"):
                installer.install(project)

    def test_upgrade_replaces_only_unmodified_installed_workflow(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = self.project(temporary)
            target = project / installer.WORKFLOW
            manifest = project / installer.MANIFEST
            target.parent.mkdir(parents=True)
            manifest.parent.mkdir(parents=True)
            old = b"name: previous controlled workflow\n"
            target.write_bytes(old)
            manifest.write_text(
                json.dumps(
                    {"path": str(installer.WORKFLOW), "sha256": installer.digest(old)}
                )
            )
            with self.assertRaisesRegex(ValueError, "requires --upgrade"):
                installer.install(project)
            self.assertTrue(installer.install(project, upgrade=True))
            self.assertEqual(
                json.loads(manifest.read_text())["sha256"],
                installer.digest(target.read_bytes()),
            )
            self.assertFalse(installer.install(project, upgrade=True))


if __name__ == "__main__":
    unittest.main()
