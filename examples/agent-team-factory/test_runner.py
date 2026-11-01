"""Isolated executor-protocol fixtures; these are NOT real Agent throughput tests."""

import fcntl
import subprocess
import tempfile
import unittest
from pathlib import Path

from factory_store import Store
from runner import Factory


class RunnerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.repo = self.base / "repo"
        self.repo.mkdir()
        self.git("init", "-b", "agent-team/experiment")
        self.git("config", "user.email", "fixture@example.invalid")
        self.git("config", "user.name", "Fixture")
        (self.repo / "README.md").write_text("executor protocol fixture\n")
        self.git("add", "README.md")
        self.git("commit", "-m", "fixture baseline")
        self.root = self.base / "runtime"
        self.probe = self.base / "executor-fixture"
        self.probe.write_text("""#!/usr/bin/env python3
import json,pathlib,subprocess,sys,time
args=sys.argv[1:]
values={args[i+1].split('=',1)[0]:args[i+1].split('=',1)[1] for i,x in enumerate(args[:-1]) if x=='-c'}
prefix='mcp_servers.agent_team_factory'
server=json.loads(values[prefix+'.command'])
server_args=json.loads(values[prefix+'.args'])
task=server_args[server_args.index('--task')+1]
runtime=pathlib.Path(server_args[server_args.index('--root')+1])
sid='replacement-thread' if 'resume' in args and (runtime/'fixture-replace-session').exists() else 'fixture-'+task
print(json.dumps({'type':'thread.started','thread_id':sid}),flush=True)
print(json.dumps({'type':'turn.started'}),flush=True)
sys.stdin.read()
time.sleep(0.3)
if task=='missing-handoff' and server_args[server_args.index('--attempt')+1]=='1': sys.exit(0)
target=pathlib.Path(task+'.txt'); target.write_text('isolated executor fixture, not real Agent output')
requests=[{'jsonrpc':'2.0','id':1,'method':'initialize','params':{'protocolVersion':'2024-11-05'}},{'jsonrpc':'2.0','id':2,'method':'tools/call','params':{'name':'handoff','arguments':{'status':'completed','message':'isolated protocol fixture','artifacts':[str(target)]}}}]
p=subprocess.run([server,*server_args],input='\\n'.join(map(json.dumps,requests))+'\\n',capture_output=True,text=True)
if p.returncode or any(json.loads(x).get('result',{}).get('isError') for x in p.stdout.splitlines()): sys.exit(2)
print(json.dumps({'type':'turn.completed','usage':{'input_tokens':1,'output_tokens':1}}),flush=True)
""")
        self.probe.chmod(0o700)
        self.store = Store(self.root)
        self.store.initialize(
            {
                "repository": str(self.repo),
                "integration_branch": "agent-team/experiment",
                "concurrency": 3,
                "codex": str(self.probe),
                "timeout_seconds": 30,
            }
        )

    def git(self, *args):
        return subprocess.check_output(
            ["git", *args], cwd=self.repo, text=True, stderr=subprocess.DEVNULL
        ).strip()

    def task(self, name):
        return {
            "id": name,
            "title": name,
            "instructions": "isolated protocol fixture",
            "depends_on": [],
            "write_paths": [name + ".txt"],
        }

    def test_three_real_processes_overlap_and_explicit_integration_keeps_versions(self):
        self.store.add_tasks([self.task(name) for name in ("one", "two", "three")])
        result = Factory(self.root).run()
        self.assertEqual(result["peak_native_processes"], 3)
        self.assertEqual({task["status"] for task in result["tasks"]}, {"ready"})
        self.assertFalse((self.repo / "one.txt").exists())
        Factory(self.root).integrate("one")
        self.assertTrue((self.repo / "one.txt").exists())
        self.assertEqual(self.store.get("one")["status"], "merged")
        for task in self.store.snapshot()["tasks"]:
            self.assertTrue(task["result"]["commit"])

    def test_zero_exit_without_handoff_fails_and_does_not_replay(self):
        self.store.add_tasks([self.task("missing-handoff")])
        result = Factory(self.root).run()
        self.assertEqual(result["tasks"][0]["status"], "failed")
        self.assertIn("without a task handoff", result["tasks"][0]["result"]["message"])
        Factory(self.root).run()
        self.assertEqual(self.store.get("missing-handoff")["attempt"], 1)

    def test_dirty_integration_target_does_not_consume_pending_task(self):
        self.store.add_tasks([self.task("later")])
        (self.repo / "unrelated.txt").write_text(
            "integration conflict or unrelated edit"
        )
        with self.assertRaises(ValueError):
            Factory(self.root).run()
        self.assertEqual(self.store.get("later")["status"], "pending")
        self.assertEqual(self.store.get("later")["attempt"], 0)

    def test_explicit_resume_keeps_worktree_thread_and_original_native_evidence(self):
        self.store.add_tasks([self.task("missing-handoff")])
        Factory(self.root).run()
        old = self.store.get("missing-handoff")
        previous_events = Path(old["execution"]["native_directory"]) / "events.jsonl"
        original = previous_events.read_bytes()
        self.store.requeue(
            "missing-handoff", "Continue the preserved task after its known exit."
        )
        Factory(self.root).run()
        task = self.store.get("missing-handoff")
        self.assertEqual(task["status"], "ready")
        self.assertEqual(task["attempt"], 2)
        self.assertEqual(task["execution"]["worktree"], old["execution"]["worktree"])
        self.assertEqual(task["execution"]["thread_id"], old["execution"]["thread_id"])
        self.assertEqual(previous_events.read_bytes(), original)

    def test_explicit_ready_resume_keeps_original_session_at_candidate_commit(self):
        self.store.add_tasks([self.task("one")])
        Factory(self.root).run()
        original = self.store.get("one")
        self.store.requeue(
            "one", "Repeat real verification after environment preparation."
        )
        Factory(self.root).run()
        resumed = self.store.get("one")
        self.assertEqual(resumed["status"], "ready")
        self.assertEqual(
            resumed["execution"]["thread_id"], original["execution"]["thread_id"]
        )
        self.assertEqual(resumed["result"]["commit"], original["result"]["commit"])
        self.assertEqual(resumed["attempt"], 2)

    def test_integration_and_resume_refuse_an_active_controller_lock(self):
        self.store.add_tasks([self.task("one")])
        Factory(self.root).run()
        with (self.root / "controller.lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            with self.assertRaisesRegex(ValueError, "controller"):
                Factory(self.root).integrate("one")
            with self.assertRaisesRegex(ValueError, "controller"):
                self.store.requeue("one", "must not race integration")
        self.assertEqual(self.store.get("one")["status"], "ready")
        self.assertFalse((self.repo / "one.txt").exists())

    def test_resume_rejects_replacement_native_session(self):
        self.store.add_tasks([self.task("missing-handoff")])
        Factory(self.root).run()
        original = self.store.get("missing-handoff")["execution"]["thread_id"]
        (self.root / "fixture-replace-session").touch()
        self.store.requeue("missing-handoff", "Resume the existing session only.")
        Factory(self.root).run()
        task = self.store.get("missing-handoff")
        self.assertEqual(task["status"], "failed")
        self.assertEqual(task["execution"]["thread_id"], original)


if __name__ == "__main__":
    unittest.main()
