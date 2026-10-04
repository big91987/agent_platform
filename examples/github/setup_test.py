"""Registration isolation: no live platform, credentials or Agent executions."""

import contextlib
import copy
import io
import json
import os
import runpy
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("setup.py")


class SetupTest(unittest.TestCase):
    def test_two_repositories_and_legacy_keep_distinct_agent_and_tool_bindings(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            agents = {}
            servers = {}

            def request(req, timeout):
                path = req.full_url.removeprefix("http://platform.invalid")
                data = json.loads(req.data) if req.data else None
                if path == "/api/login":
                    value = {}
                elif path == "/api/tool-servers":
                    servers[data["id"]] = data
                    value = data
                elif path.endswith("/discover"):
                    value = {}
                elif path == "/api/agents" and data is None:
                    value = list(agents.values())
                elif path == "/api/agents":
                    value = {**data, "id": "created-" + str(len(agents))}
                    agents[value["id"]] = value
                elif path.startswith("/api/agents/"):
                    value = data
                    agents[value["id"]] = value
                else:
                    self.fail("unexpected request: " + path)
                return io.BytesIO(json.dumps(value).encode())

            def register(path, *options):
                with (
                    patch.object(
                        sys, "argv", [str(SCRIPT), "--config", str(path), *options]
                    ),
                    patch.dict(os.environ, {"PLATFORM_ADMIN_PASSWORD": "test"}),
                    patch("urllib.request.build_opener") as opener,
                    contextlib.redirect_stdout(io.StringIO()),
                ):
                    opener.return_value.open.side_effect = request
                    try:
                        runpy.run_path(str(SCRIPT), run_name="__main__")
                    except SystemExit as e:
                        self.assertEqual(e.code, 0)
                return json.loads(path.read_text())["pipeline"]

            expected = {}
            for namespace in ("owner-alpha", "owner-beta", ""):
                key = namespace or "legacy"
                agents[key] = {"id": key, "name": "initial " + key, "tool_servers": []}
                path = root / (key + ".json")
                path.write_text(
                    json.dumps(
                        {
                            "base_url": "http://platform.invalid",
                            "agent_id": key,
                            "pipeline": {
                                "namespace": namespace,
                                "registry": str(root / key / "registry"),
                                "workspaces": str(root / key / "workspaces"),
                                "skills_root": str(root / "skills"),
                                "python": sys.executable,
                            },
                        }
                    )
                )
                first = register(path)
                stable = copy.deepcopy(agents)
                again = register(path)
                self.assertEqual(first["agents"], again["agents"])
                self.assertEqual(stable, agents)
                expected[key] = first["agents"]
                prefix = namespace + "-" if namespace else ""
                for stage in ("requirements", "design", "development", "qa"):
                    agent = agents[first["agents"][stage]]
                    self.assertEqual(
                        agent["tool_servers"][0]["server_id"],
                        prefix + "pipeline-" + stage,
                    )
                    self.assertIn(
                        str(root / key / "registry"),
                        servers[prefix + "pipeline-" + stage]["connection"]["args"],
                    )
                review = agents[first["agents"]["review"]]
                self.assertEqual(
                    review["tool_servers"],
                    [
                        {
                            "server_id": prefix + "browser-review",
                            "tools": ["check", "verify"],
                            "approvals": {"check": "auto", "verify": "auto"},
                        }
                    ],
                )
                self.assertIn(
                    "review", servers[prefix + "browser-review"]["connection"]["args"]
                )
                # Upgrade the former empty Review binding without overwriting customization.
                review.pop("tool_servers")
                review["instructions"] = "owner custom review policy"
                qa = agents[first["agents"]["qa"]]
                qa["tool_servers"][1]["approvals"]["verify"] = "confirm"
                register(path, "--tools-only")
                self.assertEqual(review["instructions"], "owner custom review policy")
                upgraded = agents[first["agents"]["review"]]
                self.assertEqual(
                    upgraded["tool_servers"][0]["tools"], ["check", "verify"]
                )
                self.assertEqual(
                    agents[first["agents"]["qa"]]["tool_servers"][1]["approvals"][
                        "verify"
                    ],
                    "confirm",
                )
                stable = copy.deepcopy(agents)
                register(path, "--tools-only")
                self.assertEqual(stable, agents)
            self.assertEqual(len(agents), 15)
            self.assertEqual(len(servers), 24)
            self.assertEqual(
                len({i for mapping in expected.values() for i in mapping.values()}), 15
            )
