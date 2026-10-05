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
    def test_new_install_reentry_and_drift_rejection(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary)
            subprocess.run(
                ["git", "init", str(project)], check=True, capture_output=True
            )
            self.assertTrue(installer.install(project))
            installed = (project / installer.WORKFLOW).read_bytes()
            manifest = json.loads((project / installer.MANIFEST).read_text())
            self.assertEqual(manifest["sha256"], installer.digest(installed))
            self.assertFalse(installer.install(project))
            (project / installer.WORKFLOW).write_bytes(installed + b"# local edit\n")
            with self.assertRaisesRegex(ValueError, "outside installer"):
                installer.install(project, upgrade=True)


if __name__ == "__main__":
    unittest.main()
