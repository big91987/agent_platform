import unittest
from pathlib import Path

from workflows import definition


class ReviewWorkflowTest(unittest.TestCase):
    def test_cli_secret_only_enters_review_and_git_only_preparation(self):
        refs = {
            key: "REVIEW_" + key
            for key in (
                "OCR_LLM_URL",
                "OCR_LLM_MODEL",
                "OCR_LLM_PROTOCOL",
                "OCR_LLM_TOKEN",
            )
        }
        graph = definition("owner/project", "REPO_TOKEN", "cli", model_refs=refs)
        nodes = {n["id"]: n for n in graph["nodes"]}
        self.assertEqual(nodes["review"]["execution"]["env_refs"], refs)
        self.assertEqual(
            nodes["prepare"]["execution"]["env_refs"], {"GITHUB_TOKEN": "REPO_TOKEN"}
        )
        for node in nodes.values():
            if node["kind"] == "execution":
                compile(node["execution"]["code"], node["id"], "exec")

    def test_delegate_uses_official_skill_and_no_credential_refs(self):
        graph = definition(
            "owner/project",
            "REPO_TOKEN",
            "delegate",
            skill=str(Path("/installed/skill")),
        )
        agent = next(n for n in graph["nodes"] if n["id"] == "review")["agent"]
        self.assertFalse(agent["inherit_env"])
        self.assertEqual(agent["skills"], ["/installed/skill"])
        self.assertNotIn("env_refs", agent)
        validate = next(n for n in graph["nodes"] if n["id"] == "validate")
        self.assertIn(
            "harness-output/code-review/result.json", validate["execution"]["artifacts"]
        )

    def test_development_template_routes_only_clean_to_qa_and_findings_back(self):
        from sandbox_delivery import definition as delivery
        from workflows import add_cli_review

        original = delivery("owner/project", "TOKEN")
        graph = add_cli_review(original, {"OCR_LLM_TOKEN": "REVIEW_KEY"})
        nodes = {n["id"]: n for n in graph["nodes"]}
        self.assertNotIn("code_review", {n["id"] for n in original["nodes"]})
        self.assertEqual(
            nodes["tests"]["rule_handoff"]["rules"][0]["target"], "code_review"
        )
        self.assertEqual(
            [
                (r["when"], r["target"])
                for r in nodes["code_review"]["rule_handoff"]["rules"]
            ],
            [("$ == 0", "qa"), ("$ == 10", "development")],
        )
        compile(nodes["code_review"]["execution"]["code"], "embedded-review", "exec")

    def test_cli_inherits_only_explicit_proxy_refs_from_preparation(self):
        from sandbox_delivery import definition as delivery
        from workflows import add_cli_review

        refs = {
            key: "REVIEW_" + key
            for key in (
                "OCR_LLM_URL",
                "OCR_LLM_MODEL",
                "OCR_LLM_PROTOCOL",
                "OCR_LLM_TOKEN",
            )
        }
        for graph in (
            definition(
                "owner/project",
                "GIT_TOKEN",
                "cli",
                model_refs=refs,
                proxy="http://proxy:7890",
            ),
            add_cli_review(
                delivery("owner/project", "GIT_TOKEN", proxy="http://proxy:7890"), refs
            ),
        ):
            node = next(
                n for n in graph["nodes"] if n["id"] in ("review", "code_review")
            )
            actual = node["execution"]["env_refs"]
            self.assertEqual(actual["https_proxy"], "PIPELINE_HTTPS_PROXY")
            self.assertEqual(actual["OCR_LLM_TOKEN"], refs["OCR_LLM_TOKEN"])
            self.assertNotIn("GITHUB_TOKEN", actual)
