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

    def test_scan_retains_native_result_and_preserves_rules_on_reviewer_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "src").mkdir()
            (root / "src/main.go").write_text('package app\nimport _ "net/http"\n')
            (root / "go.mod").write_text("module example.org/app\n\ngo 1.25\n")
            (root / "architecture.json").write_text(
                json.dumps(
                    dict(
                        languages=["go"],
                        source_roots=["src"],
                        modules=[
                            dict(
                                name="app",
                                paths=["src/**"],
                                forbid_imports=["net/http"],
                            )
                        ],
                    )
                )
            )
            reviewer = root / "reviewer.py"
            reviewer.write_text(
                'import sys,json\nb=json.load(sys.stdin)\njson.dump(dict(snapshot=b["snapshot"],reviewed_files=sorted(b["sources"]),scope_notes=["Protocol fixture, not model quality evidence"],blockers=[],checks=[dict(name=c["name"],status="not_applicable",reason="Isolated transport fixture",evidence=[]) for c in b["review_checks"]],findings=[]),sys.stdout)\n'
            )
            cmd = [
                sys.executable,
                str(TOOL / "inspect_project.py"),
                "--root",
                str(root),
                "--review-command",
                json.dumps([sys.executable, str(reviewer)]),
            ]
            result = subprocess.run(cmd, capture_output=True, text=True)
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
            receipt = json.loads(result.stdout)
            report = json.loads((root / receipt["report"]).read_text())
            self.assertEqual(report["rules"]["status"], "violations")
            self.assertEqual(report["model_review"]["status"], "no_findings")
            folder = (root / receipt["report"]).parent
            self.assertTrue((folder / "review-input.json").exists())
            self.assertTrue((folder / "model-result.json").exists())
            self.assertIn("## 模型评审", (root / receipt["summary"]).read_text())
            reviewer.write_text("raise SystemExit(7)\n")
            failed = subprocess.run(cmd, capture_output=True, text=True)
            self.assertEqual(failed.returncode, 2, failed.stdout + failed.stderr)
            current = json.loads(
                (root / "harness-output/architecture/latest-report.json").read_text()
            )
            self.assertEqual(current["rules"]["status"], "violations")
            self.assertEqual(current["model_review"]["status"], "incomplete")
            self.assertTrue((folder / "model-result.json").exists())

    def test_root_scope_model_review_does_not_scan_its_own_artifacts(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "main.go").write_text("package app\n")
            (root / "go.mod").write_text("module example.org/app\n\ngo 1.25\n")
            (root / "architecture.json").write_text(
                json.dumps(
                    dict(
                        languages=["go"],
                        source_roots=["."],
                        modules=[dict(name="app", paths=["*.go"])],
                    )
                )
            )
            script = 'import sys,json;b=json.load(sys.stdin);json.dump(dict(snapshot=b["snapshot"],reviewed_files=sorted(b["sources"]),scope_notes=[],blockers=[],checks=[dict(name=c["name"],status="not_applicable",reason="Isolated command fixture",evidence=[]) for c in b["review_checks"]],findings=[]),sys.stdout)'
            command = [
                sys.executable,
                str(TOOL / "inspect_project.py"),
                "--root",
                str(root),
                "--review-command",
                json.dumps([sys.executable, "-c", script]),
            ]
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            report = json.loads(
                (root / json.loads(result.stdout)["report"]).read_text()
            )
            self.assertEqual(report["model_review"]["status"], "no_findings")
            custom = subprocess.run(
                command + ["--output", "evidence"], capture_output=True, text=True
            )
            self.assertEqual(custom.returncode, 2, custom.stdout + custom.stderr)
            failed_receipt = json.loads(custom.stdout)
            failed_report = json.loads((root / failed_receipt["report"]).read_text())
            self.assertIn("output", " ".join(failed_report["errors"]))
            self.assertFalse(
                (root / failed_receipt["report"])
                .parent.joinpath("model-result.json")
                .exists()
            )
