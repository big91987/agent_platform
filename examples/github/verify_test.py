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


if __name__ == "__main__":
    unittest.main()
