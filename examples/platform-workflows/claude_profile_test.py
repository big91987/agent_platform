import json
import tempfile
import unittest
from pathlib import Path

from asset_snapshot import snapshot
from claude_profile import configure, role_skills
from repository import ReviewedSourceChanged, git, task_docs, verify_reviewed_tree
from review_pair import decision, prepared_head


class ClaudeProfileTest(unittest.TestCase):
    def test_dual_review_cannot_pass_incomplete_or_changed_source(self):
        self.assertEqual(decision(0, 0, "no_findings", True), 0)
        self.assertEqual(decision(10, 0, "no_findings", True), 10)
        self.assertEqual(decision(0, 1, "findings", True), 10)
        for args in (
            (20, 0, "no_findings", True),
            (0, 2, "no_findings", True),
            (0, 0, "not_run", True),
            (0, 0, "no_findings", False),
        ):
            with self.subTest(args=args):
                self.assertEqual(decision(*args), 20)

    def test_role_flow_requires_tests_reviews_qa_and_delivery_gate(self):
        graph = json.loads(
            Path(__file__).with_name("software-delivery.json").read_text()
        )
        modified = configure(graph)
        self.assertIn("intake", [n["id"] for n in graph["nodes"]])
        self.assertNotIn("intake", [n["id"] for n in modified["nodes"]])
        for blocked in (
            "requirements",
            "design",
            "development",
            "tests",
            "code_review",
            "qa_plan",
            "qa_plan_review",
            "qa",
            "approval",
            "publish_check",
        ):
            seen = set()
            pending = [modified["entry"]]
            while pending:
                n = pending.pop()
                if n in seen or n == blocked:
                    continue
                seen.add(n)
                pending.extend(
                    e["target"] for e in modified["edges"] if e["source"] == n
                )
            self.assertNotIn("done", seen, blocked)
        skills = role_skills(Path("/roles"), Path("/qa"))
        self.assertEqual(skills["development"], [])
        self.assertEqual(skills["report"], [])
        self.assertEqual(
            skills["requirements"],
            [
                Path("/roles/defining-platform-products-cn"),
                Path("/roles/batch-grill-me"),
            ],
        )

    def test_reviewed_tree_accepts_evidence_but_rejects_source_changes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            git(root, "init", "-q")
            source = root / "app.py"
            source.write_text("pass\n")
            git(root, "add", "--all")
            candidate = git(root, "write-tree")
            run = {"run_id": "a" * 32, "previous_results": []}
            directory = task_docs(run)
            evidence = directory + "/evidence.html"
            plan = directory + "/qa-plan-1.md"
            assessment = directory + "/qa-plan-review-1.md"
            run["previous_results"] = [
                {
                    "node_id": "code_review",
                    "status": "completed",
                    "connector_receipt": {
                        "exit_code": 0,
                        "output": json.dumps(
                            {
                                "candidate": candidate,
                                "exit_code": 0,
                                "source_unchanged": True,
                            }
                        ),
                    },
                },
                {
                    "node": "qa_plan",
                    "status": "completed",
                    "result": {"route": "next", "artifacts": [plan]},
                },
                {
                    "node": "qa_plan_review",
                    "status": "completed",
                    "result": {"route": "next", "artifacts": [assessment]},
                },
                {
                    "node": "qa",
                    "status": "completed",
                    "result": {"route": "next", "artifacts": [evidence]},
                },
            ]
            (root / directory).mkdir(parents=True)
            for name in (evidence, plan, assessment, directory + "/pr.md"):
                (root / name).write_text("Evidence")
            git(root, "add", "--all")
            verify_reviewed_tree(root, run, git(root, "write-tree"))
            # Development fixed a QA defect and a fresh reviewed candidate
            # already includes the first round's plan and review evidence.
            specification = directory + "/requirements.md"
            (root / specification).write_text("Approved requirements")
            source.write_text("print('fixed')\n")
            git(root, "add", "--all")
            next_candidate = git(root, "write-tree")
            next_round = [
                {
                    "node_id": "code_review",
                    "status": "completed",
                    "connector_receipt": {
                        "exit_code": 0,
                        "output": json.dumps(
                            {
                                "candidate": next_candidate,
                                "exit_code": 0,
                                "source_unchanged": True,
                            }
                        ),
                    },
                },
            ]
            for node, name in (
                ("qa_plan", "qa-plan-2.md"),
                ("qa_plan_review", "qa-plan-review-2.md"),
                ("qa", "evidence-2.html"),
            ):
                artifact = directory + "/" + name
                (root / artifact).write_text("Second round evidence")
                next_round.append(
                    {
                        "node": node,
                        "status": "completed",
                        "result": {"route": "next", "artifacts": [artifact]},
                    }
                )
            run["previous_results"].extend(next_round)
            git(root, "add", "--all")
            tree = git(root, "write-tree")
            verify_reviewed_tree(root, run, tree)
            verdict = next_round[2]["result"]
            verdict["route"] = "revise"
            with self.assertRaisesRegex(ValueError, "qa_plan_review"):
                verify_reviewed_tree(root, run, tree)
            verdict["route"] = "next"
            verdict["artifacts"].append(specification)
            (root / specification).write_text("Modified after code review")
            git(root, "add", "--all")
            with self.assertRaisesRegex(ReviewedSourceChanged, "requirements.md"):
                verify_reviewed_tree(root, run, git(root, "write-tree"))
            (root / specification).write_text("Approved requirements")
            source.chmod(0o755)
            git(root, "add", "--all")
            with self.assertRaisesRegex(ReviewedSourceChanged, "app.py"):
                verify_reviewed_tree(root, run, git(root, "write-tree"))
            source.chmod(0o644)
            (root / directory / "unlisted.json").write_text("{}")
            git(root, "add", "--all")
            with self.assertRaisesRegex(ReviewedSourceChanged, "unlisted.json"):
                verify_reviewed_tree(root, run, git(root, "write-tree"))

    def test_install_retains_asset_versions_and_detects_local_drift(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "skill"
            source.mkdir()
            (source / "SKILL.md").write_text("first")
            old, previous = snapshot(source, root / "installed", False)
            (source / "SKILL.md").write_text("second")
            with self.assertRaisesRegex(ValueError, "--upgrade"):
                snapshot(source, root / "installed", False, previous)
            new, _ = snapshot(source, root / "installed", True, previous)
            self.assertEqual((old / "SKILL.md").read_text(), "first")
            self.assertEqual((new / "SKILL.md").read_text(), "second")
            (new / "SKILL.md").write_text("drift")
            with self.assertRaisesRegex(ValueError, "recorded digest"):
                snapshot(source, root / "installed", True, previous)

    def test_review_uses_completed_platform_receipt_not_agent_result(self):
        row = {
            "node_id": "prepare",
            "status": "completed",
            "result": {"head": "b" * 40},
            "connector_receipt": {
                "exit_code": 0,
                "output": json.dumps({"head": "a" * 40}),
            },
        }
        self.assertEqual(prepared_head({"previous_results": [row]}), "a" * 40)
        row["connector_receipt"]["log"] = {"truncated": True}
        with self.assertRaises(ValueError):
            prepared_head({"previous_results": [row]})
