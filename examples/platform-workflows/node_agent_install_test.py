"""Installer API payloads and manifest upgrades for independent node configs."""

import copy
import importlib.util
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "node_agent_install", Path(__file__).with_name("install.py")
)
install = importlib.util.module_from_spec(spec)
spec.loader.exec_module(install)


class MemoryAPI:
    url = "http://localhost"

    def __init__(self):
        self.objects = {
            key: [] for key in ("agents", "workflows", "connectors", "tool-servers")
        }
        self.runs = []
        self.writes = []

    def call(self, method, path, body=None):
        parts = path.split("/")
        collection = parts[2]
        if collection == "workflow-runs":
            return copy.deepcopy(self.runs)
        if method == "GET":
            return copy.deepcopy(self.objects[collection])
        if path.endswith(("/discover", "/check")):
            return {}
        self.writes.append((method, path, copy.deepcopy(body)))
        if method == "POST":
            result = {
                **body,
                "id": body.get(
                    "id", f"{collection}-{len(self.objects[collection]) + 1}"
                ),
                "revision": 1,
            }
            self.objects[collection].append(copy.deepcopy(result))
        elif method == "PUT":
            index = next(
                i
                for i, obj in enumerate(self.objects[collection])
                if obj["id"] == parts[3]
            )
            result = {
                **body,
                "revision": self.objects[collection][index]["revision"] + 1,
            }
            self.objects[collection][index] = copy.deepcopy(result)
        else:
            raise AssertionError((method, path))
        return copy.deepcopy(result)


class NodeAgentInstallTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        self.api = MemoryAPI()
        self.workspace = self.root / "workspace"
        self.workspace.mkdir()
        for role in (
            "defining-platform-products-cn",
            "platform-architecture-cn",
            "managing-engineering-delivery-cn",
            "trellis-before-dev",
            "trellis-check",
            "trellis-spec-bootstrap",
            "trellis-update-spec",
            "qa",
        ):
            directory = self.root / "skills" / role
            directory.mkdir(parents=True)
            (directory / "SKILL.md").write_text("test asset")
        self.manifest = self.root / "manifest.json"

    def run_install(self, *extra, environment=None):
        argv = [
            "install.py",
            "--platform-url",
            self.api.url,
            "--repository",
            "owner/repo",
            "--workspace-root",
            str(self.workspace),
            "--evidence",
            str(self.root / "evidence"),
            "--browser-evidence",
            str(self.root / "browser-evidence"),
            "--browser-manifest",
            str(self.root / "browser.json"),
            "--skill-root",
            str(self.root / "skills"),
            "--qa-skill",
            str(self.root / "skills/qa"),
            "--manifest",
            str(self.manifest),
            "--trellis-executable",
            sys.executable,
            *extra,
        ]
        with (
            patch.object(sys, "argv", argv),
            patch.object(install, "API", return_value=self.api),
            patch.object(install.subprocess, "check_output", return_value="0.6.15"),
            patch.dict(
                os.environ,
                {"PLATFORM_ADMIN_PASSWORD": "test", **(environment or {})},
                clear=True,
            ),
        ):
            try:
                install.main()
            except SystemExit as error:
                self.fail(
                    f"Installer rejected supported node-config invocation: {error}"
                )

    def test_fresh_install_needs_no_shared_agent_and_repeats_without_writes(self):
        self.run_install(
            "--executor",
            "codex",
            "--model",
            "chosen-model",
            environment={"HTTPS_PROXY": "http://proxy"},
        )
        self.assertEqual(self.api.objects["agents"], [])
        graph = self.api.objects["workflows"][0]
        nodes = {node["id"]: node for node in graph["nodes"] if node["kind"] == "agent"}
        self.assertEqual(
            set(nodes),
            {"intake", "requirements", "design", "development", "qa", "report"},
        )
        for node in nodes.values():
            self.assertNotIn("agent_id", node)
            config = node["agent"]
            self.assertEqual(config["executor"], "codex")
            self.assertEqual(config["model"], "chosen-model")
            self.assertEqual(config["env"], {"HTTPS_PROXY": "http://proxy"})
            self.assertFalse(
                set(config)
                & {"id", "name", "enabled", "authorized_users", "resolved_tools"}
            )
            self.assertNotIn("prompt", node)

            self.assertNotIn("{{handoff}}", config["instructions"])
            self.assertEqual(graph["context_version"], 2)
        self.assertEqual(nodes["intake"]["agent"]["skills"], [])
        self.assertEqual(
            nodes["requirements"]["agent"]["skills"],
            [str(self.root / "skills/defining-platform-products-cn")],
        )
        self.assertEqual(nodes["qa"]["agent"]["skills"], [str(self.root / "skills/qa")])
        self.assertEqual(len(nodes["development"]["agent"]["skills"]), 5)
        self.assertEqual(
            nodes["qa"]["agent"]["tool_servers"][0]["server_id"], "browser-validation"
        )
        writes = len(self.api.writes)
        self.run_install("--executor", "codex", "--model", "chosen-model")
        self.assertEqual(len(self.api.writes), writes)

    def test_explicit_execution_permissions_survive_upgrade_and_can_be_revoked(self):
        self.run_install("--agent-network-access", "--allow-agent-elevation")
        self.api.writes.clear()
        self.run_install("--upgrade")
        self.assertEqual(self.api.writes, [])
        for node in self.api.objects["workflows"][0]["nodes"]:
            if node["kind"] == "agent":
                self.assertTrue(node["agent"]["network_access"])
                self.assertTrue(node["agent"]["allow_elevation"])
        self.run_install(
            "--upgrade", "--no-agent-network-access", "--no-allow-agent-elevation"
        )
        for node in self.api.objects["workflows"][0]["nodes"]:
            if node["kind"] == "agent":
                self.assertFalse(node["agent"]["network_access"])
                self.assertFalse(node["agent"]["allow_elevation"])

    def test_product_e2e_keeps_product_failure_for_review_without_publish(self):
        self.run_install("--template", "product-e2e")
        graph = self.api.objects["workflows"][0]
        nodes = {node["id"]: node for node in graph["nodes"]}
        self.assertNotIn("publish", nodes)
        self.assertNotIn("pr", nodes)
        failed = [
            edge["target"]
            for edge in graph["edges"]
            if edge["source"] == "tests" and edge["route"] == "failed"
        ]
        self.assertEqual(failed, ["e2e_review"])
        self.assertIn("e2e_execute", nodes)

    def test_product_e2e_browser_skill_survives_upgrade_until_explicit_removal(self):
        browser = self.root / "skills" / "browser"
        browser.mkdir()
        (browser / "SKILL.md").write_text("real browser instructions")
        self.run_install("--template", "product-e2e", "--browser-skill", str(browser))
        self.api.writes.clear()
        self.run_install("--template", "product-e2e", "--upgrade")
        self.assertEqual(self.api.writes, [])
        self.run_install(
            "--template", "product-e2e", "--upgrade", "--clear-browser-skill"
        )
        for node in self.api.objects["workflows"][0]["nodes"]:
            if node["id"] in ("e2e_execute", "e2e_review"):
                self.assertEqual(
                    node["agent"]["skills"], [str(self.root / "skills/qa")]
                )

    def legacy_install(self):
        self.api.objects["agents"] = [
            {"id": "base", "name": "Base", "executor": "codex", "model": "copied-model"}
        ]
        self.run_install(
            "--base-agent", "base", environment={"HTTPS_PROXY": "http://legacy-proxy"}
        )
        data = json.loads(self.manifest.read_text())
        graph = self.api.objects["workflows"][0]
        for node in graph["nodes"]:
            if node["kind"] != "agent":
                continue
            role = node["id"]
            config = node.pop("agent")
            agent = {
                **config,
                "id": "legacy-" + role,
                "name": "Legacy " + role,
                "enabled": True,
                "authorized_users": ["user"],
            }
            self.api.objects["agents"].append(agent)
            node["agent_id"] = agent["id"]
            data["objects"][role] = {"id": agent["id"], "spec": copy.deepcopy(agent)}
        data["objects"]["workflow"]["spec"] = copy.deepcopy(graph)
        self.manifest.write_text(json.dumps(data))
        self.api.writes.clear()

    def test_legacy_upgrade_preserves_resources_and_frozen_run_then_repeats(self):
        self.legacy_install()
        agents = copy.deepcopy(self.api.objects["agents"])
        connectors = copy.deepcopy(self.api.objects["connectors"])
        graph = self.api.objects["workflows"][0]
        old_id = graph["id"]
        self.api.runs = [
            {
                "id": "stopped-run",
                "workflow_id": old_id,
                "status": "stopped",
                "definition": copy.deepcopy(graph),
            }
        ]
        frozen = copy.deepcopy(self.api.runs)
        self.run_install("--base-agent", "base", "--upgrade")
        self.assertEqual(self.api.objects["agents"], agents)
        self.assertEqual(self.api.objects["connectors"], connectors)
        self.assertEqual(self.api.runs, frozen)
        self.assertEqual(self.api.objects["workflows"][0]["id"], old_id)
        for node in self.api.objects["workflows"][0]["nodes"]:
            if node["kind"] == "agent":
                self.assertNotIn("agent_id", node)
                self.assertEqual(node["agent"]["model"], "copied-model")
                self.assertEqual(
                    node["agent"]["env"], {"HTTPS_PROXY": "http://legacy-proxy"}
                )
        self.assertEqual(
            [path for _, path, _ in self.api.writes], ["/api/workflows/" + old_id]
        )
        self.run_install("--base-agent", "base", "--upgrade")
        self.assertEqual(len(self.api.writes), 1)

    def test_legacy_agent_drift_is_not_silently_discarded(self):
        self.legacy_install()
        self.api.objects["agents"][1]["native_config"] = "user customization"
        with self.assertRaisesRegex(ValueError, "edited outside"):
            self.run_install("--base-agent", "base", "--upgrade")
        self.assertEqual(self.api.writes, [])

    def test_added_legacy_tools_are_rejected_before_migration_writes(self):
        self.legacy_install()
        saved = json.loads(self.manifest.read_text())["objects"]["intake"]["spec"]
        self.assertNotIn("tool_servers", saved)
        self.api.objects["agents"][1]["tool_servers"] = [
            {
                "server_id": "custom-tool",
                "tools": ["check"],
                "approvals": {"check": "ask"},
            }
        ]
        with self.assertRaisesRegex(ValueError, "intake was edited outside"):
            self.run_install("--upgrade")
        self.assertEqual(self.api.writes, [])

    def test_legacy_empty_execution_defaults_do_not_block_migration(self):
        self.legacy_install()
        agent = self.api.objects["agents"][1]
        agent.update(tool_servers=[], skills=None, env=None)
        data = json.loads(self.manifest.read_text())
        data["objects"]["intake"]["spec"]["env"] = {}
        for key in (
            "native_config",
            "network_access",
            "allow_elevation",
            "inherit_env",
            "trust_hooks",
            "seed_dir",
        ):
            data["objects"]["intake"]["spec"].pop(key)
        self.manifest.write_text(json.dumps(data))
        self.run_install("--upgrade")
        self.assertEqual(
            [path for _, path, _ in self.api.writes], ["/api/workflows/workflows-1"]
        )

    def test_legacy_environment_empty_value_is_still_configuration_drift(self):
        self.legacy_install()
        self.api.objects["agents"][1]["env"]["EXPLICIT_EMPTY"] = ""
        with self.assertRaisesRegex(ValueError, "intake was edited outside"):
            self.run_install("--upgrade")
        self.assertEqual(self.api.writes, [])

    def test_legacy_upgrade_without_base_option_keeps_model(self):
        self.legacy_install()
        self.run_install("--upgrade")
        for node in self.api.objects["workflows"][0]["nodes"]:
            if node["kind"] == "agent":
                self.assertEqual(node["agent"]["model"], "copied-model")
        self.api.objects["agents"][0]["model"] = "source-changed"
        self.run_install("--base-agent", "base", "--upgrade")
        self.assertEqual(len(self.api.writes), 1)

    def test_inline_config_drift_blocks_upgrade(self):
        self.run_install("--executor", "codex")
        graph = self.api.objects["workflows"][0]
        next(node for node in graph["nodes"] if node["kind"] == "agent")["agent"][
            "native_config"
        ] = "user customization"
        self.api.writes.clear()
        with self.assertRaisesRegex(ValueError, "edited outside"):
            self.run_install("--executor", "codex", "--upgrade")
        self.assertEqual(self.api.writes, [])

    def test_active_run_matches_workflow_id_even_without_legacy_agent_reference(self):
        self.run_install("--executor", "codex")
        self.api.writes.clear()
        for status in ("running", "waiting", "stopping"):
            self.api.runs = [
                {
                    "id": "live",
                    "workflow_id": self.api.objects["workflows"][0]["id"],
                    "status": status,
                    "definition": {"nodes": []},
                }
            ]
            with (
                self.subTest(status=status),
                self.assertRaisesRegex(ValueError, "live"),
            ):
                self.run_install("--executor", "codex", "--upgrade")
        self.assertEqual(self.api.writes, [])

    def test_other_templates_install_independent_nodes_without_stage_agents(self):
        self.run_install("--executor", "codex", "--template", "collaboration-check")
        self.run_install("--executor", "codex", "--template", "qa-rework")
        self.assertEqual(self.api.objects["agents"], [])
        for graph in self.api.objects["workflows"]:
            for node in graph["nodes"]:
                if node["kind"] == "agent":
                    self.assertIn("agent", node)
                    self.assertNotIn("agent_id", node)

    def test_additional_template_preserves_existing_installation_authorization(self):
        self.run_install("--authorized-user", "operator")
        self.run_install("--template", "qa-rework")
        self.assertEqual(
            self.api.objects["workflows"][1]["authorized_users"], ["operator"]
        )


if __name__ == "__main__":
    unittest.main()
