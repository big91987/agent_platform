import importlib.util
import json
import subprocess
import tempfile
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
