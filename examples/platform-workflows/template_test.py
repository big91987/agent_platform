"""Executable graph contracts; Agent decisions are verified in real journeys."""

import json
import unittest
from pathlib import Path


class DeliveryRoutesTest(unittest.TestCase):
    def setUp(self):
        self.graph = json.loads(
            Path(__file__).with_name("software-delivery.json").read_text()
        )

    def test_agent_exit_modes_are_explicit_and_exclusive(self):
        for name in ("software-delivery", "qa-rework", "collaboration-check"):
            graph = json.loads(Path(__file__).with_name(name + ".json").read_text())
            for node in graph["nodes"]:
                if node["kind"] != "agent":
                    continue
                with self.subTest(template=name, node=node["id"]):
                    self.assertEqual(node.get("exit_mode"), "handoff")
                    self.assertTrue(
                        all(
                            edge["mode"] == "handoff"
                            for edge in graph["edges"]
                            if edge["source"] == node["id"]
                        )
                    )

    def reachable(self, excluded=()):
        visited, pending = set(), [self.graph["entry"]]
        while pending:
            node = pending.pop()
            if node in visited or node in excluded:
                continue
            visited.add(node)
            pending.extend(
                edge["target"] for edge in self.graph["edges"] if edge["source"] == node
            )
        return visited

    def test_scoped_repair_can_reach_delivery_without_requirements_or_design(self):
        reached = self.reachable({"requirements", "design"})
        self.assertIn("development", reached, "No true repair entry exists")
        self.assertIn("done", reached, "Repair must still deliver through QA")

    def test_shortcut_cannot_skip_repository_setup_or_independent_verification(self):
        for required in (
            "prepare",
            "issue",
            "development",
            "tests",
            "qa",
            "report",
            "publish",
            "pr",
        ):
            with self.subTest(required=required):
                self.assertNotIn("done", self.reachable({required}))
        # Direct graph starts must not silently bypass preparation and Issue association.
        self.assertFalse(self.graph["start_nodes"])


if __name__ == "__main__":
    unittest.main()
