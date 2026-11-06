import fcntl
import json
import shutil
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
            self.assertIn(
                report["rules"]["findings"][0]["message"],
                (root / receipt["summary"]).read_text(),
            )
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

    def test_snapshot_directory_does_not_claim_unrelated_parent_git_head(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            subprocess.run(["git", "init", "-q", str(parent)], check=True)
            (parent / "unrelated.txt").write_text("different repository")
            subprocess.run(["git", "add", "unrelated.txt"], cwd=parent, check=True)
            subprocess.run(
                [
                    "git",
                    "-c",
                    "user.name=Fixture",
                    "-c",
                    "user.email=fixture@example.invalid",
                    "commit",
                    "-qm",
                    "fixture",
                ],
                cwd=parent,
                check=True,
            )
            root = parent / "snapshot"
            root.mkdir()
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
            result = subprocess.run(
                [sys.executable, str(TOOL / "inspect_project.py"), "--root", str(root)],
                capture_output=True,
                text=True,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            report = json.loads(
                (root / json.loads(result.stdout)["report"]).read_text()
            )
            self.assertIsNone(report["git_head"])

    def test_model_finding_is_presented_with_real_parser_symbol_and_untouched_text(
        self,
    ):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "src").mkdir()
            (root / "src/http.go").write_text(
                'package app\nfunc Handle() {\n println("storage detail")\n}\n'
            )
            (root / "go.mod").write_text("module example.org/app\n\ngo 1.25\n")
            (root / "architecture.json").write_text(
                json.dumps(
                    {
                        "languages": ["go"],
                        "source_roots": ["src"],
                        "modules": [{"name": "app", "paths": ["src/**"]}],
                    }
                )
            )
            evidence = {"path": "src/http.go", "quote": 'println("storage detail")'}
            finding = {
                "checks": ["module_responsibilities"],
                "title": "Fixture architecture observation",
                "severity": "medium",
                "principle": "module_responsibilities",
                "impact": "Boundary coupling",
                "recommendation": "Move storage detail out of Handle.",
                "validation": "Recheck imports at the module boundary.",
                "evidence": [evidence],
            }
            script = (
                "import sys,json; b=json.load(sys.stdin); finding="
                + repr(finding)
                + '; json.dump(dict(snapshot=b["snapshot"], reviewed_files=sorted(b["sources"]),'
                'scope_notes=["Controlled protocol fixture, not model-quality evidence"], blockers=[],'
                'checks=[dict(name=c["name"],status="deviation" if i==0 else "not_applicable",'
                'reason="Fixture only",evidence=finding["evidence"] if i==0 else []) '
                'for i,c in enumerate(b["review_checks"])],findings=[finding]),sys.stdout)'
            )
            result = subprocess.run(
                [
                    sys.executable,
                    str(TOOL / "inspect_project.py"),
                    "--root",
                    str(root),
                    "--review-command",
                    json.dumps([sys.executable, "-c", script]),
                ],
                capture_output=True,
                text=True,
            )
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
            receipt = json.loads(result.stdout)
            summary = (root / receipt["summary"]).read_text()
            self.assertIn("src/http.go:3", summary)
            self.assertIn("`Handle`", summary)
            self.assertIn(finding["recommendation"], summary)
            raw = json.loads(
                (root / receipt["report"])
                .parent.joinpath("model-result.json")
                .read_text()
            )
            self.assertEqual(raw["findings"][0], finding)

    def test_isolated_entry_preserves_incomplete_receipt_during_tool_upgrade_lock(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            installed = root / ".harness/archcheck"
            shutil.copytree(
                TOOL,
                installed,
                ignore=shutil.ignore_patterns("node_modules", "__pycache__"),
            )
            manifest = installed / "SOURCE.json"
            manifest.write_text('{"version":"fixture"}')
            with manifest.open("rb") as lock:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                result = subprocess.run(
                    [
                        sys.executable,
                        "-I",
                        str(installed / "inspect_project.py"),
                        "--root",
                        str(root),
                    ],
                    capture_output=True,
                    text=True,
                )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            receipt = json.loads(result.stdout)
            self.assertEqual(receipt["status"], "incomplete")
            report = json.loads((root / receipt["report"]).read_text())
            self.assertTrue(report["errors"])
            self.assertIn("规则检查未完成", (root / receipt["summary"]).read_text())
            latest = root / "harness-output/architecture/latest-report.json"
            self.assertEqual(json.loads(latest.read_text()), report)
