"""Real local Git repositories exercise publish boundaries; no remote API mock."""

import importlib.util
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location(
    "repository", Path(__file__).with_name("repository.py")
)
repo = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(repo)


class RepositoryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.remote = self.root / "remote.git"
        subprocess.run(
            ["git", "init", "--bare", str(self.remote)], check=True, capture_output=True
        )
        self.workspace = self.root / "workspace"
        subprocess.run(
            ["git", "clone", str(self.remote), str(self.workspace)],
            check=True,
            capture_output=True,
        )
        for key, value in [
            ("user.name", "Workflow Test"),
            ("user.email", "workflow@example.invalid"),
        ]:
            repo.git(self.workspace, "config", key, value)
        repo.git(self.workspace, "checkout", "-b", "main")
        (self.workspace / "README.md").write_text("Example\n")
        repo.git(self.workspace, "add", "README.md")
        repo.git(self.workspace, "commit", "-m", "initial")
        repo.git(self.workspace, "push", "origin", "main")
        self.run = {
            "run_id": "a" * 32,
            "input": "Add a feature",
            "workspace_path": str(self.workspace),
        }

    def test_prepare_and_publish_preserve_main_and_reject_wrong_branch(self):
        repo.prepare(self.workspace, self.run, "main")
        (self.workspace / "feature.txt").write_text("implemented\n")
        docs = self.workspace / "docs/workflow"
        docs.mkdir(parents=True)
        (docs / "qa.md").write_text("Validation evidence\n")
        (docs / "pr.md").write_text("Background and behavior\n")
        repo.publish(self.workspace, self.run)
        self.assertEqual(
            repo.git(self.workspace, "show", "origin/main:README.md"), "Example"
        )
        self.assertEqual(
            repo.git(
                self.workspace, "show", "origin/workflow/" + "a" * 32 + ":feature.txt"
            ),
            "implemented",
        )
        original = repo.git(self.workspace, "rev-parse", "HEAD")
        repo.publish(self.workspace, self.run)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD"), original)
        repo.git(self.workspace, "checkout", "main")
        with self.assertRaises(ValueError):
            repo.publish(self.workspace, self.run)

    def published_task(self):
        repo.prepare(self.workspace, self.run, "main")
        (self.workspace / "feature.txt").write_text("task change\n")
        repo.git(self.workspace, "add", "feature.txt")
        repo.git(self.workspace, "commit", "-m", "task")
        head = repo.git(self.workspace, "rev-parse", "HEAD")
        self.run["previous_results"] = [
            {
                "node_id": "publish",
                "status": "completed",
                "connector_receipt": {
                    "exit_code": 0,
                    "output": json.dumps(
                        {
                            "branch": repo.branch_for(self.run),
                            "head": head,
                            "pushed": True,
                        }
                    ),
                },
            }
        ]
        repo.git(self.workspace, "checkout", "main")
        (self.workspace / "upstream.txt").write_text("upstream change\n")
        repo.git(self.workspace, "add", "upstream.txt")
        repo.git(self.workspace, "commit", "-m", "upstream")
        base = repo.git(self.workspace, "rev-parse", "HEAD")
        repo.git(self.workspace, "push", "origin", "main")
        repo.git(self.workspace, "checkout", repo.branch_for(self.run))
        return head, base

    def test_published_prepare_integrates_base_without_committing_and_is_repeatable(
        self,
    ):
        head, base = self.published_task()
        directory = self.workspace / repo.task_docs(self.run)
        directory.mkdir(parents=True)
        evidence = directory / "blocked.md"
        evidence.write_text("Original native failure\n")
        receipt = repo.prepare(self.workspace, self.run, "main", True)
        self.assertTrue((self.workspace / "upstream.txt").is_file())
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD"), head)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "MERGE_HEAD"), base)
        self.assertEqual(receipt["integration_base"], base)
        self.assertEqual(
            repo.prepare(self.workspace, self.run, "main", True)["integration_base"],
            base,
        )
        self.assertEqual(evidence.read_text(), "Original native failure\n")

    def test_published_prepare_rejects_dirty_product_without_overwriting(self):
        head, _ = self.published_task()
        (self.workspace / "feature.txt").write_text("unfinished product work\n")
        with self.assertRaisesRegex(ValueError, "uncommitted"):
            repo.prepare(self.workspace, self.run, "main")
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD"), head)
        self.assertEqual(
            (self.workspace / "feature.txt").read_text(), "unfinished product work\n"
        )

    def test_unpublished_prepare_continuation_preserves_legacy_behavior(self):
        repo.prepare(self.workspace, self.run, "main")
        (self.workspace / "README.md").write_text("unfinished\n")
        receipt = repo.prepare(self.workspace, self.run, "main")
        self.assertTrue(receipt["continued"])
        self.assertEqual((self.workspace / "README.md").read_text(), "unfinished\n")

    def test_prepare_conflict_preserves_files_and_cannot_publish_unresolved(self):
        head, _ = self.published_task()
        repo.git(self.workspace, "checkout", "main")
        (self.workspace / "feature.txt").write_text("opposite upstream change\n")
        repo.git(self.workspace, "add", "feature.txt")
        repo.git(self.workspace, "commit", "-m", "conflicting upstream")
        base = repo.git(self.workspace, "rev-parse", "HEAD")
        repo.git(self.workspace, "push", "origin", "main")
        repo.git(self.workspace, "checkout", repo.branch_for(self.run))
        receipt = repo.prepare(self.workspace, self.run, "main")
        self.assertEqual(receipt["integration_conflicts"], ["feature.txt"])
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD"), head)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "MERGE_HEAD"), base)
        self.assertEqual(
            repo.git(self.workspace, "diff", "--name-only", "--diff-filter=U"),
            "feature.txt",
        )
        self.assertEqual(
            repo.prepare(self.workspace, self.run, "main")["integration_conflicts"],
            ["feature.txt"],
        )
        docs = self.workspace / "docs/workflow"
        docs.mkdir(parents=True)
        (docs / "qa.md").write_text("Evidence")
        (docs / "pr.md").write_text("Report")
        with self.assertRaisesRegex(ValueError, "conflicts"):
            repo.publish(self.workspace, self.run)
        with self.assertRaises(subprocess.CalledProcessError):
            repo.run_tests(self.workspace, self.run, ["python3", "-c", "pass"])
        (self.workspace / "feature.txt").write_text("resolved by development\n")
        self.assertEqual(
            repo.run_tests(self.workspace, self.run, ["python3", "-c", "pass"]), 0
        )
        self.assertEqual(
            repo.git(self.workspace, "diff", "--name-only", "--diff-filter=U"), ""
        )
        repo.publish(self.workspace, self.run)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD^2"), base)

    def test_conflict_paths_are_literal_and_do_not_stage_unrelated_files(self):
        name = ":(glob)*"
        (self.workspace / name).write_text("initial\n")
        repo.git(self.workspace, "add", ".")
        repo.git(self.workspace, "commit", "-m", "literal filename")
        repo.git(self.workspace, "push", "origin", "main")
        head, _ = self.published_task()
        (self.workspace / name).write_text("task\n")
        repo.git(self.workspace, "--literal-pathspecs", "add", "--", name)
        repo.git(self.workspace, "commit", "-m", "task literal change")
        self.run["previous_results"][0]["connector_receipt"]["output"] = json.dumps(
            {
                "branch": repo.branch_for(self.run),
                "head": repo.git(self.workspace, "rev-parse", "HEAD"),
                "pushed": True,
            }
        )
        repo.git(self.workspace, "checkout", "main")
        (self.workspace / name).write_text("upstream\n")
        repo.git(self.workspace, "--literal-pathspecs", "add", "--", name)
        repo.git(self.workspace, "commit", "-m", "upstream literal change")
        repo.git(self.workspace, "push", "origin", "main")
        repo.git(self.workspace, "checkout", repo.branch_for(self.run))
        self.assertEqual(
            repo.prepare(self.workspace, self.run, "main")["integration_conflicts"],
            [name],
        )
        (self.workspace / name).write_text("resolved\n")
        (self.workspace / "unrelated.txt").write_text("must remain unstaged\n")
        self.assertEqual(
            repo.run_tests(self.workspace, self.run, ["python3", "-c", "pass"]), 0
        )
        self.assertNotIn(
            "unrelated.txt", repo.git(self.workspace, "diff", "--cached", "--name-only")
        )
        self.assertIn(
            "unrelated.txt",
            repo.git(self.workspace, "ls-files", "--others", "--exclude-standard"),
        )

    def test_publication_commits_pending_merge_and_preserves_both_parents(self):
        head, base = self.published_task()
        repo.prepare(self.workspace, self.run, "main")
        docs = self.workspace / "docs/workflow"
        docs.mkdir(parents=True)
        (docs / "qa.md").write_text("Joint evidence")
        (docs / "pr.md").write_text("Joint report")
        receipt = repo.publish(self.workspace, self.run)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD^1"), head)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD^2"), base)
        self.assertEqual(
            repo.git(
                self.workspace, "rev-parse", "origin/" + repo.branch_for(self.run)
            ),
            receipt["head"],
        )
        repo.publish(self.workspace, self.run)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD"), receipt["head"])

    def test_publish_finishes_equal_tree_merge_without_new_content(self):
        repo.prepare(self.workspace, self.run, "main")
        docs = self.workspace / "docs/workflow"
        docs.mkdir(parents=True)
        (docs / "qa.md").write_text("Evidence")
        (docs / "pr.md").write_text("Report")
        repo.git(self.workspace, "add", "docs")
        repo.git(self.workspace, "commit", "-m", "task documents")
        head = repo.git(self.workspace, "rev-parse", "HEAD")
        self.run["previous_results"] = [
            {
                "node_id": "publish",
                "status": "completed",
                "connector_receipt": {
                    "exit_code": 0,
                    "output": json.dumps(
                        {
                            "branch": repo.branch_for(self.run),
                            "head": head,
                            "pushed": True,
                        }
                    ),
                },
            }
        ]
        repo.git(self.workspace, "checkout", "main")
        repo.git(self.workspace, "cherry-pick", "--no-commit", head)
        repo.git(self.workspace, "commit", "-m", "same upstream content")
        base = repo.git(self.workspace, "rev-parse", "HEAD")
        repo.git(self.workspace, "push", "origin", "main")
        repo.git(self.workspace, "checkout", repo.branch_for(self.run))
        repo.prepare(self.workspace, self.run, "main")
        self.assertEqual(repo.git(self.workspace, "diff", "HEAD", "--name-only"), "")
        receipt = repo.publish(self.workspace, self.run)
        self.assertNotEqual(receipt["head"], head)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD^2"), base)

    def test_failed_publication_does_not_fall_back_to_unpublished_continuation(self):
        self.published_task()
        self.run["previous_results"].append(
            {
                "node_id": "publish",
                "status": "failed",
                "connector_receipt": {"exit_code": 1},
            }
        )
        (self.workspace / "feature.txt").write_text("unfinished\n")
        with self.assertRaisesRegex(ValueError, "publication"):
            repo.prepare(self.workspace, self.run, "main")

    def test_staged_task_evidence_is_rejected_before_merge_without_losing_content(self):
        head, _ = self.published_task()
        directory = self.workspace / repo.task_docs(self.run)
        directory.mkdir(parents=True)
        (directory / "blocked.md").write_text("Failure evidence")
        repo.git(self.workspace, "add", str(directory.relative_to(self.workspace)))
        with self.assertRaisesRegex(ValueError, "staged"):
            repo.prepare(self.workspace, self.run, "main")
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD"), head)
        self.assertEqual((directory / "blocked.md").read_text(), "Failure evidence")

    def test_task_artifacts_use_skill_filenames_and_do_not_overwrite_other_runs(self):
        receipt = repo.prepare(self.workspace, self.run, "main", True)
        directory = self.workspace / receipt["document_root"]
        directory.mkdir(parents=True)
        other = self.workspace / "docs/workflow/runs" / ("b" * 32)
        other.mkdir()
        (other / "acceptance-matrix.md").write_text("Other task evidence")
        artifact = directory / "verification" / "acceptance-matrix.md"
        artifact.parent.mkdir()
        artifact.write_text("Actual Skill report")
        (directory / "pr.md").write_text("PR delivery")
        self.run["previous_results"] = [
            {
                "node_id": "qa",
                "status": "completed",
                "result": {
                    "route": "next",
                    "artifacts": [str(artifact.relative_to(self.workspace))],
                },
            }
        ]
        repo.publish(self.workspace, self.run, True)
        self.assertEqual(
            (other / "acceptance-matrix.md").read_text(), "Other task evidence"
        )
        self.assertFalse((directory / "qa.md").exists())
        self.assertEqual(
            repo.prepare(self.workspace, self.run, "main", True)["document_root"],
            receipt["document_root"],
        )
        self.run["previous_results"][0]["result"]["artifacts"] = [
            str((other / "acceptance-matrix.md").relative_to(self.workspace))
        ]
        with self.assertRaisesRegex(ValueError, "this task"):
            repo.publish(self.workspace, self.run, True)

    def test_two_task_branches_merge_without_document_conflicts(self):
        branches = []
        for identity in ("a" * 32, "b" * 32):
            repo.git(self.workspace, "checkout", "main")
            run = {**self.run, "run_id": identity}
            prepared = repo.prepare(self.workspace, run, "main", True)
            directory = self.workspace / prepared["document_root"]
            directory.mkdir(parents=True)
            report = directory / "acceptance-matrix.md"
            report.write_text("Evidence for " + identity)
            (directory / "pr.md").write_text("Delivery for " + identity)
            run["previous_results"] = [
                {
                    "node_id": "qa",
                    "status": "completed",
                    "result": {
                        "route": "next",
                        "artifacts": [str(report.relative_to(self.workspace))],
                    },
                }
            ]
            repo.publish(self.workspace, run, True)
            branches.append(prepared["branch"])
        repo.git(self.workspace, "checkout", "main")
        for branch in branches:
            repo.git(self.workspace, "merge", "--no-edit", branch)
        for identity in ("a" * 32, "b" * 32):
            self.assertEqual(
                (
                    self.workspace
                    / "docs/workflow/runs"
                    / identity
                    / "acceptance-matrix.md"
                ).read_text(),
                "Evidence for " + identity,
            )
        self.assertEqual(
            repo.git(self.workspace, "diff", "--name-only", "--diff-filter=U"), ""
        )

    def test_publication_rejects_missing_or_newer_failed_qa(self):
        repo.prepare(self.workspace, self.run, "main", True)
        with self.assertRaisesRegex(ValueError, "QA handoff"):
            repo.publish(self.workspace, self.run, True)
        self.run["previous_results"] = [
            {
                "node_id": "qa",
                "status": "completed",
                "result": {"route": "next", "artifacts": ["old.md"]},
            },
            {
                "node_id": "qa",
                "status": "completed",
                "result": {"route": "development"},
            },
        ]
        with self.assertRaisesRegex(ValueError, "QA handoff"):
            repo.publish(self.workspace, self.run, True)

    def test_dirty_initial_workspace_and_secret_are_rejected(self):
        (self.workspace / "leftover.txt").write_text("preserve me")
        with self.assertRaises(ValueError):
            repo.prepare(self.workspace, self.run, "main")
        self.assertEqual((self.workspace / "leftover.txt").read_text(), "preserve me")
        (self.workspace / "leftover.txt").unlink()
        repo.prepare(self.workspace, self.run, "main")
        (self.workspace / ".env").write_text("PASSWORD=private")
        with self.assertRaises(ValueError):
            repo.check_publish_files(self.workspace)
        self.assertEqual(
            repo.git(self.workspace, "diff", "--cached", "--name-only"), ""
        )

    def test_existing_trellis_version_is_checked_before_project_scripts(self):
        trellis = self.workspace / ".trellis"
        (trellis / "scripts").mkdir(parents=True)
        script = trellis / "scripts/get_context.py"
        script.write_text("raise RuntimeError('outdated script must not run')\n")
        (trellis / ".version").write_text("0.5.0")
        cli = self.root / "trellis"
        cli.write_text("#!/usr/bin/env python3\nprint('0.6.15')\n")
        cli.chmod(0o700)
        with self.assertRaisesRegex(ValueError, "project.*version"):
            repo.prepare_trellis(self.workspace, str(cli), "0.6.15")
        self.assertEqual((trellis / ".version").read_text(), "0.5.0")

    def test_trellis_project_context_does_not_receive_git_write_token(self):
        trellis = self.workspace / ".trellis"
        (trellis / "scripts").mkdir(parents=True)
        (trellis / ".version").write_text("0.6.15")
        (trellis / "scripts/get_context.py").write_text(
            "import os\nassert 'GH_TOKEN' not in os.environ\n"
        )
        cli = self.root / "trellis"
        cli.write_text("#!/usr/bin/env python3\nprint('0.6.15')\n")
        cli.chmod(0o700)
        with patch.dict(os.environ, {"GH_TOKEN": "unit-test-only"}):
            result = repo.prepare_trellis(self.workspace, str(cli), "0.6.15")
            self.assertEqual(os.environ["GH_TOKEN"], "unit-test-only")
        self.assertFalse(result["initialized"])
        self.assertEqual(result["version"], "0.6.15")

    def test_remote_identity_normalizes_only_github_ssh_and_https(self):
        self.assertEqual(
            repo.remote_repository("git@github.com:owner/repo.git"), "owner/repo"
        )
        self.assertEqual(
            repo.remote_repository("https://github.com/owner/repo.git"), "owner/repo"
        )
        for value in [
            "https://secret@github.com/owner/repo",
            "https://evil.example/owner/repo",
            "../local",
        ]:
            with self.assertRaises(ValueError):
                repo.remote_repository(value)

    def test_material_drift_blocks_publish_and_private_inputs_never_enter_commit(self):
        import hashlib
        import json

        import materials
        from materials_test import REPOSITORY, bundle

        raw = bundle()
        self.run["parameters"] = {
            "material": json.dumps(
                {
                    "url": "https://api.github.com/repos/example/product/releases/assets/123",
                    "sha256": hashlib.sha256(raw).hexdigest(),
                    "version": "v1",
                }
            )
        }
        repo.prepare(self.workspace, self.run, "main")
        with patch.object(
            materials, "download", side_effect=lambda d, p: p.write_bytes(raw)
        ):
            receipt = repo.prepare_material(self.workspace, self.run, REPOSITORY)
        docs = self.workspace / "docs/workflow"
        docs.mkdir(parents=True)
        (docs / "qa.md").write_text("Independent QA")
        (docs / "pr.md").write_text("Delivery")
        original = repo.git(self.workspace, "rev-parse", "HEAD")
        source = self.workspace / receipt["root"] / "docs/prd.md"
        source.write_text("modified input")
        with self.assertRaises(ValueError):
            repo.publish(self.workspace, self.run, materials=True)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD"), original)
        source.write_bytes(b"Actual requirements")
        repo.publish(self.workspace, self.run, materials=True)
        committed = repo.git(
            self.workspace, "ls-tree", "-r", "--name-only", "HEAD"
        ).splitlines()
        self.assertIn(".gitignore", committed)
        self.assertFalse(any(x.startswith(".workflow-input/") for x in committed))


