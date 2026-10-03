"""Boundary tests for the external GitHub integration, not platform business rules."""

import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import pipeline
from pipeline import (
    delivery_token,
    load_handoff,
    product_digest,
    require_verified_product,
    run,
)
from verify import regression_plan


class HandoffTest(unittest.TestCase):
    def test_receiver_recovers_missing_run_handle_without_changing_frozen_content(self):
        with tempfile.TemporaryDirectory() as temp:
            file = Path(temp) / "result.json"
            value = {
                "delivery": "uncertain",
                "sha256": "frozen",
                "documents": {"prd.md": "old"},
            }
            pipeline.save(file, value)
            pipeline.recover_run_handle(file, value, "owner/repo", "123")
            recovered = json.loads(file.read_text())
            self.assertEqual(recovered["run_id"], 123)
            self.assertEqual(recovered["delivery"], "accepted")
            self.assertEqual(recovered["sha256"], "frozen")
            self.assertEqual(recovered["documents"], {"prd.md": "old"})
            pipeline.recover_run_handle(file, recovered, "owner/repo", "124")
            self.assertEqual(json.loads(file.read_text())["run_id"], 123)

    def test_qa_rework_preserves_sessions_and_rejects_stale_callbacks(self):
        for source, target in (
            (s, t)
            for s in pipeline.STAGES
            for t in pipeline.STAGES[: pipeline.STAGES.index(s)]
        ):
            with (
                self.subTest(source=source, target=target),
                tempfile.TemporaryDirectory() as temp,
            ):
                root = Path(temp)
                (root / "qa.md").write_text("FAIL: actual product defect")
                receipt = root / (source + "-result.json")
                pipeline.save(
                    receipt,
                    {
                        "sha256": "abc",
                        "handoff": {"summary": "Defect", "target_stage": target},
                        "documents": {"qa.md": "FAIL: actual product defect"},
                    },
                )
                task = {
                    "stages": {
                        "design": {"conversation_id": "original-design"},
                        "qa": {"conversation_id": "original-qa"},
                    }
                }
                args = (
                    task,
                    root,
                    target,
                    source,
                    str(receipt),
                    "abc",
                    root,
                    lambda *a: None,
                )
                original = json.loads(receipt.read_text())
                missing_target = json.loads(receipt.read_text())
                missing_target["handoff"].pop("target_stage")
                pipeline.save(receipt, missing_target)
                with self.assertRaises(ValueError):
                    pipeline.accept_handoff(
                        task,
                        root,
                        "report",
                        source,
                        str(receipt),
                        "abc",
                        root,
                        lambda *a: None,
                    )
                pipeline.save(receipt, original)
                pipeline.accept_handoff(*args)
                self.assertEqual(task["round"], "1")
                self.assertEqual(task["active_stage"], target)
                self.assertEqual(
                    task["stages"]["design"]["conversation_id"], "original-design"
                )
                self.assertEqual(
                    pipeline.result_path(root, task, target).name,
                    target + "-1-result.json",
                )
                # A retried dispatch must not reset the iteration or reject docs
                # legitimately updated by the recipient since acceptance.
                (root / "qa.md").write_text("next revision")
                pipeline.accept_handoff(*args)
                task["active_stage"] = "qa"
                task["transition"] = {
                    "file": "new-result",
                    "sha256": "new",
                    "stage": "qa",
                }
                with self.assertRaises(ValueError):
                    pipeline.accept_handoff(*args)

    def test_review_pr_resolves_registered_issue_and_rejects_wrong_checkout(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            workspace = root / "work"
            workspace.mkdir()
            run(["git", "init", "-q", str(workspace)])
            run(
                [
                    "git",
                    "-c",
                    "user.name=Test",
                    "-c",
                    "user.email=test@example.invalid",
                    "commit",
                    "--allow-empty",
                    "-qm",
                    "base",
                ],
                workspace,
            )
            run(["git", "checkout", "-qb", "codex/issue-82-platform"], workspace)
            settings = {"registry": str(root / "registry")}
            task = {"workspace": str(workspace), "branch": "codex/issue-82-platform"}
            pipeline.save(root / "registry/issues/82.json", task)
            pr = {
                "html_url": "https://github.com/owner/product/pull/85",
                "state": "open",
                "draft": False,
                "head": {
                    "repo": {"full_name": "owner/product"},
                    "ref": task["branch"],
                    "sha": run(["git", "rev-parse", "HEAD"], workspace),
                },
            }
            receipt = pipeline.task_directory(settings, workspace) / "delivery.json"
            pipeline.save(receipt, {"pr_url": pr["html_url"]})
            with patch.object(pipeline, "github", return_value=pr):
                self.assertEqual(
                    pipeline.issue_for_review(settings, "owner/product", 85), 82
                )
                for field, value in (
                    ("sha", "wrong-head"),
                    ("ref", "different-task"),
                    ("repo", {"full_name": "outside/fork"}),
                ):
                    previous = pr["head"][field]
                    pr["head"][field] = value
                    with self.assertRaises(ValueError):
                        pipeline.issue_for_review(settings, "owner/product", 85)
                    pr["head"][field] = previous
                (workspace / "app").mkdir()
                (workspace / "app/new.js").write_text("not in the PR")
                with self.assertRaisesRegex(ValueError, "uncommitted"):
                    pipeline.issue_for_review(settings, "owner/product", 85)
                (workspace / "app/new.js").unlink()
                receipt.unlink()
                with self.assertRaisesRegex(ValueError, "registered"):
                    pipeline.issue_for_review(settings, "owner/product", 85)

    def test_old_manual_verification_cannot_restart_after_qa_handoff(self):
        task = {
            "stages": {"development": {}},
            "verification_run": "123",
            "round": "124",
            "active_stage": "design",
        }
        with self.assertRaisesRegex(ValueError, "Stale"):
            pipeline.start_reverification(task, None, "123")

    def test_handoff_cannot_skip_qa_or_choose_an_unregistered_route(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "dev.md").write_text("done")
            receipt = root / "development-result.json"
            pipeline.save(
                receipt,
                {
                    "sha256": "abc",
                    "handoff": {"summary": "done", "target_stage": "report"},
                    "documents": {"dev.md": "done"},
                },
            )
            task = {"stages": {"development": {"conversation_id": "dev"}}}
            with self.assertRaises(ValueError):
                pipeline.accept_handoff(
                    task,
                    root,
                    "report",
                    "development",
                    str(receipt),
                    "abc",
                    root,
                    lambda *a: None,
                )
            self.assertNotIn("round", task)

    def test_product_regression_plan_replaces_legacy_without_empty_fallback(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            legacy = root / ".harness/reading-core.json"
            legacy.parent.mkdir()
            legacy.write_text('[{"action":"reload"}]')
            self.assertEqual(regression_plan(root), legacy)
            product = root / "tests/browser/core.json"
            product.parent.mkdir(parents=True)
            product.write_text('[{"action":"click","role":"button","name":"已读 1"}]')
            self.assertEqual(regression_plan(root), product)
            product.write_text("[]")
            with self.assertRaises(ValueError):
                regression_plan(root)

    def test_delivery_reads_explicit_private_credential_without_keychain(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "delivery-token"
            path.write_text("test-only-token\n")
            path.chmod(0o600)
            self.assertEqual(
                delivery_token({"delivery_token_file": str(path)}), "test-only-token"
            )
            path.chmod(0o644)
            with self.assertRaises(ValueError):
                delivery_token({"delivery_token_file": str(path)})
            path.chmod(0o600)
            path.write_text("")
            with self.assertRaises(ValueError):
                delivery_token({"delivery_token_file": str(path)})
            with self.assertRaises(ValueError):
                delivery_token({})

    def test_pipeline_rework_resumes_roles_then_publishes_without_review_gate(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            workspace = root / "work"
            workspace.mkdir()
            run(["git", "init", "-q", str(workspace)])
            run(
                [
                    "git",
                    "-c",
                    "user.name=Test",
                    "-c",
                    "user.email=test@example.invalid",
                    "commit",
                    "--allow-empty",
                    "-qm",
                    "base",
                ],
                workspace,
            )
            run(["git", "update-ref", "refs/remotes/origin/main", "HEAD"], workspace)
            settings = {
                "registry": str(root / "registry"),
                "repository": "owner/product",
                "tool_binary": "verify-tool",
                "agents": {
                    stage: stage + "-agent" for stage in (*pipeline.STAGES, "review")
                },
                "python": "python3",
            }
            registration = pipeline.task_directory(settings, workspace)
            registration.mkdir(parents=True)
            taskfile = root / "registry/issues/1.json"
            pipeline.save(
                taskfile,
                {
                    "workspace": str(workspace),
                    "branch": "task",
                    "autonomous": True,
                    "stages": {
                        "development": {"conversation_id": "dev"},
                        "design": {"conversation_id": "design"},
                    },
                },
            )
            config = root / "config.json"
            config.write_text(
                json.dumps(
                    {
                        "base_url": "http://localhost",
                        "token": "test",
                        "pipeline": settings,
                    }
                )
            )
            issued = []
            prompts = []
            access_changes = []

            class Client:
                def __init__(self, *args):
                    pass

                def wait(self, *args, **kwargs):
                    return {"conversation": {"status": "idle"}, "messages": []}

                def workspace_access(self, conversation_id, **kwargs):
                    access_changes.append((conversation_id, kwargs["read_only"]))

                def conversation(self, conversation_id):
                    return {"conversation": {"status": "idle"}}

                def invoke(self, prompt, **kwargs):
                    target = kwargs.get("conversation_id") or kwargs["agent_id"]
                    issued.append(target)
                    prompts.append(prompt)
                    return {
                        "conversation_id": target,
                        "message_id": len(issued),
                    }

            def result(stage, name, target=""):
                file = workspace / name
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_text("Confirmed report")
                handoff_path = pipeline.result_path(
                    registration, json.loads(taskfile.read_text()), stage
                )
                pipeline.save(
                    handoff_path,
                    {
                        "sha256": "digest",
                        "handoff": {"summary": "User approved", "target_stage": target},
                        "documents": {name: file.read_text()},
                    },
                )

                return str(handoff_path)

            real_run = pipeline.run

            def command(argv, cwd=None):
                return "" if argv[0] == "verify-tool" else real_run(argv, cwd)

            def execute(stage):
                with patch(
                    "sys.argv", ["pipeline", "--config", str(config), "--stage", stage]
                ):
                    pipeline.main()

            result("development", "docs/development.md")
            with (
                patch.dict(
                    os.environ,
                    {
                        "GH_REPO": "owner/product",
                        "ISSUE_NUMBER": "1",
                        "RESULT_SHA256": "digest",
                    },
                ),
                patch.object(pipeline, "Client", Client),
                patch.object(
                    pipeline,
                    "github",
                    return_value={
                        "id": 1,
                        "state": "open",
                        "title": "Feature",
                        "body": "",
                    },
                ),
                patch.object(pipeline, "comment_once") as comment,
                patch.object(
                    pipeline, "prepare_registration", return_value=registration
                ),
                patch.object(pipeline, "run", side_effect=command),
                patch.object(pipeline, "publish") as publish,
            ):
                execute("qa")
                self.assertEqual(issued, ["qa-agent"])
                self.assertIn("Issue 已选择自主推进", prompts[-1])
                publish.assert_not_called()
                with self.assertRaises(ValueError):
                    execute("report")
                incoming = result("qa", "docs/05-validation/tasks/1/qa.md", "design")
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "qa", "RESULT_FILE": incoming}
                ):
                    execute("design")
                    execute("design")
                self.assertEqual(issued, ["qa-agent", "design"])
                saved = json.loads(taskfile.read_text())
                self.assertEqual(saved["active_stage"], "design")
                self.assertTrue(saved["autonomous"])
                self.assertIn("Issue 已选择自主推进", prompts[-1])
                self.assertEqual(saved["stages"]["design"]["conversation_id"], "design")
                incoming = result("design", "docs/design.md")
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "design", "RESULT_FILE": incoming}
                ):
                    execute("development")
                incoming = result("development", "docs/development.md")
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "development", "RESULT_FILE": incoming}
                ):
                    execute("qa")
                self.assertEqual(issued, ["qa-agent", "design", "dev", "qa-agent"])
                with patch.dict(
                    os.environ,
                    {
                        "AFTER_STAGE": "qa",
                        "RESULT_FILE": str(registration / "qa-result.json"),
                    },
                ):
                    with self.assertRaisesRegex(ValueError, "Stale callback"):
                        execute("report")
                # The same unresolved defect can recur with identical report bytes.
                incoming = result("qa", "docs/05-validation/tasks/1/qa.md", "design")
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "qa", "RESULT_FILE": incoming}
                ):
                    execute("design")
                rework_comments = [
                    call.args[2]
                    for call in comment.call_args_list
                    if "platform-rework:" in call.args[2]
                ]
                self.assertEqual(len(set(rework_comments)), 2)
                incoming = result("design", "docs/design.md")
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "design", "RESULT_FILE": incoming}
                ):
                    execute("development")
                incoming = result("development", "docs/development.md")
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "development", "RESULT_FILE": incoming}
                ):
                    execute("qa")
                incoming = result("qa", "docs/05-validation/tasks/1/qa.md", "report")
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "qa", "RESULT_FILE": incoming}
                ):
                    execute("report")
                publish.assert_called_once()
                pipeline.save(
                    registration / "delivery.json",
                    {"pr_url": "https://github.com/owner/product/pull/2"},
                )
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "code_review", "GITHUB_RUN_ID": "456"}
                ):
                    execute("review")
                    execute("review")
                self.assertEqual(
                    json.loads(taskfile.read_text())["active_stage"], "report"
                )
                self.assertEqual(issued.count("review-agent"), 1)
                publish.assert_called_once()
                with patch.dict(os.environ, {"REVERIFY_RUN": "123"}):
                    execute("qa")
                    execute("qa")
                self.assertEqual(
                    issued,
                    [
                        "qa-agent",
                        "design",
                        "dev",
                        "qa-agent",
                        "design",
                        "dev",
                        "qa-agent",
                        "review-agent",
                        "qa-agent",
                    ],
                )
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "qa", "RESULT_FILE": incoming}
                ):
                    with self.assertRaisesRegex(ValueError, "Stale callback"):
                        execute("report")

                # A replacement returning to the calling stage reuses that
                # session; duplicate dispatches must never demote it to read-only.
                incoming = result("qa", "docs/05-validation/tasks/1/qa.md", "qa")
                value = json.loads(Path(incoming).read_text())
                value["source_stage"] = "qa"
                pipeline.save(Path(incoming), value)
                task = json.loads(taskfile.read_text())
                task["round"] = "124"
                task["replacement"] = {
                    "file": incoming,
                    "sha256": "digest",
                    "stage": "qa",
                }
                pipeline.save(taskfile, task)
                access_changes.clear()
                before = len(issued)
                with patch.dict(
                    os.environ, {"AFTER_STAGE": "redirect", "RESULT_FILE": incoming}
                ):
                    execute("qa")
                    execute("qa")
                self.assertEqual(len(issued), before + 1)
                self.assertEqual(access_changes, [("qa-agent", False)])

    def test_publish_requires_qa_of_the_unchanged_product(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            run(["git", "init", "-q", str(root)])
            (root / "app").mkdir()
            code = root / "app/main.js"
            code.write_text("original")
            run(["git", "add", "app"], root)
            digest = product_digest(root)
            task = {"stages": {}}
            with self.assertRaises(ValueError):
                require_verified_product(task, root)
            task["stages"]["qa"] = {"product_sha256": digest}
            require_verified_product(task, root)
            (root / "docs").mkdir()
            (root / "docs/qa.md").write_text("QA evidence")
            require_verified_product(task, root)
            code.write_text("changed after approval")
            with self.assertRaises(ValueError):
                require_verified_product(task, root)
            code.write_text("original")
            new_file = root / "app/new.js"
            new_file.write_text("unreviewed new code")
            with self.assertRaises(ValueError):
                require_verified_product(task, root)
            new_file.unlink()
            code.unlink()
            with self.assertRaises(ValueError):
                require_verified_product(task, root)

    def test_receiver_rejects_wrong_digest_and_changed_document(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "prd.md").write_text("approved")
            receipt = root / "result.json"
            receipt.write_text(
                json.dumps({"sha256": "abc", "documents": {"prd.md": "approved"}})
            )

            # Digest verification is the producer's shared canonical implementation.
            def verify(path, digest):
                return None

            self.assertEqual(
                load_handoff(receipt, "abc", root, verify)["documents"]["prd.md"],
                "approved",
            )
            with self.assertRaises(ValueError):
                load_handoff(receipt, "other", root, verify)
            (root / "prd.md").write_text("unapproved change")
            with self.assertRaises(ValueError):
                load_handoff(receipt, "abc", root, verify)
            receipt.write_text(
                json.dumps({"sha256": "abc", "documents": {"../outside": "bad"}})
            )
            with self.assertRaises(ValueError):
                load_handoff(receipt, "abc", root, verify)


if __name__ == "__main__":
    unittest.main()


class WithdrawalTest(unittest.TestCase):
    def test_revoked_handoff_cannot_be_consumed_even_as_duplicate(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            receipt = root / "qa-result.json"
            pipeline.save(
                receipt,
                {
                    "sha256": "abc",
                    "revoked": True,
                    "handoff": {"target_stage": "design", "summary": "old"},
                    "documents": {},
                },
            )
            task = {
                "active_stage": "design",
                "transition": {
                    "file": str(receipt),
                    "sha256": "abc",
                    "stage": "design",
                },
            }
            with self.assertRaisesRegex(ValueError, "withdrawn"):
                pipeline.accept_handoff(
                    task,
                    root,
                    "design",
                    "qa",
                    str(receipt),
                    "abc",
                    root,
                    lambda *a: None,
                )

    def test_redirect_requires_registered_replacement_and_preserves_sessions(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            receipt = root / "replacement-result.json"
            (root / "prd.md").write_text("updated requirement")
            pipeline.save(
                receipt,
                {
                    "sha256": "new",
                    "handoff": {
                        "target_stage": "requirements",
                        "summary": "user corrected target",
                    },
                    "documents": {"prd.md": "updated requirement"},
                },
            )
            task = {
                "active_stage": "design",
                "round": "3",
                "stages": {"requirements": {"conversation_id": "original"}},
            }
            args = (
                task,
                root,
                "requirements",
                "redirect",
                str(receipt),
                "new",
                root,
                lambda *a: None,
            )
            with self.assertRaises(ValueError):
                pipeline.accept_handoff(*args)
            task["replacement"] = {
                "file": str(receipt),
                "sha256": "new",
                "stage": "requirements",
            }
            pipeline.accept_handoff(*args)
            self.assertEqual(task["active_stage"], "requirements")
            self.assertEqual(task["round"], "3")
            self.assertEqual(
                task["stages"]["requirements"]["conversation_id"], "original"
            )
            self.assertNotIn("replacement", task)
