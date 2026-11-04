import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOL = Path(__file__).resolve().parents[1]


class InspectionTests(unittest.TestCase):
    def test_real_parser_keeps_each_check_and_distinguishes_invalid_configuration(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "go.mod").write_text("module example.org/app\n\ngo 1.25\n")
            (root / "src").mkdir()
            source = root / "src/main.go"
            source.write_text("package app\n")
            config = {
                "languages": ["go"],
                "source_roots": ["src"],
                "modules": [
                    {"name": "app", "paths": ["src/**"], "forbid_imports": ["net/http"]}
                ],
            }
            (root / "architecture.json").write_text(json.dumps(config))
            command = [
                sys.executable,
                str(TOOL / "inspect_project.py"),
                "--root",
                str(root),
            ]
            first = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(first.returncode, 0, first.stderr + first.stdout)
            first_summary = json.loads(first.stdout)
            source.write_text('package app\nimport _ "net/http"\n')
            second = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(second.returncode, 1, second.stderr + second.stdout)
            second_summary = json.loads(second.stdout)
            self.assertNotEqual(first_summary["report"], second_summary["report"])
            report = json.loads((root / second_summary["report"]).read_text())
            self.assertEqual(report["status"], "violations")
            self.assertTrue(report["snapshot"])
            self.assertIn("src/main.go", report["hashes"])
            self.assertIn("net/http", (root / second_summary["summary"]).read_text())
            (root / "architecture.json").unlink()
            failed = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(failed.returncode, 2)
            self.assertEqual(json.loads(failed.stdout)["status"], "incomplete")
            latest = json.loads(
                (root / "harness-output/architecture/latest-report.json").read_text()
            )
            self.assertEqual(latest["status"], "incomplete")
            self.assertTrue((root / first_summary["report"]).exists())
