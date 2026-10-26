import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from install import install


class InstallationTest(unittest.TestCase):
    def graph(self, name):
        return {
            "name": name,
            "enabled": True,
            "environment": "opensandbox",
            "context_version": 2,
            "entry": "done",
            "max_steps": 2,
            "nodes": [{"id": "done", "name": "done", "kind": "end"}],
            "edges": [],
        }

    def test_upgrade_supplies_actual_revision_and_reentry_keeps_ids(self):
        class API:
            def __init__(self):
                self.graphs = {}
                self.puts = []

            def _json(self, path, value=None, method=None):
                if path == "/api/workflows" and value is None:
                    return list(self.graphs.values())
                if value is None:
                    return copy.deepcopy(self.graphs[path.split("/")[-1]])
                if method == "PUT":
                    self.assert_revision = value["revision"]
                    self.puts.append(value)
                    identity = path.split("/")[-1]
                else:
                    identity = str(len(self.graphs) + 1)
                result = {
                    **value,
                    "id": identity,
                    "revision": value.get("revision", 0) + 1,
                }
                self.graphs[identity] = copy.deepcopy(result)
                return result

            def workflow_runs(self, **kwargs):
                return []

        api = API()
        with tempfile.TemporaryDirectory() as folder:
            manifest = Path(folder) / "manifest.json"
            with patch("install.definition", side_effect=lambda *a: self.graph(a[2])):
                first = install(
                    api,
                    "owner/project",
                    "GIT_REF",
                    "/skill",
                    manifest,
                    model_refs={},
                    authorized_users=["caller"],
                )
                self.assertEqual(
                    install(
                        api,
                        "owner/project",
                        "GIT_REF",
                        "/skill",
                        manifest,
                        model_refs={},
                        authorized_users=["caller"],
                    ),
                    first,
                )
            with patch(
                "install.definition",
                side_effect=lambda *a: self.graph(a[2] + " changed"),
            ):
                second = install(
                    api,
                    "owner/project",
                    "GIT_REF",
                    "/skill",
                    manifest,
                    model_refs={},
                    authorized_users=["caller"],
                )
            self.assertEqual(first, second)
            self.assertEqual([v["revision"] for v in api.puts], [1, 1])
            self.assertEqual(set(json.loads(manifest.read_text())), {"cli", "delegate"})
