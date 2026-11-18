import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).parents[1] / "scripts"


class UpgradeTest(unittest.TestCase):
    def test_controller_upgrade_preserves_legacy_service_and_data(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "repository").mkdir()
            subprocess.run(["git", "init", "-q", str(root / "repository")], check=True)
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(root / "repository"),
                    "remote",
                    "add",
                    "origin",
                    "https://github.com/example/product.git",
                ],
                check=True,
            )
            (root / "controller").mkdir()
            (root / "controller/local_deploy.py").write_text(
                "# previous trusted controller"
            )
            (root / "data").mkdir()
            (root / "data/keep").write_text("retained")
            (root / "releases/old").mkdir(parents=True)
            (root / "current").symlink_to("releases/old")
            for _ in range(2):
                result = subprocess.run(
                    [
                        sys.executable,
                        str(SCRIPTS / "install_local_preview.py"),
                        "--root",
                        str(root),
                        "--repository",
                        "example/product",
                        "--port",
                        "5533",
                        "--controller-only",
                    ],
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual((root / "data/keep").read_text(), "retained")
                self.assertEqual((root / "current").readlink(), Path("releases/old"))
                self.assertEqual(
                    (root / "controller/local_deploy.py").read_bytes(),
                    (SCRIPTS / "local_deploy.py").read_bytes(),
                )
            version = json.loads((root / "controller/version.json").read_text())
            self.assertEqual(
                version["sha256"],
                hashlib.sha256((SCRIPTS / "local_deploy.py").read_bytes()).hexdigest(),
            )
            self.assertTrue(
                any(
                    p.read_text() == "# previous trusted controller"
                    for p in (root / "controller/history").glob("*.py")
                )
            )
            mismatch = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPTS / "install_local_preview.py"),
                    "--root",
                    str(root),
                    "--repository",
                    "another/product",
                    "--controller-only",
                ],
                capture_output=True,
                text=True,
            )
            self.assertNotEqual(mismatch.returncode, 0)
            self.assertEqual((root / "data/keep").read_text(), "retained")
