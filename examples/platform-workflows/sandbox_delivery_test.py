import unittest

from sandbox_delivery import definition


class DeliveryExampleTest(unittest.TestCase):
    def test_shared_task_environment_keeps_git_credential_out_of_agents_and_tests(self):
        graph = definition(
            "owner/project",
            "PROJECT_GITHUB_TOKEN",
            "http://proxy:7890",
            "localhost,platform",
        )
        nodes = {n["id"]: n for n in graph["nodes"]}
        self.assertEqual(
            nodes["prepare"]["execution"]["env_refs"]["GITHUB_TOKEN"],
            "PROJECT_GITHUB_TOKEN",
        )
        self.assertEqual(
            nodes["publish"]["execution"]["env_refs"]["GITHUB_TOKEN"],
            "PROJECT_GITHUB_TOKEN",
        )
        self.assertNotIn("GITHUB_TOKEN", nodes["tests"]["execution"]["env_refs"])
        for key in ["development", "qa", "report"]:
            self.assertFalse(nodes[key]["agent"]["inherit_env"])
            self.assertNotIn("GITHUB_TOKEN", nodes[key]["agent"]["env"])
            self.assertEqual(
                nodes[key]["agent"]["env"]["no_proxy"], "localhost,platform"
            )
        compile(nodes["prepare"]["execution"]["code"], "prepare", "exec")
        edges = {(e["source"], e["route"]): e["target"] for e in graph["edges"]}
        self.assertEqual(edges["tests", "rule_2"], "development")
        self.assertEqual(edges["qa", "revise"], "development")
        self.assertEqual(edges["qa", "approved"], "approval")
