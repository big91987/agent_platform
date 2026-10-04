"""Real MCP/browser regression: native zoom, AX evidence and task-owned scripts."""

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from pipeline import prepare_registration

ROOT = Path(__file__).resolve().parents[2]


class BrowserCapabilitiesTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="browser-capabilities-")
        self.addCleanup(self.tmp.cleanup)
        self.workspace = Path(self.tmp.name).resolve() / "product"
        (self.workspace / "app").mkdir(parents=True)
        (self.workspace / "app/index.html").write_text(
            '<!doctype html><meta charset="utf-8"><p>版本 0.1.0 rc2</p>'
            "<button>Continue</button><p hidden>Hidden fixture</p>"
        )
        subprocess.run(["git", "init", "-q", str(self.workspace)], check=True)
        self.directory = self.workspace / "docs/05-validation/tasks/1"
        self.directory.mkdir(parents=True)
        self.config = self.workspace.parent / "runner.json"
        settings = {
            "registry": str(self.workspace.parent / "registry"),
            "repository": "fixture/product",
            "python": sys.executable,
            "config_path": str(self.config),
        }
        self.config.write_text(json.dumps({"pipeline": settings}))
        prepare_registration(
            settings, {"workspace": str(self.workspace)}, "development", 1
        )

    def check(self, steps):
        plan = self.directory / "browser-plan.json"
        plan.write_text(json.dumps(steps))
        result = subprocess.run(
            [
                sys.executable,
                str(ROOT / "examples/github/browser_tool.py"),
                "--config",
                str(self.config),
                "--stage",
                "development",
            ],
            cwd=self.workspace,
            input=json.dumps(
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": "tools/call",
                    "params": {
                        "name": "check",
                        "arguments": {
                            "root": "app",
                            "plan": plan.relative_to(self.workspace).as_posix(),
                        },
                    },
                }
            )
            + "\n",
            text=True,
            capture_output=True,
            check=True,
            timeout=40,
        )
        reply = json.loads(result.stdout)["result"]
        return reply, reply["content"][0]["text"]

    def script(self, content):
        file = self.directory / "browser-scripts/assert.js"
        file.parent.mkdir(exist_ok=True)
        file.write_text(content)
        return {
            "action": "page_script",
            "script": file.relative_to(self.workspace).as_posix(),
        }

    def test_direct_runner_creates_ax_output_before_any_screenshot(self):
        plan = self.directory / "browser-plan.json"
        plan.write_text(
            json.dumps([{"action": "accessibility", "contains": "版本 0.1.0 rc2"}])
        )
        output = self.directory / "runner-checks/feature"
        self.assertFalse(output.exists())
        for attempt in range(2):
            result = subprocess.run(
                [
                    "node",
                    str(
                        ROOT / "examples/github/tooling/full_harness/browser/check.cjs"
                    ),
                    str(self.workspace / "app"),
                    str(output),
                    str(plan),
                ],
                text=True,
                capture_output=True,
                timeout=40,
            )
            self.assertEqual(
                result.returncode, 0, f"attempt {attempt}: {result.stderr}"
            )
            self.assertTrue(json.loads((output / "browser.json").read_text())["passed"])
            self.assertTrue(
                json.loads((output / "accessibility-1.json").read_text())["nodes"]
            )

    def test_native_zoom_reflows_and_ax_tree_is_recorded(self):
        script = self.script(
            "() => { if(innerWidth !== 720 || devicePixelRatio !== 2 || visualViewport.scale !== 1) throw Error('not native zoom'); return {width:innerWidth, host:typeof process, node:typeof require}; }"
        )
        reply, text = self.check(
            [
                {"action": "zoom", "factor": 2},
                script,
                {"action": "accessibility", "contains": "版本 0.1.0 rc2"},
                {"action": "viewport", "width": 320},
                {"action": "zoom", "factor": 2},
                {"action": "visible", "text": "版本 0.1.0 rc2"},
            ]
        )
        self.assertFalse(reply.get("isError"), text)
        result = json.loads(text)["result"]
        self.assertEqual(result["observations"][0]["actual"], 2)
        self.assertEqual(
            result["observations"][1]["value"],
            {"width": 720, "host": "undefined", "node": "undefined"},
        )
        ax = next(self.directory.rglob("accessibility-*.json"))
        self.assertIn("版本 0.1.0 rc2", ax.read_text())

    def test_task_assertion_failure_is_not_passed(self):
        reply, text = self.check(
            [self.script("() => { throw Error('intentional layout failure'); }")]
        )
        self.assertTrue(reply.get("isError"), text)
        self.assertIn("intentional layout failure", text)

    def test_hidden_text_does_not_satisfy_accessibility(self):
        reply, text = self.check(
            [{"action": "accessibility", "contains": "Hidden fixture"}]
        )
        self.assertTrue(reply.get("isError"), text)
        self.assertIn("Accessible text not found", text)

    def test_script_cannot_read_outside_task_or_follow_symlink(self):
        outside = self.workspace / "outside.js"
        outside.write_text("() => true")
        reply, text = self.check([{"action": "page_script", "script": "outside.js"}])
        self.assertTrue(reply.get("isError"), text)
        self.assertIn("task browser-scripts", text)
        reply, text = self.check([self.script("() => ({recovered: true})")])
        self.assertFalse(reply.get("isError"), text)
        link = self.directory / "browser-scripts/link.js"
        link.parent.mkdir(exist_ok=True)
        link.symlink_to(outside)
        reply, text = self.check(
            [
                {
                    "action": "page_script",
                    "script": link.relative_to(self.workspace).as_posix(),
                }
            ]
        )
        self.assertTrue(reply.get("isError"), text)
        self.assertIn("Symlinks need a project import policy", text)

    def test_task_script_cannot_fetch_external_network(self):
        reply, text = self.check(
            [
                self.script(
                    "async () => { try { await fetch('https://example.com'); } catch { return {blocked:true}; } throw Error('External network allowed'); }"
                )
            ]
        )
        self.assertFalse(reply.get("isError"), text)
        self.assertEqual(
            json.loads(text)["result"]["observations"][0]["value"], {"blocked": True}
        )

    def test_unchanged_storage_writes_passes_across_reload_with_observations(self):
        reply, text = self.check(
            [
                {"action": "snapshot_storage"},
                {"action": "zoom", "factor": 2},
                {"action": "accessibility", "contains": "0.1.0 rc2"},
                {"action": "reload"},
                {"action": "unchanged_storage"},
                {"action": "unchanged_storage_writes"},
            ]
        )
        self.assertFalse(reply.get("isError"), text)
        result = json.loads(text)["result"]
        self.assertEqual(result["storageWrites"], 0)
        self.assertEqual(result["observations"][0]["actual"], 2)

    def test_new_storage_writes_fail_even_when_original_value_is_restored(self):
        reply, text = self.check(
            [
                {"action": "snapshot_storage"},
                self.script(
                    "() => { localStorage.setItem('probe', 'value'); localStorage.removeItem('probe'); return true; }"
                ),
                {"action": "reload"},
                {"action": "unchanged_storage"},
                {"action": "unchanged_storage_writes"},
            ]
        )
        self.assertTrue(reply.get("isError"), text)
        result = json.loads(text)["result"]
        self.assertEqual(result["storageWrites"], 2)
        self.assertEqual(result["performed"][-1]["action"], "unchanged_storage")
        self.assertIn("localStorage write attempts changed", result["failure"])
        self.assertNotIn("message", result["failure"])
