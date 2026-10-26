import unittest

from runtime import assess, candidate_snapshot


class ReviewResultTest(unittest.TestCase):
    def envelope(self):
        item = {"path": "app.go", "item_id": "a"}
        return {
            "status": "complete",
            "comments": [],
            "manifest": {
                "schema_version": "ocr.run-manifest/v1",
                "terminal_state": "complete",
                "coverage": {
                    "selected": [item],
                    "completed": [item],
                    "reused": [],
                    "failed": [],
                    "waived": [],
                },
            },
        }

    def test_exit_zero_does_not_accept_partial_coverage(self):
        value = self.envelope()
        value["manifest"]["coverage"]["completed"] = []
        value["manifest"]["coverage"]["failed"] = [{"path": "app.go", "item_id": "a"}]
        self.assertEqual(assess(value, 0, ["app.go"])["status"], "incomplete")

    def test_no_comments_requires_exact_coverage_and_terminal_state(self):
        self.assertEqual(assess(self.envelope(), 0, ["app.go"])["status"], "clean")
        self.assertEqual(
            assess(self.envelope(), 0, ["app.go", "other.go"])["status"], "incomplete"
        )
        self.assertEqual(assess(self.envelope(), 1, ["app.go"])["status"], "error")
        self.assertEqual(
            assess({"status": "skipped", "comments": []}, 0, [])["status"], "incomplete"
        )

    def test_findings_are_not_success_and_unknown_severity_is_explicit(self):
        value = self.envelope()
        value["comments"] = [
            {"path": "app.go", "content": "missing check", "severity": "high"}
        ]
        self.assertEqual(assess(value, 0, ["app.go"])["status"], "findings")
        value["comments"][0]["severity"] = "unknown"
        self.assertEqual(assess(value, 0, ["app.go"])["status"], "incomplete")

    def test_warning_duplicate_or_foreign_coverage_is_not_clean(self):
        value = self.envelope()
        value["warnings"] = ["failed one file"]
        self.assertEqual(assess(value, 0, ["app.go"])["status"], "incomplete")
        value = self.envelope()
        value["manifest"]["coverage"]["completed"] *= 2
        self.assertEqual(assess(value, 0, ["app.go"])["status"], "incomplete")

    def test_null_comments_produces_a_report_without_accepting_partial_review(self):
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import patch

        import runtime

        value = self.envelope()
        value["comments"] = None
        for status, exit_code, expected_code in (
            ("complete", 0, 0),
            ("partial", 0, 20),
            ("failed", 1, 30),
        ):
            value["status"] = status
            value["manifest"]["terminal_state"] = value["status"]
            with tempfile.TemporaryDirectory() as folder:
                output = Path(folder)
                source = {
                    "base": "b" * 40,
                    "head": "a" * 40,
                    "expected": ["app.go"],
                    "files": {"app.go": "digest"},
                }
                (output / "source.json").write_text(json.dumps(source))
                with (
                    patch.object(runtime, "OUTPUT", output),
                    patch.object(runtime, "trusted_source", return_value=source),
                    patch.object(
                        runtime, "candidate_snapshot", return_value=source["files"]
                    ),
                ):
                    code = runtime.finish("ocr-cli", value, exit_code)
                self.assertEqual(code, expected_code)
                self.assertTrue((output / "report.md").is_file())
                self.assertEqual(
                    json.loads((output / "result.json").read_text())["comments"], []
                )

    def test_malformed_findings_stay_incomplete_and_can_be_presented(self):
        for comments in ("", "invalid", {}, [None], ["invalid"]):
            value = self.envelope()
            value["comments"] = comments
            for status, exit_code in (("complete", 0), ("partial", 0), ("failed", 1)):
                value["status"] = status
                value["manifest"]["terminal_state"] = status
                result = assess(value, exit_code, ["app.go"])
                self.assertEqual(
                    result["status"], "error" if exit_code else "incomplete"
                )
                self.assertEqual(result["comments"], [])

    def test_worktree_change_without_head_change_is_detected(self):
        import subprocess
        import tempfile
        from pathlib import Path

        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            subprocess.run(["git", "init", "-q", folder], check=True)
            (root / "app.py").write_text("one")
            subprocess.run(["git", "-C", folder, "add", "app.py"], check=True)
            before = candidate_snapshot(root)
            (root / "app.py").write_text("two")
            self.assertNotEqual(candidate_snapshot(root), before)


