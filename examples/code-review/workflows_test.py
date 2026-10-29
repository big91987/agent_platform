import shlex
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
                compile(
                    shlex.split(node["execution"]["command"])[-1], node["id"], "exec"
                )

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
        compile(
            shlex.split(nodes["code_review"]["execution"]["command"])[-1],
            "embedded-review",
            "exec",
        )

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

    def test_explicit_review_upgrade_keeps_other_roles_permissions_and_routes(self):
        import copy

        from sandbox_delivery import definition as delivery
        from workflows import add_cli_review, upgrade_cli_review

        original = add_cli_review(
            delivery("owner/project", "TOKEN"), {"OCR_LLM_TOKEN": "KEY"}
        )
        review = next(n for n in original["nodes"] if n["id"] == "code_review")
        review["execution"]["command"] = "previous reviewed program"
        review["execution_timeout_seconds"] = 2100
        review["name"] = "Team review"
        review["execution"]["env_refs"]["https_proxy"] = "TEAM_PROXY"
        original["revision"] = 15
        frozen = copy.deepcopy(original)
        upgraded = upgrade_cli_review(original)
        actual = next(n for n in upgraded["nodes"] if n["id"] == "code_review")
        self.assertNotEqual(
            actual["execution"]["command"], review["execution"]["command"]
        )
        compile(
            shlex.split(actual["execution"]["command"])[-1], "upgraded-review", "exec"
        )
        actual["execution"]["command"] = review["execution"]["command"]
        actual["execution_timeout_seconds"] = review["execution_timeout_seconds"]
        self.assertEqual(upgraded, original)
        self.assertEqual(original, frozen)
        for graph in (delivery("owner/project", "TOKEN"), dict(original, nodes=[])):
            with self.assertRaises(ValueError):
                upgrade_cli_review(graph)

    def test_pr_module_cannot_run_during_trusted_review_imports(self):
        import subprocess
        import tempfile

        from sandbox_delivery import python_command
        from workflows import program

        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / "hashlib.py").write_text(
                "open('pr-executed', 'w').write('executed')\nraise RuntimeError('PR code ran')\n"
            )
            (root / "sitecustomize.py").write_text(
                "open('startup-executed', 'w').write('executed')\n"
            )
            for code in (
                program("print(hashlib.sha256(b'fixture').hexdigest())"),
                "import hashlib; print(hashlib.sha256(b'fixture').hexdigest())",
            ):
                result = subprocess.run(
                    python_command(code),
                    shell=True,
                    cwd=root,
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertFalse((root / "pr-executed").exists())
                self.assertFalse((root / "startup-executed").exists())
            graph = definition(
                "owner/project",
                "TOKEN",
                "cli",
                model_refs={
                    key: "REVIEW_" + key
                    for key in (
                        "OCR_LLM_URL",
                        "OCR_LLM_TOKEN",
                        "OCR_LLM_MODEL",
                        "OCR_LLM_PROTOCOL",
                    )
                },
            )
            for node in graph["nodes"]:
                if node["kind"] != "execution":
                    continue
                subprocess.run(
                    node["execution"]["command"],
                    shell=True,
                    cwd=root,
                    capture_output=True,
                )
                self.assertFalse((root / "pr-executed").exists())
                self.assertFalse((root / "startup-executed").exists())
            text = "$(touch shell-executed) `touch shell-executed` ' \""
            result = subprocess.run(
                python_command("print(" + repr(text) + ")"),
                shell=True,
                cwd=root,
                capture_output=True,
                text=True,
            )
            self.assertEqual(result.stdout.strip(), text)
            self.assertFalse((root / "shell-executed").exists())

    def test_delegate_keeps_explicit_proxy_without_git_or_model_secrets(self):
        graph = definition(
            "owner/project",
            "TOKEN",
            "delegate",
            skill="/official/skill",
            proxy="http://proxy:7890",
        )
        agent = next(node for node in graph["nodes"] if node["id"] == "review")["agent"]
        self.assertEqual(agent["env"]["https_proxy"], "http://proxy:7890")
        self.assertEqual(agent["env"]["HTTPS_PROXY"], "http://proxy:7890")
        self.assertNotIn("GITHUB_TOKEN", agent["env"])
        self.assertNotIn("OCR_LLM_TOKEN", agent["env"])
        self.assertFalse(agent["inherit_env"])
