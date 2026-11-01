import concurrent.futures
import json
import os
import signal
import subprocess
import sys
import tempfile
import threading
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

    def test_backlog_100_drains_once_in_order_with_20_claims_across_connections(self):
        # Database contention fixture only: these are not model executions or
        # product tasks. A held first wave makes the 20-slot boundary observable.
        self.store.set_concurrency(20)
        expected = [f"backlog-{i:03d}" for i in range(100)]
        self.store.add_tasks([self.task(name) for name in expected])
        wave = threading.Barrier(20)
        lock = threading.Lock()
        done = threading.Event()
        claimed, completed = [], []
        deadline = time.monotonic() + 20

        def worker(_):
            store = Store(self.root)
            while time.monotonic() < deadline:
                task = store.claim()
                if task is None:
                    if done.wait(0.01):
                        return
                    continue
                with lock:
                    claimed.append(task["id"])
                    first_wave = len(claimed) <= 20
                if first_wave:
                    wave.wait(timeout=15)
                store.finish(
                    task["id"], "ready", {"message": "database contention fixture"}
                )
                with lock:
                    completed.append(task["id"])
                    if len(completed) == len(expected):
                        done.set()
            raise AssertionError(
                "backlog did not drain before contention fixture deadline"
            )

        with concurrent.futures.ThreadPoolExecutor(max_workers=40) as pool:
            list(pool.map(worker, range(40)))
        snapshot = self.store.snapshot()
        self.assertEqual(len(completed), len(set(completed)))
        self.assertEqual(set(completed), set(expected))
        self.assertEqual(
            [e["task_id"] for e in snapshot["events"] if e["kind"] == "started"],
            expected,
        )
        self.assertEqual(snapshot["peak_claimed"], 20)
        self.assertEqual(snapshot["peak_native_processes"], 0)
        self.assertEqual({t["status"] for t in snapshot["tasks"]}, {"ready"})

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

    def test_system_message_identities_cannot_be_assigned_to_tasks(self):
        for reserved in ("user", "leader"):
            with self.assertRaises(ValueError):
                self.store.add_tasks([self.task(reserved)])
        self.assertEqual(self.store.snapshot()["tasks"], [])

    def test_ambiguous_existing_sender_is_rejected_instead_of_trusted(self):
        self.store.add_tasks([self.task("victim")])
        # Isolated fixture of an older installation; not a production migration.
        with self.store.connect() as db:
            db.execute(
                "INSERT INTO tasks(id,definition,created) VALUES (?,?,?)",
                ("user", json.dumps(self.task("user")), 1),
            )
            db.execute(
                "INSERT INTO messages(sender,recipient,body,created) VALUES ('user','victim','ambiguous old peer text',1)"
            )
        with self.assertRaises(ValueError):
            self.store.user_input("victim")
        with self.assertRaises(ValueError):
            self.store.message("user", "victim", "must not add ambiguous input")

    def test_pending_dependency_revision_is_audited_and_started_tasks_are_frozen(self):
        self.store.add_tasks([self.task("one"), self.task("two", ["one"])])
        self.store.set_dependencies(
            "two", [], "independent work must not wait on an unrelated gate"
        )
        self.assertEqual(self.store.get("two")["depends_on"], [])
        event = self.store.snapshot()["events"][-1]
        self.assertEqual(event["kind"], "dependencies_changed")
        self.assertEqual(event["detail"]["previous"], ["one"])
        self.store.set_dependencies("two", ["one"], "restore a real input dependency")
        with self.assertRaises(ValueError):
            self.store.set_dependencies("one", ["two"], "cycle is invalid")
        with self.assertRaises(ValueError):
            self.store.set_dependencies("two", ["missing"], "unknown input is invalid")
        self.assertEqual(self.store.get("one")["depends_on"], [])
        self.assertEqual(self.store.get("two")["depends_on"], ["one"])
        for invalid in (None, "one", ["one", "one"], [1]):
            with self.assertRaises(ValueError):
                self.store.set_dependencies("two", invalid, "invalid input")
        with self.assertRaises(ValueError):
            self.store.set_dependencies("two", [], " ")
        with self.store.control(), self.assertRaises(ValueError):
            Store(self.root).set_dependencies("two", [], "controller is busy")
        self.assertEqual(self.store.get("two")["depends_on"], ["one"])
        self.store.claim()
        with self.assertRaises(ValueError):
            self.store.set_dependencies("one", [], "cannot change a frozen execution")
        self.assertEqual(self.store.get("one")["attempt"], 1)

    def test_execution_configuration_is_persistent_and_cannot_change_active_task(self):
        self.store.configure_execution({"PATH": "/pinned/bin:/usr/bin"}, True)
        self.assertEqual(
            Store(self.root).settings()["execution_env"]["PATH"], "/pinned/bin:/usr/bin"
        )
        self.assertTrue(self.store.settings()["network_access"])
        self.store.add_tasks([self.task("one")])
        self.store.claim()
        with self.assertRaises(ValueError):
            self.store.configure_execution({}, False)
        self.assertTrue(self.store.settings()["network_access"])

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

    def test_resume_ignores_reused_pid_outside_the_owned_process_group(self):
        # Simulate a reused PID: an unrelated live process belongs to a
        # different group. Recovery must neither signal nor claim ownership.
        unrelated = subprocess.Popen(
            [sys.executable, "-c", "import time;time.sleep(20)"]
        )
        try:
            self.assertNotEqual(os.getpgid(unrelated.pid), unrelated.pid)
            self.store.add_tasks([self.task("one")])
            self.store.claim()
            self.store.execution(
                "one",
                pid=unrelated.pid,
                thread_id="preserved-session",
                worktree=str(self.root / "work"),
                branch="agent-team/task-one-1",
                base_commit="a" * 40,
                finished=time.time(),
                returncode=0,
            )
            self.store.finish("one", "blocked", {"message": "preserved failure"})
            self.store.requeue(
                "one", "Original process group is gone; PID now unrelated."
            )
            self.assertEqual(self.store.get("one")["status"], "pending")
            self.assertIsNone(unrelated.poll())
        finally:
            unrelated.terminate()
            unrelated.wait(timeout=3)

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
