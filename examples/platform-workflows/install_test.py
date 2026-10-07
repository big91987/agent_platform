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


class ProxyUpgradeTest(unittest.TestCase):
    def test_api_null_env_references_upgrade_like_empty(self):
        self.assertEqual(install.proxy_env_refs(None, {}), {})

    def test_fresh_direct_install_needs_no_proxy_environment(self):
        self.assertEqual(install.proxy_env_refs({}, {}), {})
        self.assertEqual(
            install.proxy_env_refs({}, {"HTTPS_PROXY": "http://proxy"}),
            {"HTTPS_PROXY": "HTTPS_PROXY"},
        )

    def test_upgrade_preserves_configured_references_without_cli_environment(self):
        old = {"HTTPS_PROXY": "PLATFORM_PROXY", "GH_TOKEN": "PRIVATE_TOKEN"}
        self.assertEqual(
            install.proxy_env_refs(old, {}), {"HTTPS_PROXY": "PLATFORM_PROXY"}
        )
        self.assertEqual(
            install.proxy_env_refs(old, {"HTTPS_PROXY": "http://new"}),
            {"HTTPS_PROXY": "HTTPS_PROXY"},
        )


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


class MaterialInstallCommandTest(unittest.TestCase):
    def test_every_fixed_delivery_command_is_installed_from_source(self):
        prepare = install.repository_command(
            "prepare", "owner/repo", "main", ["make", "verify"]
        )
        publish = install.repository_command(
            "publish", "owner/repo", "main", ["make", "verify"]
        )
        tests = install.repository_command(
            "tests", "owner/repo", "main", ["make", "verify"]
        )
        for argv in [prepare, publish, tests]:
            self.assertEqual(argv[0], str(install.HERE / "repository.py"))
            self.assertIn("--materials", argv)
            self.assertIn("owner/repo", argv)
        self.assertIn("verify", tests)
        self.assertEqual(
            json.loads(tests[tests.index("--test-command") + 1]), ["make", "verify"]
        )


class MaterialManifestUpgradeTest(unittest.TestCase):
    def test_first_repeat_upgrade_and_drift_keep_original_manifest_and_ids(self):
        import tempfile

        class API:
            def __init__(self):
                self.items = []
                self.writes = 0

            def call(self, method, path, body=None):
                if method == "GET":
                    return self.items
                self.writes += 1
                if method == "POST":
                    result = {**body, "id": "connector-1", "revision": 1}
                    self.items.append(result)
                    return result
                if method == "PUT":
                    result = {**body, "revision": self.items[0]["revision"] + 1}
                    self.items[0] = result
                    return result
                raise AssertionError(method)

        with tempfile.TemporaryDirectory() as tmp:
            api = API()
            path = Path(tmp) / "manifest.json"
            old = {
                "name": "tests",
                "kind": "command",
                "executable": "/usr/bin/env",
                "args": ["make", "verify"],
            }
            fresh = install.Installation(api, path, {"repository": "owner/repo"})
            old_id = fresh.apply("connectors", "connector-tests", old)["id"]
            repeat = install.Installation(api, path, {"repository": "owner/repo"})
            repeat.apply("connectors", "connector-tests", old)
            self.assertEqual(api.writes, 1)
            new = {
                **old,
                "executable": sys.executable,
                "args": install.repository_command(
                    "tests", "owner/repo", "main", ["make", "verify"]
                ),
            }
            with self.assertRaises(ValueError):
                repeat.apply("connectors", "connector-tests", new)
            upgraded = install.Installation(
                api, path, {"repository": "owner/repo"}, True
            )
            self.assertEqual(
                upgraded.apply("connectors", "connector-tests", new)["id"], old_id
            )
            self.assertEqual(
                json.loads(path.read_text())["objects"]["connector-tests"]["spec"][
                    "args"
                ],
                new["args"],
            )
            same = install.Installation(api, path, {"repository": "owner/repo"}, True)
            same.apply("connectors", "connector-tests", new)
            self.assertEqual(api.writes, 2)
            api.items[0]["args"] = ["unexpected external edit"]
            with self.assertRaisesRegex(ValueError, "edited outside"):
                same.apply("connectors", "connector-tests", new)
            self.assertEqual(api.writes, 2)


class SessionPromptTest(unittest.TestCase):
    def test_role_is_stable_and_node_session_owns_task_policy(self):
        role, session = install.stage_prompts(
            "development", "公共节点工作说明", "make verify"
        )
        self.assertNotIn("公共节点工作说明", role)
        self.assertNotIn("make verify", role)
        self.assertIn("公共节点工作说明", session)
        self.assertIn("make verify", session)
        self.assertEqual(session.count("{{handoff}}"), 1)


class WaitingSOPTest(unittest.TestCase):
    def test_installed_sop_has_one_explicit_wait_contract(self):
        common = Path(__file__).with_name("prompts").joinpath("common.md").read_text()
        _, session = install.stage_prompts("development", common, "make verify")
        self.assertNotIn("正常输出问题并等待即可", session)
        self.assertNotIn("outgoing_edges", session)
        self.assertIn("wait_for_input", session)


if __name__ == "__main__":
    unittest.main()
