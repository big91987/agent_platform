"""The standard assessment runs the configured model unless rules-only is explicit."""

import json
import os
import subprocess
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

TOOL = Path(__file__).resolve().parents[1] / "inspect_project.py"


class DefaultReviewTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root / "src").mkdir()
        (self.root / "src/main.go").write_text("package app\n")
        (self.root / "go.mod").write_text("module example.org/app\n\ngo 1.25\n")
        (self.root / "architecture.json").write_text(
            json.dumps(
                dict(
                    languages=["go"],
                    source_roots=["src"],
                    modules=[dict(name="app", paths=["src/**"])],
                )
            )
        )
        self.env = {
            k: v for k, v in os.environ.items() if not k.startswith("CODE_REVIEW_")
        }

    def inspect(self, *args, env=None):
        r = subprocess.run(
            [sys.executable, "-I", str(TOOL), "--root", str(self.root), *args],
            env=self.env if env is None else env,
            capture_output=True,
            text=True,
        )
        receipt = json.loads(r.stdout)
        return r, json.loads((self.root / receipt["report"]).read_text())

    def test_default_missing_configuration_fails_instead_of_reporting_rules_as_assessment(
        self,
    ):
        r, report = self.inspect()
        self.assertEqual(r.returncode, 2)
        self.assertEqual(report["model_review"]["status"], "incomplete")
        self.assertIsNone(report["score"]["total"])
        self.assertIn("CODE_REVIEW_", " ".join(report["errors"]))

    def test_explicit_rules_only_runs_without_model_configuration(self):
        r, report = self.inspect("--rules-only")
        self.assertEqual(r.returncode, 0)
        self.assertEqual(report["model_review"]["status"], "not_run")
        self.assertIsNone(report["score"]["total"])

    def test_default_uses_configured_provider_and_generates_a_score_and_plan(self):
        received = []

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                payload = json.loads(
                    self.rfile.read(int(self.headers["Content-Length"]))
                )
                b = json.loads(payload["messages"][1]["content"])
                received.append(payload["model"])
                evidence = [dict(path="src/main.go", quote="package app")]
                response = dict(
                    snapshot=b["snapshot"],
                    reviewed_files=sorted(b["sources"]),
                    scope_notes=[
                        "Controlled transport fixture, not real model quality"
                    ],
                    blockers=[],
                    findings=[],
                    checks=[
                        dict(
                            name=c["name"],
                            status="conformant",
                            reason="Fixture",
                            evidence=evidence,
                            criteria=[
                                dict(
                                    name=p["name"],
                                    status="conformant",
                                    reason="Fixture",
                                    evidence=evidence,
                                    finding_refs=[],
                                )
                                for p in c["criteria"]
                            ],
                        )
                        for c in b["review_checks"]
                    ],
                    architecture_plan=dict(
                        summary="保留当前结构",
                        modules=[
                            dict(
                                name="app",
                                current_responsibility="现有职责",
                                responsibility="现有职责",
                                current_paths=["src"],
                                target_paths=["src"],
                                finding_refs=[],
                                evidence=evidence,
                            )
                        ],
                        changes=[],
                    ),
                )
                body = json.dumps(
                    dict(
                        model="fixture-model",
                        choices=[
                            dict(
                                finish_reason="stop",
                                message=dict(content=json.dumps(response)),
                            )
                        ],
                        usage={},
                    )
                ).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

        server = HTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            env = dict(
                self.env,
                CODE_REVIEW_URL=f"http://127.0.0.1:{server.server_port}/chat/completions",
                CODE_REVIEW_TOKEN="fixture-token",
                CODE_REVIEW_MODEL="fixture-model",
                CODE_REVIEW_STREAM="false",
            )
            r, report = self.inspect(env=env)
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
            self.assertEqual(received, ["fixture-model"])
            self.assertEqual(report["score"]["total"], 100.0)
            self.assertEqual(
                report["model_review"]["architecture_plan"]["status"], "ready"
            )
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == "__main__":
    unittest.main()
