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

    def test_cli_reviews_all_files_without_aggregate_token_cutoff(self):
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import patch

        import runtime

        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)

            def execute(args, destination, *, llm, timeout):
                # The pinned CLI treats 0 as unlimited. A numeric cap can
                # stop dispatch while native coverage still has pending files.
                self.assertEqual(args[args.index("--max-tokens-budget") + 1], "0")
                destination.write_text(json.dumps(self.envelope()))
                return 0, 1.0

            source = {"base": "a" * 40, "head": "b" * 40, "expected": ["app.go"]}
            with (
                patch.object(runtime, "STATE", output),
                patch.object(runtime, "OUTPUT", output),
                patch.object(runtime, "trusted_source", return_value=source),
                patch.object(runtime, "run_tool", side_effect=execute),
                patch.object(runtime, "finish", return_value=0) as finish,
            ):
                self.assertEqual(runtime.cli_review(binary="ocr"), 0)
            self.assertEqual(finish.call_args.args[:3], ("ocr-cli", self.envelope(), 0))

    def test_cli_resumes_incomplete_group_once_with_bound_coverage(self):
        import copy
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import patch

        import runtime

        first = self.envelope()
        first["status"] = "partial"
        sid = "905bef8d-29fd-4cdc-8ad2-378a7c883977"
        first["session_id"] = sid
        m = first["manifest"]
        m.update(run_id=sid, terminal_state="partial", input={"mode": "range"})
        covered = {"path": "app.go", "item_id": "a", "fingerprint": "old"}
        pending = {"path": "ui.js", "item_id": "b", "fingerprint": "new"}
        m["coverage"].update(
            selected=[covered, pending], completed=[covered],
            failed=[dict(pending, reason="reached the maximum tool-request rounds without finishing")],
        )
        second = copy.deepcopy(first)
        second["status"] = "complete"
        second["manifest"].update(terminal_state="complete", parent_run_id=sid)
        second["manifest"]["coverage"].update(completed=[pending], reused=[covered], failed=[])
        for case in ("valid", "wrong-parent", "wrong-reuse", "different-failure"):
            with self.subTest(case=case), tempfile.TemporaryDirectory() as folder:
                output = Path(folder)
                initial, resumed = copy.deepcopy(first), copy.deepcopy(second)
                if case == "wrong-parent":
                    resumed["manifest"]["parent_run_id"] = "other"
                if case == "wrong-reuse":
                    resumed["manifest"]["coverage"]["reused"][0]["fingerprint"] = "changed"
                if case == "different-failure":
                    initial["manifest"]["coverage"]["failed"][0]["reason"] = "model unavailable"
                calls = []

                def execute(args, destination, **kwargs):
                    calls.append(args)
                    value = resumed if "--resume" in args else initial
                    destination.write_text(json.dumps(value))
                    destination.with_suffix(".stderr.log").write_text("native log")
                    return 0, 1.0

                source = {"base": "a" * 40, "head": "b" * 40, "expected": ["app.go", "ui.js"], "files": {}}
                with (
                    patch.object(runtime, "STATE", output), patch.object(runtime, "OUTPUT", output),
                    patch.object(runtime, "trusted_source", return_value=source),
                    patch.object(runtime, "candidate_snapshot", return_value={}),
                    patch.object(runtime, "run_tool", side_effect=execute),
                ):
                    (output / "source.json").write_text(json.dumps(source))
                    self.assertEqual(runtime.cli_review(binary="ocr"), 0 if case == "valid" else 20)
                self.assertEqual(len(calls), 1 if case == "different-failure" else 2)
                if len(calls) == 2:
                    self.assertEqual(calls[1][-2:], ["--resume", sid])
                    self.assertEqual(json.loads((output / "raw-initial.json").read_text()), initial)

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

        for ignored in (False, True):
            with self.subTest(ignored=ignored), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                subprocess.run(["git", "init", "-q", folder], check=True)
                (root / "app.py").write_text("before")
                (root / "deleted.py").write_text("remove me")
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
                (root / "app.py").write_text("staged")
                git(root, "add", "app.py")
                index = git(root, "write-tree")
                (root / "app.py").write_text("after")
                (root / "deleted.py").unlink()
                new_name = ":(glob)new [entry]\nfile.py"
                (root / new_name).write_text("new file")
                (root / "harness-output").mkdir()
                private = root / "harness-output/deep/private.txt"
                private.parent.mkdir()
                private.write_text("report excluded")
                private_blob = git(root, "hash-object", str(private)).decode().strip()
                if ignored:
                    (root / ".git/info/exclude").write_text("harness-output/\n")
                candidate = candidate_commit(root)
                self.assertEqual(git(root, "rev-parse", "HEAD"), head)
                self.assertEqual(git(root, "write-tree"), index)
                self.assertEqual(git(root, "show", candidate + ":app.py"), b"after")
                self.assertEqual(
                    git(root, "show", candidate + ":" + new_name), b"new file"
                )
                self.assertNotIn(
                    b"deleted.py", git(root, "ls-tree", "-r", "--name-only", candidate)
                )
                self.assertNotIn(
                    b"harness-output",
                    git(root, "ls-tree", "-r", "--name-only", candidate),
                )
                self.assertNotEqual(
                    subprocess.run(
                        ["git", "-C", str(root), "cat-file", "-e", private_blob],
                        stdout=subprocess.DEVNULL,
                        stderr=subprocess.DEVNULL,
                    ).returncode,
                    0,
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
