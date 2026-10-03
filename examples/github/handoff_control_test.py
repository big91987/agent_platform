"""Real receipt/state transitions with network boundaries replaced for isolation."""

import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import pipeline

try:
    import handoff_control as control
except ImportError:
    control = None


class Client:
    def __init__(self):
        self.status = "running"
        self.stops = []
        self.inputs = []

    def conversation(self, cid):
        return {
            "conversation": {"status": self.status},
            "messages": [{"id": 12, "role": "user", "status": self.status}],
        }

    def stop(self, cid, **kw):
        self.stops.append((cid, kw))
        self.status = "stopped"

    def wait(self, cid, **kw):
        return self.conversation(cid)

    def invoke(self, message, **kw):
        self.inputs.append(message)
        return {"message_id": 13, "conversation_id": kw["conversation_id"]}

    def steer(self, *a):
        pass


class ControlTest(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(control, "handoff management is not implemented")
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.workspace = self.root / "workspace"
        self.workspace.mkdir()
        self.settings = {
            "registry": str(self.root / "registry"),
            "repository": "owner/repo",
            "tool_binary": "unused",
        }
        self.directory = pipeline.task_directory(self.settings, self.workspace)
        self.directory.mkdir(parents=True)
        self.path = self.root / "registry/issues/1.json"
        self.file = self.directory / "qa-result.json"
        pipeline.save(
            self.file,
            {
                "run_id": 100,
                "delivery": "accepted",
                "sha256": "old",
                "handoff": {
                    "summary": "QA defect",
                    "artifacts": ["qa.md"],
                    "target_stage": "design",
                },
                "documents": {"qa.md": "report"},
            },
        )
        (self.workspace / "qa.md").write_text("report")
        pipeline.save(
            self.directory / "qa.json",
            {
                "inputs": {"issue": "1", "after": "qa"},
                "ref": "main",
                "dispatch_url": "unused",
                "token_env": "TOKEN",
            },
        )
        pipeline.save(
            self.path,
            {
                "workspace": str(self.workspace),
                "active_stage": "design",
                "round": "1",
                "transition": {
                    "file": str(self.file),
                    "sha256": "old",
                    "stage": "design",
                },
                "stages": {
                    "qa": {"conversation_id": "qa"},
                    "design": {
                        "conversation_id": "design",
                        "message_id": 12,
                        "round": "1",
                        "run_id": "100",
                    },
                },
            },
        )
        self.client = Client()
        self.request = {
            "operation": "replace_handoff",
            "source_stage": "qa",
            "workspace": str(self.workspace),
            "input": {
                "run_id": 100,
                "content": "User says requirements are wrong; return to requirements",
                "target_stage": "requirements",
            },
        }

    def test_returned_stage_cannot_stop_itself_through_old_handle(self):
        task = json.loads(self.path.read_text())
        task["active_stage"] = "qa"
        task["stages"]["qa"] = dict(task["stages"]["design"], conversation_id="qa")
        pipeline.save(self.path, task)
        saved = json.loads(self.file.read_text())
        saved["handoff"]["target_stage"] = "qa"
        pipeline.save(self.file, saved)
        with self.assertRaisesRegex(ValueError, "current stage"):
            control.manage(self.settings, self.client, 1, self.request)
        self.assertEqual(self.client.stops, [])
        self.assertNotIn("replacement", json.loads(self.path.read_text()))
        self.assertFalse(json.loads(self.file.read_text()).get("revoked"))

    def dispatch(self, settings, path, cfg, body):
        # Dispatch boundary must only be reached after withdrawal and process stop.
        self.assertTrue(json.loads(self.file.read_text())["revoked"])
        self.assertEqual(self.client.status, "stopped")
        value = json.loads(path.read_text())
        value.update(run_id=200, delivery="accepted")
        pipeline.save(path, value)
        return value

    def test_replace_stops_old_execution_and_duplicate_returns_same_run(self):
        with (
            patch.object(control, "github", return_value={"status": "completed"}),
            patch.object(control, "hash_result", return_value="new"),
            patch.object(
                control, "dispatch_replacement", side_effect=self.dispatch
            ) as dispatch,
        ):
            first = control.manage(self.settings, self.client, 1, self.request)
            again = control.manage(self.settings, self.client, 1, self.request)
        self.assertEqual(first["run_id"], 200)
        self.assertEqual(again["run_id"], 200)
        self.assertEqual(dispatch.call_count, 1)
        self.assertEqual(len(self.client.stops), 1)
        task = json.loads(self.path.read_text())
        self.assertEqual(task["round"], "2")
        self.assertEqual(task["replacement"]["stage"], "requirements")
        self.assertEqual(
            json.loads(self.file.read_text())["documents"], {"qa.md": "report"}
        )

    def test_replacement_preserves_prior_supplements(self):
        saved = json.loads(self.file.read_text())
        saved["supplements"] = [
            {"request_id": "addition", "content": "Keep the final row accessible"}
        ]
        pipeline.save(self.file, saved)
        with (
            patch.object(control, "github", return_value={"status": "completed"}),
            patch.object(control, "hash_result", return_value="new"),
            patch.object(control, "dispatch_replacement", side_effect=self.dispatch),
        ):
            control.manage(self.settings, self.client, 1, self.request)
        replacement = next(self.directory.glob("qa-replace-*-result.json"))
        self.assertIn(
            "Keep the final row accessible",
            json.loads(replacement.read_text())["handoff"]["summary"],
        )

    def test_stop_failure_never_dispatches_and_retry_can_finish(self):
        with (
            patch.object(control, "github", return_value={"status": "completed"}),
            patch.object(control, "hash_result", return_value="new"),
            patch.object(
                control, "dispatch_replacement", side_effect=self.dispatch
            ) as dispatch,
        ):
            with patch.object(
                self.client, "stop", side_effect=RuntimeError("unavailable")
            ):
                with self.assertRaises(RuntimeError):
                    control.manage(self.settings, self.client, 1, self.request)
            self.assertEqual(dispatch.call_count, 0)
            self.assertTrue(json.loads(self.file.read_text())["revoked"])
            self.assertEqual(
                control.manage(self.settings, self.client, 1, self.request)["run_id"],
                200,
            )

    def test_stale_handle_cannot_cancel_a_later_stage(self):
        task = json.loads(self.path.read_text())
        task["active_stage"] = "development"
        pipeline.save(self.path, task)
        with self.assertRaisesRegex(ValueError, "advanced"):
            control.manage(self.settings, self.client, 1, self.request)
        self.assertFalse(self.client.stops)

    def test_supplement_deduplicates_and_reaches_existing_conversation(self):
        self.request["operation"] = "supplement_handoff"
        self.request["input"].pop("target_stage")
        first = control.manage(self.settings, self.client, 1, self.request)
        again = control.manage(self.settings, self.client, 1, self.request)
        self.assertEqual(first, again)
        self.assertEqual(len(self.client.inputs), 1)
        self.assertIn("requirements are wrong", self.client.inputs[0])
        self.assertFalse(json.loads(self.file.read_text()).get("revoked", False))

    def test_dispatch_retry_handle_can_control_canonical_receiver_run(self):
        task = json.loads(self.path.read_text())
        task["stages"]["design"]["run_id"] = "99"
        pipeline.save(self.path, task)
        self.request["operation"] = "supplement_handoff"
        self.request["input"].pop("target_stage")
        result = control.manage(self.settings, self.client, 1, self.request)
        self.assertEqual(result["conversation_id"], "design")
        self.assertEqual(len(self.client.inputs), 1)

    def test_interrupted_replacement_preserves_original_stop_precondition(self):
        from agent_platform_client import APIError

        real_save = control.save

        def crash(path, value):
            if path == self.file:
                raise RuntimeError("crash after recording the request")
            real_save(path, value)

        with patch.object(control, "save", side_effect=crash):
            with self.assertRaises(RuntimeError):
                control.manage(self.settings, self.client, 1, self.request)
        with (
            patch.object(
                self.client,
                "conversation",
                return_value={
                    "conversation": {"status": "running"},
                    "messages": [{"id": 13, "role": "user"}],
                },
            ),
            patch.object(
                self.client, "stop", side_effect=APIError(409, "new input")
            ) as stop,
        ):
            result = control.manage(self.settings, self.client, 1, self.request)
        self.assertEqual(stop.call_args.kwargs["expected_message_id"], 12)
        self.assertEqual(result["status"], "conflict")
        self.assertFalse(json.loads(self.file.read_text()).get("revoked"))

    def test_stop_precondition_conflict_restores_live_handoff(self):
        from agent_platform_client import APIError

        with patch.object(self.client, "stop", side_effect=APIError(409, "new input")):
            result = control.manage(self.settings, self.client, 1, self.request)
        self.assertEqual(result["status"], "conflict")
        self.assertFalse(json.loads(self.file.read_text()).get("revoked"))
        self.assertNotIn("replacement", json.loads(self.path.read_text()))

    def test_retry_recovers_accepted_replacement_after_receiver_edits_files(self):
        real_save = control.save

        def crash(path, value):
            if path.parent.name == "operations" and value.get("result"):
                raise RuntimeError("process failed after dispatch")
            real_save(path, value)

        with (
            patch.object(control, "github", return_value={"status": "completed"}),
            patch.object(control, "hash_result", return_value="new"),
            patch.object(
                control, "dispatch_replacement", side_effect=self.dispatch
            ) as dispatch,
        ):
            with patch.object(control, "save", side_effect=crash):
                with self.assertRaises(RuntimeError):
                    control.manage(self.settings, self.client, 1, self.request)
            (self.workspace / "qa.md").write_text("recipient revised document")
            self.assertEqual(
                control.manage(self.settings, self.client, 1, self.request)["run_id"],
                200,
            )
            self.assertEqual(dispatch.call_count, 1)

    def test_retry_of_pending_supplement_delivers_after_receiver_started(self):
        task = json.loads(self.path.read_text())
        original = json.loads(self.path.read_text())
        task["active_stage"] = "qa"
        task["round"] = ""
        task.pop("transition")
        pipeline.save(self.path, task)
        self.request["operation"] = "supplement_handoff"
        self.request["input"].pop("target_stage")
        real_save = control.save

        def crash(path, value):
            if path == self.file:
                raise RuntimeError("interrupted before append")
            real_save(path, value)

        with patch.object(control, "save", side_effect=crash):
            with self.assertRaises(RuntimeError):
                control.manage(self.settings, self.client, 1, self.request)
        pipeline.save(self.path, original)
        result = control.manage(self.settings, self.client, 1, self.request)
        self.assertEqual(result["conversation_id"], "design")
        self.assertEqual(len(self.client.inputs), 1)

    def test_consumed_handoff_without_receiver_receipt_is_not_treated_as_unstarted(
        self,
    ):
        task = json.loads(self.path.read_text())
        task["stages"].pop("design")
        pipeline.save(self.path, task)
        with self.assertRaisesRegex(ValueError, "uncertain"):
            control.manage(self.settings, self.client, 1, self.request)
        self.assertFalse(self.client.stops)
        self.assertFalse(json.loads(self.file.read_text()).get("revoked"))
