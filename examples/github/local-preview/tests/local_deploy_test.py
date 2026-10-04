import importlib.util
import json
import os
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

MODULE = Path(__file__).parents[1] / "scripts" / "local_deploy.py"


class DeploymentTest(unittest.TestCase):
    def setUp(self):
        self.assertTrue(MODULE.exists(), "durable local deployment is not implemented")
        spec = importlib.util.spec_from_file_location("local_deploy", MODULE)
        self.d = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.d)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.git("init", "-q", "-b", "main")
        self.git("config", "user.email", "test@example.invalid")
        self.git("config", "user.name", "Test")
        (self.repo / "app").mkdir()
        for name, content in {
            "index.html": "<html><head></head><body>Hello</body></html>",
            "app.js": "const version = 1;",
            "styles.css": "body {}",
        }.items():
            (self.repo / "app" / name).write_text(content)
        self.runtime = self.root / "runtime"
        self.runtime.mkdir()

    def git(self, *args):
        return subprocess.check_output(
            ["git", "-C", str(self.repo), *args], text=True
        ).strip()

    def commit(self, impact="none", declare=True):
        if declare:
            self.d.declare(
                self.repo,
                impact,
                "Tested change",
                "deploy/migration.md" if impact == "migration" else "",
            )
        if impact == "migration":
            (self.repo / "deploy" / "migration.md").write_text(
                "Backup before conversion; retain source; validate; recover from backup."
            )
        self.git("add", ".")
        self.git("commit", "-qm", "test release")
        return self.git("rev-parse", "HEAD")

    def plan(self, sha):
        return self.d.prepare(self.runtime, self.repo, sha)

    def test_obsolete_and_already_deployed_requests_exit_without_approval(self):
        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        (self.repo / "app/styles.css").write_text("body {color: blue}")
        second = self.commit()
        self.git("remote", "add", "origin", str(self.repo))
        (self.runtime / "repository").symlink_to(self.repo)
        output = self.root / "output"
        planfile = self.root / "plan.json"
        for sha in (first, second):
            if sha == second:
                self.d.activate(self.runtime, self.plan(second), health=lambda: True)
            result = subprocess.run(
                [
                    sys.executable,
                    str(MODULE),
                    "prepare",
                    "--root",
                    str(self.runtime),
                    "--sha",
                    sha,
                    "--plan",
                    str(planfile),
                    "--review",
                ],
                env={**os.environ, "GITHUB_OUTPUT": str(output)},
                capture_output=True,
                text=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("deploy=false", output.read_text())
            self.assertFalse(planfile.exists())
        self.assertEqual(self.d.current(self.runtime), second)

    def test_approved_old_plan_skips_after_main_advances(self):
        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        (self.repo / "app/styles.css").write_text("body {color: red}")
        second = self.commit()
        planfile = self.root / "plan.json"
        planfile.write_text(json.dumps(self.plan(second)))
        (self.repo / "app/styles.css").write_text("body {color: blue}")
        self.commit()
        self.git("remote", "add", "origin", str(self.repo))
        (self.runtime / "repository").symlink_to(self.repo)
        result = subprocess.run(
            [
                sys.executable,
                str(MODULE),
                "activate",
                "--root",
                str(self.runtime),
                "--plan",
                str(planfile),
                "--approved",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("supersedes", result.stdout)
        self.assertEqual(self.d.current(self.runtime), first)

    def test_cancellation_during_switch_restores_previous_release(self):
        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        (self.repo / "app/styles.css").write_text("body {color: blue}")
        plan = self.plan(self.commit())
        planfile = self.root / "plan.json"
        planfile.write_text(json.dumps(plan))
        marker = self.root / "health-started"
        script = """
import importlib.util, json, sys, time
from pathlib import Path
spec = importlib.util.spec_from_file_location('deploy', sys.argv[1])
d = importlib.util.module_from_spec(spec)
spec.loader.exec_module(d)
def health():
    Path(sys.argv[4]).touch()
    time.sleep(30)
    return True
d.activate(Path(sys.argv[2]), json.loads(Path(sys.argv[3]).read_text()), health=health)
"""
        for sig in (signal.SIGINT, signal.SIGTERM):
            with self.subTest(signal=sig):
                marker.unlink(missing_ok=True)
                proc = subprocess.Popen(
                    [
                        sys.executable,
                        "-c",
                        script,
                        str(MODULE),
                        str(self.runtime),
                        str(planfile),
                        str(marker),
                    ],
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                )
                try:
                    deadline = time.monotonic() + 5
                    while (
                        not marker.exists()
                        and proc.poll() is None
                        and time.monotonic() < deadline
                    ):
                        time.sleep(0.02)
                    self.assertTrue(
                        marker.exists(), "activation never reached health check"
                    )
                    proc.send_signal(sig)
                    proc.wait(timeout=3)
                    self.assertNotEqual(proc.returncode, 0)
                    self.assertEqual(self.d.current(self.runtime), first)
                    self.assertEqual(
                        json.loads((self.runtime / "deployed.json").read_text())["sha"],
                        first,
                    )
                finally:
                    if proc.poll() is None:
                        proc.kill()
                        proc.wait()

    def test_safe_release_switch_keeps_data_and_previous_version(self):
        first = self.commit()
        plan = self.plan(first)
        self.assertFalse(plan["requires_approval"])
        (self.runtime / "data").mkdir()
        (self.runtime / "data" / "keep").write_text("user data")
        self.d.activate(self.runtime, plan, health=lambda: True)
        (self.repo / "app" / "styles.css").write_text("body {color: blue}")
        second = self.commit()
        self.d.activate(self.runtime, self.plan(second), health=lambda: True)
        self.assertEqual(self.d.current(self.runtime), second)
        self.assertEqual((self.runtime / "previous").resolve().name, first)
        self.assertEqual((self.runtime / "data" / "keep").read_text(), "user data")

    def test_migration_and_undeclared_changes_wait_without_touching_live(self):
        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        (self.repo / "app" / "app.js").write_text("const version = 2;")
        second = self.commit(declare=False)
        plan = self.plan(second)
        self.assertTrue(plan["requires_approval"])
        with self.assertRaisesRegex(ValueError, "approval"):
            self.d.activate(self.runtime, plan, health=lambda: True)
        self.assertEqual(self.d.current(self.runtime), first)
        third = self.commit(impact="migration")
        plan = self.plan(third)
        self.assertTrue(plan["requires_approval"])
        self.d.activate(self.runtime, plan, approved=True, health=lambda: True)
        self.assertEqual(self.d.current(self.runtime), third)

    def test_failed_health_restores_old_release(self):
        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        (self.repo / "app" / "app.js").write_text("const version = 2;")
        plan = self.plan(self.commit())
        with self.assertRaisesRegex(RuntimeError, "health"):
            self.d.activate(self.runtime, plan, health=lambda: False)
        self.assertEqual(self.d.current(self.runtime), first)

    def test_tampered_release_cannot_replace_current(self):
        first = self.commit()
        plan = self.plan(first)
        (self.runtime / "releases" / first / "app.js").write_text("tampered")
        with self.assertRaisesRegex(ValueError, "changed"):
            self.d.activate(self.runtime, plan, health=lambda: True)
        self.assertIsNone(self.d.current(self.runtime))

    def test_stale_plan_cannot_overwrite_a_newer_deployment(self):
        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        (self.repo / "app/styles.css").write_text("body {color: red}")
        second = self.commit()
        waiting = self.plan(second)
        (self.repo / "app/styles.css").write_text("body {color: blue}")
        third = self.commit()
        self.d.activate(
            self.runtime, self.plan(third), approved=True, health=lambda: True
        )
        with self.assertRaisesRegex(ValueError, "stale"):
            self.d.activate(self.runtime, waiting, health=lambda: True)
        self.assertEqual(self.d.current(self.runtime), third)

    def test_invalid_javascript_and_missing_migration_plan_keep_live_release(self):
        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        (self.repo / "app/app.js").write_text("const = ;")
        with self.assertRaises(subprocess.CalledProcessError):
            self.plan(self.commit())
        self.assertEqual(self.d.current(self.runtime), first)
        (self.repo / "app/app.js").write_text("const version = 3;")
        with self.assertRaisesRegex(ValueError, "requires a deploy/"):
            self.plan(self.commit(impact="destructive"))
        self.assertEqual(self.d.current(self.runtime), first)

    def test_safe_followup_cannot_hide_an_unreleased_migration(self):
        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        (self.repo / "app/app.js").write_text("const migration = true;")
        migration = self.commit(impact="migration")
        self.assertTrue(self.plan(migration)["requires_approval"])
        (self.repo / "app/styles.css").write_text("body {color: blue}")
        followup = self.commit()
        plan = self.plan(followup)
        self.assertTrue(
            plan["requires_approval"],
            "followup is compatible with an unreleased base, not the live version",
        )
        with self.assertRaisesRegex(ValueError, "approval"):
            self.d.activate(self.runtime, plan, health=lambda: True)
        self.assertEqual(self.d.current(self.runtime), first)

    def test_server_pins_assets_and_never_exposes_private_runtime(self):
        import threading
        import urllib.error
        import urllib.request

        first = self.commit()
        self.d.activate(self.runtime, self.plan(first), health=lambda: True)
        server = self.d.server(self.runtime, port=0)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        base = f"http://127.0.0.1:{server.server_port}"
        with urllib.request.urlopen(base) as response:
            html = response.read().decode()
            self.assertIn(f"/_releases/{first}/", html)
            self.assertIn("no-store", response.headers["Cache-Control"])
        with urllib.request.urlopen(base + "/__deployment.json") as response:
            self.assertEqual(json.load(response)["sha"], first)
        (self.repo / "app/app.js").write_text("const version = 2;")
        pending = self.commit(impact="migration")
        self.plan(pending)
        for path in [
            f"/_releases/{pending}/",
            f"/_releases/{pending}/app.js",
            "/../data/keep",
            "/_releases/../../data/keep",
            "/control/local_deploy.py",
        ]:
            with self.assertRaises(urllib.error.HTTPError) as raised:
                urllib.request.urlopen(base + path)
            raised.exception.close()
        (self.runtime / "published" / (first + ".json")).unlink()

        def health_with_old_page_loading():
            with urllib.request.urlopen(
                base + f"/_releases/{first}/app.js"
            ) as response:
                return b"version = 1" in response.read()

        self.d.activate(
            self.runtime,
            self.plan(pending),
            approved=True,
            health=health_with_old_page_loading,
        )
        with urllib.request.urlopen(base + f"/_releases/{first}/app.js") as response:
            self.assertIn(b"version = 1", response.read())


if __name__ == "__main__":
    unittest.main()
