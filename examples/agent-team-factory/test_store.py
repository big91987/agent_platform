import concurrent.futures
import os
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

from factory_store import Store


class StoreTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.store = Store(self.root)
        self.store.initialize({"concurrency": 2})

    def task(self, name, dependencies=None):
        return {
            "id": name,
            "title": name,
            "instructions": "inspect real code",
            "depends_on": dependencies or [],
            "write_paths": [],
        }

    def test_concurrent_controllers_share_limit_and_never_double_claim(self):
        self.store.add_tasks([self.task(f"task-{i}") for i in range(8)])

        def claim(_):
            return Store(self.root).claim()

        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
            claims = [x for x in pool.map(claim, range(8)) if x]
        self.assertEqual(len(claims), 2)
        self.assertEqual(len({x["id"] for x in claims}), 2)
        self.store.finish(claims[0]["id"], "ready", {"message": "finished"})
        self.assertIsNotNone(self.store.claim())

    def test_dependency_only_releases_after_success_and_state_survives_restart(self):
        self.store.add_tasks([self.task("first"), self.task("second", ["first"])])
        self.assertEqual(self.store.claim()["id"], "first")
        self.assertIsNone(self.store.claim())
        self.store.finish("first", "failed", {"message": "native failure"})
        self.assertIsNone(Store(self.root).claim())

    def test_submission_is_idempotent_and_conflict_does_not_overwrite(self):
        self.store.add_tasks([self.task("one")])
        self.store.claim()
        report = {"status": "completed", "message": "done", "artifacts": []}
        self.store.submit("one", report)
        self.store.submit("one", report)
        with self.assertRaises(ValueError):
            self.store.submit("one", {**report, "message": "different"})
        task = Store(self.root).snapshot()["tasks"][0]
        self.assertEqual(task["submission"], report)
        self.assertEqual(task["status"], "running")

    def test_bad_dependency_and_duplicate_plan_are_rejected_atomically(self):
        with self.assertRaises(ValueError):
            self.store.add_tasks([self.task("one", ["missing"])])
        self.assertEqual(self.store.snapshot()["tasks"], [])
        with self.assertRaises(ValueError):
            self.store.add_tasks([self.task("one"), self.task("one")])

    def test_message_is_persistent_without_completing_task(self):
        self.store.add_tasks([self.task("one")])
        self.store.message("one", "leader", "Which API contract should I use?")
        snap = Store(self.root).snapshot()
        self.assertEqual(
            snap["messages"][0]["body"], "Which API contract should I use?"
        )
        self.assertEqual(snap["tasks"][0]["status"], "pending")

    def test_claimed_peak_is_not_reported_as_native_process_peak(self):
        self.store.add_tasks([self.task("one"), self.task("two")])
        self.store.claim()
        self.store.claim()
        self.assertEqual(self.store.snapshot()["peak_native_processes"], 0)
        with self.store.connect() as db:
            self.store.event(db, "one", "native_started", {})
            self.store.event(db, "two", "native_started", {})
            self.store.event(db, "one", "native_finished", {})
        self.assertEqual(self.store.snapshot()["peak_native_processes"], 2)

    def test_concurrency_change_is_explicit_and_refused_while_tasks_run(self):
        self.store.set_concurrency(4)
        self.assertEqual(Store(self.root).settings()["concurrency"], 4)
        self.store.add_tasks([self.task("one")])
        self.store.claim()
        with self.assertRaises(ValueError):
            self.store.set_concurrency(20)
        self.assertEqual(self.store.settings()["concurrency"], 4)
        self.store.finish("one", "failed", {"message": "preserved"})
        self.store.set_concurrency(20)
        self.assertEqual(self.store.settings()["concurrency"], 20)
        with self.assertRaises(ValueError):
            self.store.set_concurrency(21)

    def test_resume_refuses_old_process_group_after_parent_has_exited(self):
        parent = subprocess.Popen(
            [
                sys.executable,
                "-c",
                "import subprocess,sys; subprocess.Popen([sys.executable,'-c','import time; time.sleep(20)'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)",
            ],
            start_new_session=True,
        )
        parent.wait(timeout=3)
        self.store.add_tasks([self.task("one")])
        self.store.claim()
        self.store.execution(
            "one",
            pid=parent.pid,
            thread_id="recorded-session",
            worktree=str(self.root / "worktree"),
            branch="agent-team/task-one-1",
            base_commit="a" * 40,
            finished=time.time(),
            returncode=0,
        )
        self.store.finish("one", "failed", {"message": "parent exited without handoff"})
        try:
            with self.assertRaises(ValueError):
                self.store.requeue("one", "Do not overlap with surviving descendants.")
            self.assertEqual(self.store.get("one")["status"], "failed")
        finally:
            try:
                os.killpg(parent.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass


if __name__ == "__main__":
    unittest.main()
