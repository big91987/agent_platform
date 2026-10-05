"""Project verification commands remain argv, never implicit shell snippets."""

import importlib.util
import json
import subprocess
import sys
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "workflow_install", Path(__file__).with_name("install.py")
)
install = importlib.util.module_from_spec(spec)
spec.loader.exec_module(install)


class VerificationCommandTest(unittest.TestCase):
    def test_go_project_can_select_its_own_verification_entry(self):
        self.assertEqual(
            install.verification_command('["make", "verify"]'), ["make", "verify"]
        )

    def test_argument_is_not_evaluated_as_shell_code(self):
        literal = "$(exit 91); untouched"
        command = install.verification_command(
            json.dumps(
                [sys.executable, "-c", "import sys; print(sys.argv[1])", literal]
            )
        )
        result = subprocess.run(command, text=True, capture_output=True, check=True)
        self.assertEqual(result.stdout.strip(), literal)

    def test_invalid_command_cannot_become_env_options(self):
        for value in (
            "[]",
            '"go test ./..."',
            '["--ignore-environment"]',
            '["go", 1]',
            '["go", "a\\u0000b"]',
            "{}",
        ):
            with self.subTest(value=value), self.assertRaises(ValueError):
                install.verification_command(value)


class SharedBrowserTest(unittest.TestCase):
    def test_two_projects_reference_one_registration(self):
        import tempfile

        class API:
            url = "http://localhost"

            def __init__(self):
                self.items = []

            def call(self, method, path, body=None):
                if method == "GET":
                    return self.items
                if method == "POST" and path.endswith("/discover"):
                    return {}
                if method == "POST":
                    self.items.append(dict(body))
                    return self.items[-1]
                raise AssertionError((method, path))

        api = API()
        with tempfile.TemporaryDirectory() as tmp:
            for project in ("first", "second"):
                installation = install.Installation(
                    api, Path(tmp) / project / "install.json", {"project": project}
                )
                result = install.install_shared_browser(
                    api, installation, Path(tmp) / "shared.json", Path(tmp) / "evidence"
                )
                self.assertEqual(result["id"], "browser-validation")
            self.assertEqual(len(api.items), 1)
            self.assertIn("{{workspace}}", api.items[0]["connection"]["args"])


class UpgradeRunSafetyTest(unittest.TestCase):
    def installation(self, directory):
        installation = install.Installation(
            None, Path(directory) / "install.json", {}, True
        )
        installation.data["objects"] = {
            "development": {"id": "dev", "spec": {"executor": "codex"}},
            "qa": {"id": "qa", "spec": {"executor": "codex"}},
            "browser": {
                "id": "legacy-browser",
                "spec": {"id": "legacy-browser", "name": "Legacy", "enabled": True},
            },
        }
        return installation

    def test_role_keyed_manifest_blocks_active_run_before_writes(self):
        import tempfile

        class API:
            def call(self, method, path, body=None):
                if method != "GET":
                    raise AssertionError("upgrade wrote before checking active run")
                return [
                    {
                        "id": "live",
                        "status": "running",
                        "definition": {"nodes": [{"agent_id": "dev"}]},
                    }
                ]

        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(ValueError, "live"):
                install.check_upgrade_runs(API(), self.installation(tmp))

    def test_upgrade_checks_active_run_beyond_recent_page(self):
        import tempfile

        class API:
            def call(self, method, path, body=None):
                if "before=" in path:
                    return [
                        {
                            "id": "old-live",
                            "status": "running",
                            "definition": {"nodes": [{"agent_id": "dev"}]},
                        }
                    ]
                return [
                    {"id": str(i), "status": "completed", "definition": {"nodes": []}}
                    for i in range(200)
                ]

        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(ValueError, "old-live"):
                install.check_upgrade_runs(API(), self.installation(tmp))

    def test_old_platform_ignoring_cursor_fails_closed(self):
        class API:
            def call(self, method, path, body=None):
                return [{"id": str(i)} for i in range(200)]

        with self.assertRaisesRegex(ValueError, "pagination did not advance"):
            list(install.workflow_runs(API()))

    def test_retirement_preserves_tool_for_resumable_run(self):
        import tempfile

        class API:
            def call(self, method, path, body=None):
                if method != "GET":
                    raise AssertionError("resumable tool was changed")
                if path == "/api/agents":
                    return []
                return [
                    {
                        "id": "paused",
                        "status": "stopped",
                        "definition": {"nodes": [{"agent_id": "qa"}]},
                    }
                ]

        with tempfile.TemporaryDirectory() as tmp:
            installation = self.installation(tmp)
            installation.api = API()
            install.retire_project_browser(installation.api, installation)


if __name__ == "__main__":
    unittest.main()