class CandidateAndDelegateTest(unittest.TestCase):
    def test_candidate_commit_keeps_original_head_index_and_changed_file(self):
        import subprocess
        import tempfile
        from pathlib import Path

        from runtime import candidate_commit, git

        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            subprocess.run(["git", "init", "-q", folder], check=True)
            (root / "app.py").write_text("before")
            git(root, "add", ".")
            git(
                root,
                "-c",
                "user.name=fixture",
                "-c",
                "user.email=fixture@localhost",
                "commit",
                "-qm",
                "base",
            )
            head = git(root, "rev-parse", "HEAD")
            index = git(root, "write-tree")
            (root / "app.py").write_text("after")
            (root / "harness-output").mkdir()
            (root / "harness-output/private.txt").write_text("report excluded")
            candidate = candidate_commit(root)
            self.assertEqual(git(root, "rev-parse", "HEAD"), head)
            self.assertEqual(git(root, "write-tree"), index)
            self.assertEqual(git(root, "show", candidate + ":app.py"), b"after")
            self.assertNotIn(
                b"harness-output", git(root, "ls-tree", "-r", "--name-only", candidate)
            )

    def test_delegate_accounts_for_every_selected_file_and_skips_are_incomplete(self):
        from runtime import assess_delegate

        value = {
            "engine": "codex-delegate",
            "files": [{"path": "a.go", "status": "reviewed"}],
            "comments": [],
        }
        self.assertEqual(assess_delegate(value, ["a.go"])["status"], "clean")
        self.assertEqual(
            assess_delegate(value, ["a.go", "b.go"])["status"], "incomplete"
        )
        value["files"][0]["status"] = "skipped"
        self.assertEqual(assess_delegate(value, ["a.go"])["status"], "incomplete")

    def test_delegate_cannot_replace_platform_captured_source(self):
        import hashlib
        import json
        import os
        import tempfile
        from pathlib import Path
        from unittest.mock import patch

        import runtime

        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder) / "output"
            output.mkdir()
            expected = {
                "head": "a" * 40,
                "base": "b" * 40,
                "files": {"app.go": "original"},
                "expected": ["app.go"],
            }
            raw = json.dumps(expected).encode()
            (output / "source.json").write_bytes(raw)
            receipt = {
                "output": '{"Event":"ping","Data":"{}"}\n{"prepared":true}CODE_REVIEW_SOURCE_SHA256='
                + hashlib.sha256(raw).hexdigest()
                + '{"Event":"execution_complete"}\n',
                "exit_code": 0,
            }
            input_file = Path(folder) / "input.json"
            input_file.write_text(
                json.dumps(
                    {
                        "previous_results": [
                            {"node": "prepare", "execution_receipt": receipt}
                        ]
                    }
                )
            )
            with (
                patch.dict(os.environ, {"WORKFLOW_INPUT_FILE": str(input_file)}),
                patch.object(runtime, "OUTPUT", output),
            ):
                self.assertEqual(runtime.trusted_source(required=True), expected)
                (output / "source.json").write_text("{}")
                with self.assertRaises(ValueError):
                    runtime.trusted_source(required=True)
                (output / "source.json").write_bytes(raw)
                receipt["exit_code"] = 1
                input_file.write_text(
                    json.dumps(
                        {
                            "previous_results": [
                                {"node": "prepare", "execution_receipt": receipt}
                            ]
                        }
                    )
                )
                with self.assertRaises(ValueError):
                    runtime.trusted_source(required=True)

    def test_truncated_or_duplicate_baseline_log_is_rejected(self):
        import hashlib
        import json
        import os
        import tempfile
        from pathlib import Path
        from unittest.mock import patch

        import runtime

        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder) / "output"
            output.mkdir()
            raw = b'{"files":{},"expected":[]}'
            (output / "source.json").write_bytes(raw)
            line = "CODE_REVIEW_SOURCE_SHA256=" + hashlib.sha256(raw).hexdigest() + "\n"
            input_file = Path(folder) / "input.json"
            for log, truncated in ((line, True), (line + line, False)):
                input_file.write_text(
                    json.dumps(
                        {
                            "previous_results": [
                                {
                                    "node": "prepare",
                                    "execution_receipt": {
                                        "output": log,
                                        "exit_code": 0,
                                        "log": {"truncated": truncated},
                                    },
                                }
                            ]
                        }
                    )
                )
                with (
                    patch.dict(os.environ, {"WORKFLOW_INPUT_FILE": str(input_file)}),
                    patch.object(runtime, "OUTPUT", output),
                ):
                    with self.assertRaises(ValueError):
                        runtime.trusted_source(required=True)
