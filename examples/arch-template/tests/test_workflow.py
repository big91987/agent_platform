import importlib.util
import json
import shlex
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class WorkflowTests(unittest.TestCase):
    def test_composition_preserves_tests_and_stops_incomplete_analysis(self):
        spec = importlib.util.spec_from_file_location(
            "architecture_workflow", ROOT / "workflow.py"
        )
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        original = {
            "nodes": [
                {
                    "id": "tests",
                    "kind": "execution",
                    "execution": {
                        "mode": "command",
                        "command": "make verify",
                        "output": {"source": "exit_code"},
                    },
                    "rule_handoff": {
                        "rules": [
                            {"when": "$ == 0", "target": "code_review"},
                            {"when": "$ != 0", "target": "development"},
                        ]
                    },
                },
                {"id": "code_review"},
                {"id": "development"},
            ],
            "edges": [{"source": "tests", "route": "rule_1", "target": "code_review"}],
        }
        graph = module.add_check(original)
        self.assertEqual(
            original["nodes"][0]["rule_handoff"]["rules"][0]["target"], "code_review"
        )
        node = next(n for n in graph["nodes"] if n["id"] == "architecture_check")
        self.assertEqual(
            [(r["when"], r["target"]) for r in node["rule_handoff"]["rules"]],
            [
                ("$ == 0", "code_review"),
                ("$ == 1", "development"),
                ("$ == 2", "architecture_blocked"),
            ],
        )
        self.assertEqual(graph["nodes"][0]["execution"]["command"], "make verify")
        self.assertFalse(node["execution"].get("env_refs"))
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            command = [
                sys.executable,
                str(ROOT / "scaffold.py"),
                "--install-tool",
                str(root),
            ]
            subprocess.run(command, check=True, capture_output=True)
            (root / "go.mod").write_text("module example.org/service\n\ngo 1.25\n")
            (root / "src").mkdir()
            (root / "src/app.go").write_text("package app\n")
            (root / "architecture.json").write_text(
                json.dumps(
                    {
                        "languages": ["go"],
                        "source_roots": ["src"],
                        "modules": [{"name": "app", "paths": ["src/**"]}],
                    }
                )
            )
            argv = shlex.split(node["execution"]["command"])
            clean = subprocess.run(argv, cwd=root, capture_output=True, text=True)
            self.assertEqual(clean.returncode, 0, clean.stderr + clean.stdout)
            manifest = root / ".harness/archcheck/SOURCE.json"
            saved = manifest.read_bytes()
            manifest.write_text("null")
            invalid = subprocess.run(argv, cwd=root, capture_output=True, text=True)
            self.assertEqual(invalid.returncode, 2, invalid.stderr + invalid.stdout)
            self.assertEqual(
                json.loads(
                    (
                        root / "harness-output/architecture/latest-report.json"
                    ).read_text()
                )["status"],
                "incomplete",
            )
            manifest.write_bytes(saved)
            shadow = root / ".harness/archcheck/tempfile.py"
            shadow.write_text("raise RuntimeError('must never load')")
            added = subprocess.run(argv, cwd=root, capture_output=True, text=True)
            self.assertEqual(added.returncode, 2, added.stderr + added.stdout)
            shadow.unlink()
            (root / ".harness/archcheck/inspect_project.py").write_text(
                "raise SystemExit(0)\n"
            )
            drift = subprocess.run(argv, cwd=root, capture_output=True, text=True)
            self.assertEqual(drift.returncode, 2, drift.stderr + drift.stdout)
        with self.assertRaises(ValueError):
            module.add_check(graph)