class MaterialCommandTest(unittest.TestCase):
    def test_fixed_tests_verify_input_before_running_project_command(self):
        import json
        import sys

        import materials
        from materials_test import REPOSITORY, bundle

        with tempfile.TemporaryDirectory() as tmp:
            workspace = Path(tmp)
            raw = bundle()
            import hashlib

            run = {
                "run_id": "a" * 32,
                "parameters": {
                    "material": json.dumps(
                        {
                            "url": "https://api.github.com/repos/example/product/releases/assets/123",
                            "sha256": hashlib.sha256(raw).hexdigest(),
                            "version": "v1",
                        }
                    )
                },
            }
            with patch.object(
                materials, "download", side_effect=lambda d, p: p.write_bytes(raw)
            ):
                receipt = materials.install_material(workspace, run, REPOSITORY)
            cmd = [
                sys.executable,
                "-c",
                "from pathlib import Path; Path('test-effects').write_text('ran')",
            ]
            (workspace / receipt["root"] / "docs/prd.md").write_text("changed")
            with self.assertRaises(ValueError):
                repo.run_tests(workspace, run, cmd)
            self.assertFalse((workspace / "test-effects").exists())
            (workspace / receipt["root"] / "docs/prd.md").write_bytes(
                b"Actual requirements"
            )
            self.assertEqual(repo.run_tests(workspace, run, cmd), 0)
            self.assertEqual((workspace / "test-effects").read_text(), "ran")
            self.assertEqual(
                repo.run_tests(
                    workspace, run, [sys.executable, "-c", "raise SystemExit(7)"]
                ),
                7,
            )


if __name__ == "__main__":
    unittest.main()
