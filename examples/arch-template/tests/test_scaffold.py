"""Generated projects must not depend on the platform worktree."""

import fcntl
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SOURCE = Path(__file__).resolve().parents[1]


class ScaffoldTests(unittest.TestCase):
    def test_install_existing_repository_keeps_product_and_refuses_replacement(self):
        self.project.mkdir()
        product = self.project / "AGENTS.md"
        product.write_text("existing rules\n")
        command = [
            sys.executable,
            str(SOURCE / "scaffold.py"),
            "--install-tool",
            str(self.project),
        ]
        installed = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(installed.returncode, 0, installed.stderr)
        self.assertEqual(product.read_text(), "existing rules\n")
        self.assertTrue(
            (self.project / ".harness/archcheck/inspect_project.py").exists()
        )
        again = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(again.returncode, 2)
        self.assertIn("already", again.stderr)

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.project = Path(self.tmp.name) / "new-project"

    def scaffold(self, *args):
        return subprocess.run(
            [
                sys.executable,
                str(SOURCE / "scaffold.py"),
                "--destination",
                str(self.project),
                *args,
            ],
            capture_output=True,
            text=True,
        )

    def test_combined_backend_and_frontend_export_builds_and_checks_independently(self):
        r = self.scaffold(
            "--backend", "saas", "--backend", "model-gateway", "--frontend", "vue"
        )
        self.assertEqual(r.returncode, 0, r.stderr + r.stdout)
        self.assertTrue((self.project / "web/src/app/App.vue").exists())
        self.assertFalse((self.project / ".harness/archcheck/node_modules").exists())
        r = subprocess.run(
            ["go", "test", "./..."], cwd=self.project, capture_output=True, text=True
        )
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        r = subprocess.run(
            [sys.executable, ".harness/archcheck/archcheck.py", "check"],
            cwd=self.project,
            capture_output=True,
            text=True,
        )
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("docs/engineering-principles.md", json.loads(r.stdout)["hashes"])
        frontend = subprocess.run(
            [
                sys.executable,
                str(SOURCE / "tool/archcheck.py"),
                "check",
                "--root",
                str(self.project),
                "--config",
                "web/architecture.json",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(frontend.returncode, 0, frontend.stdout + frontend.stderr)
        self.assertIn(
            "docs/engineering-principles.md", json.loads(frontend.stdout)["hashes"]
        )
        self.assertEqual(self.scaffold("--backend", "saas").returncode, 2)

    def test_default_review_artifacts_ignored_after_export_and_upgrade(self):
        r = self.scaffold("--backend", "saas")
        self.assertEqual(r.returncode, 0, r.stderr)
        subprocess.run(["git", "init", "-q", str(self.project)], check=True)
        paths = [
            "harness-output/architecture/review-input.json",
            ".harness-output/backend-architecture.json",
        ]
        for name in paths:
            r = subprocess.run(
                ["git", "check-ignore", name],
                cwd=self.project,
                capture_output=True,
                text=True,
            )
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        (self.project / ".gitignore").write_text(".harness-output/\n")
        r = subprocess.run(
            [
                sys.executable,
                str(SOURCE / "scaffold.py"),
                "--update-tool",
                str(self.project),
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(r.returncode, 0, r.stderr)
        r = subprocess.run(
            ["git", "check-ignore", paths[0]],
            cwd=self.project,
            capture_output=True,
            text=True,
        )
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_frontend_only_contains_no_go_module(self):
        r = self.scaffold("--frontend", "vue")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertFalse((self.project / "go.mod").exists())

    def test_saas_export_enforces_application_and_domain_boundaries(self):
        self.assertEqual(self.scaffold("--backend", "saas").returncode, 0)
        command = [sys.executable, ".harness/archcheck/archcheck.py", "check"]
        clean = subprocess.run(
            command, cwd=self.project, capture_output=True, text=True
        )
        self.assertEqual(clean.returncode, 0, clean.stdout + clean.stderr)
        bypasses = [
            (
                "internal/httpapi/bypass.go",
                "httpapi",
                "example.com/service/internal/accounting/domain",
                "dependency",
            ),
            (
                "internal/accounting/application/bypass.go",
                "application",
                "example.com/service/internal/infrastructure/sqlite",
                "dependency",
            ),
            (
                "internal/accounting/domain/bypass.go",
                "domain",
                "database/sql",
                "forbidden_import",
            ),
            (
                "internal/accounting/domain/bypass.go",
                "domain",
                "database/sql/driver",
                "forbidden_import",
            ),
            (
                "internal/accounting/domain/bypass.go",
                "domain",
                "example.org/new-library",
                "external_dependency",
            ),
        ]
        for relative, package, imported, rule in bypasses:
            with self.subTest(relative=relative, imported=imported):
                path = self.project / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(f'package {package}\nimport _ "{imported}"\n')
                result = subprocess.run(
                    command, cwd=self.project, capture_output=True, text=True
                )
                path.unlink()
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                report = json.loads(result.stdout)
                self.assertTrue(
                    any(
                        f["rule"] == rule and f["source"] == relative
                        for f in report["rules"]["findings"]
                    ),
                    report,
                )

    def test_combining_scenarios_preserves_external_restrictions_in_both_orders(self):
        for order in [("saas", "model-gateway"), ("model-gateway", "saas")]:
            with self.subTest(order=order):
                self.project = Path(self.tmp.name) / ("-".join(order))
                result = self.scaffold("--backend", order[0], "--backend", order[1])
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                path = self.project / "internal/httpapi/unapproved.go"
                path.write_text('package httpapi\nimport _ "example.org/unapproved"\n')
                result = subprocess.run(
                    [sys.executable, ".harness/archcheck/archcheck.py", "check"],
                    cwd=self.project,
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertTrue(
                    any(
                        f["rule"] == "external_dependency"
                        for f in json.loads(result.stdout)["rules"]["findings"]
                    )
                )

    def test_tool_upgrade_refuses_local_changes_and_allows_unmodified_installation(
        self,
    ):
        self.assertEqual(self.scaffold("--backend", "model-gateway").returncode, 0)
        command = [
            sys.executable,
            str(SOURCE / "scaffold.py"),
            "--update-tool",
            str(self.project),
        ]
        r = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        tool = self.project / ".harness/archcheck/archcheck.py"
        tool.write_text(tool.read_text() + "\n# local change\n")
        r = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(r.returncode, 2, r.stderr)
        self.assertIn("local change", tool.read_text())

    def test_tool_upgrade_refuses_active_execution_lock(self):
        self.assertEqual(self.scaffold("--backend", "model-gateway").returncode, 0)
        tool = self.project / ".harness/archcheck"
        before = (tool / "archcheck.py").read_bytes()
        with (tool / "SOURCE.json").open("rb") as active:
            fcntl.flock(active, fcntl.LOCK_SH)
            result = subprocess.run(
                [
                    sys.executable,
                    str(SOURCE / "scaffold.py"),
                    "--update-tool",
                    str(self.project),
                ],
                capture_output=True,
                text=True,
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertEqual((tool / "archcheck.py").read_bytes(), before)

    def test_no_selection_does_not_leave_partial_destination(self):
        self.assertEqual(self.scaffold().returncode, 2)
        self.assertFalse(self.project.exists())


if __name__ == "__main__":
    unittest.main()
