import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from verify import verify


class VerifyTest(unittest.TestCase):
    def test_failed_rerun_replaces_stale_green_receipt(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / "app").mkdir()
            (root / "tests/browser").mkdir(parents=True)
            (root / "tests/browser/core.json").write_text('[{"action":"reload"}]')
            evidence = root / "docs/05-validation/tasks/1/delivery-checks"
            evidence.mkdir(parents=True)
            (evidence.parent / "browser-plan.json").write_text("[]")
            (evidence / "checks.json").write_text('[{"exit_code":0}]')

            def failed(command, **kwargs):
                return subprocess.CompletedProcess(command, 1, "", "actual failure")

            with patch("verify.subprocess.run", side_effect=failed):
                with self.assertRaisesRegex(RuntimeError, "actual failure"):
                    verify({"checkout": d, "python": "python3"}, root, 1)
            self.assertEqual(
                json.loads((evidence / "checks.json").read_text())[0]["exit_code"], 1
            )


class BackendGateTest(unittest.TestCase):
    def test_python_backend_failure_blocks_even_without_python_in_app(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for name in (
                "tests/browser/core.json",
                "docs/05-validation/tasks/1/browser-plan.json",
            ):
                p = root / name
                p.parent.mkdir(parents=True, exist_ok=True)
                p.write_text('[{"action":"screenshot"}]')
            (root / "scripts").mkdir()
            (root / "scripts/backend.py").write_text("bad_python_formatting = True")
            config = {
                "python": "python3",
                "checkout": str(root),
                "extra_product_files": ["scripts/backend.py"],
            }

            def command(argv, **kwargs):
                failed = any(
                    str(arg).endswith("full_harness/quality.py") for arg in argv
                )
                return subprocess.CompletedProcess(
                    argv, int(failed), "backend lint failed" if failed else "", ""
                )

            with patch("verify.subprocess.run", side_effect=command):
                with self.assertRaisesRegex(RuntimeError, "backend lint failed"):
                    verify(config, root, 1)


if __name__ == "__main__":
    unittest.main()
