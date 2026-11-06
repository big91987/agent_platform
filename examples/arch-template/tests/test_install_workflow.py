import copy
import importlib.util
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
spec = importlib.util.spec_from_file_location(
    "architecture_install", ROOT / "install_workflow.py"
)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class InstallTests(unittest.TestCase):
    def test_behavior_fields_cannot_disappear_during_upgrade(self):
        graph = {
            "name": "check",
            "enabled": True,
            "environment": "opensandbox",
            "context_version": 2,
            "authorized_users": [],
            "entry": "prepare",
            "max_steps": 10,
            "nodes": [],
            "edges": [],
            "hooks": [],
            "start_nodes": [],
        }
        for key, value in [
            ("hooks", [{"event": "run.completed"}]),
            ("start_nodes", ["other"]),
        ]:
            edited = copy.deepcopy(graph)
            edited[key] = value
            self.assertNotEqual(module.comparable(graph), module.comparable(edited))
        graph.pop("hooks")
        graph["start_nodes"] = None
        self.assertEqual(module.comparable(graph)["hooks"], [])

    def test_active_run_beyond_first_page_blocks_upgrade(self):
        class Client:
            def workflow_runs(self, *, workflow_id, before=""):
                return (
                    [{"id": "new", "status": "completed"}]
                    if not before
                    else (
                        [{"id": "old", "status": "waiting"}] if before == "new" else []
                    )
                )

        self.assertTrue(module.has_active_runs(Client(), "inspection"))
