"""Run the user's installed native Codex without bypassing its sandbox or approval."""

import json
import os
import selectors
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path


class Native:
    def __init__(self, store, stop=None):
        self.store = store
        self.stop = stop or threading.Event()

    def run(self, name, workspace, prompt, *, task=None, schema=None, thread_id=None):
        settings = self.store.settings()
        directory = self.store.root / "native" / name
        directory.mkdir(parents=True, mode=0o700)
        prompt_path = directory / "input.md"
        if prompt_path.exists():
            raise ValueError("native attempt already exists; refusing uncertain replay")
        prompt_path.write_text(prompt)
        argv = [
            settings.get("codex", "codex"),
            "-C",
            str(workspace),
            "-c",
            'sandbox_mode="workspace-write"'
            if task and task["write_paths"]
            else 'sandbox_mode="read-only"',
        ]
        if settings.get("model"):
            argv += ["-m", settings["model"]]
        if task:
            server = Path(__file__).with_name("mcp_server.py").resolve()
            prefix = "mcp_servers.agent_team_factory"
            server_args = [
                str(server),
                "--root",
                str(self.store.root),
                "--task",
                task["id"],
                "--attempt",
                str(task["attempt"]),
            ]
            argv += [
                "-c",
                f"{prefix}.command={json.dumps(sys.executable)}",
                "-c",
                f"{prefix}.args={json.dumps(server_args)}",
                "-c",
                f'{prefix}.default_tools_approval_mode="prompt"',
                "-c",
                f'{prefix}.enabled_tools=["get_task","send_message","handoff"]',
            ]
            # exec cannot prompt. Only these task-bound local operations are
            # pre-authorized; shell and other MCP permissions are unchanged.
            for tool in ("get_task", "send_message", "handoff"):
                argv += ["-c", f'{prefix}.tools.{tool}.approval_mode="approve"']
        argv += ["exec"]
        if thread_id:
            argv += ["resume", thread_id]
        argv += ["--json", "--color", "never"] if not thread_id else ["--json"]
        argv += ["--output-last-message", str(directory / "reply.md")]
        if schema:
            argv += ["--output-schema", str(schema)]
        argv += ["-"]
        started = time.time()
        task_id = task["id"] if task else None
        (directory / "invocation.json").write_text(
            json.dumps(
                {"argv": argv, "cwd": str(workspace), "started": started}, indent=2
            )
        )
        status = {
            "started": started,
            "thread_id": thread_id,
            "native_directory": str(directory),
        }
        with (
            (directory / "events.jsonl").open("wb") as output,
            (directory / "stderr.log").open("wb") as error,
        ):
            process = subprocess.Popen(
                argv,
                cwd=workspace,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=error,
                start_new_session=True,
            )
            status["pid"] = process.pid
            if task:
                self.store.execution(task_id, **status)
            with self.store.connect() as db:
                self.store.event(
                    db, task_id, "native_started", {"name": name, "pid": process.pid}
                )
            selector = selectors.DefaultSelector()
            pending = b""
            deadline = time.monotonic() + settings.get("timeout_seconds", 1800)
            reason = None
            drain_deadline = None
            termination_sent = False
            try:
                os.set_blocking(process.stdout.fileno(), False)
                os.set_blocking(process.stdin.fileno(), False)
                selector.register(process.stdout, selectors.EVENT_READ, "output")
                payload = memoryview(prompt.encode())
                offset = 0
                if payload:
                    selector.register(process.stdin, selectors.EVENT_WRITE, "input")
                else:
                    process.stdin.close()
                while selector.get_map() or process.poll() is None:
                    if not termination_sent and (
                        reason is not None
                        or self.stop.is_set()
                        or time.monotonic() > deadline
                    ):
                        reason = reason or (
                            "interrupted" if self.stop.is_set() else "timeout"
                        )
                        self.terminate(process)
                        termination_sent = True
                        drain_deadline = time.monotonic() + 1
                    if drain_deadline and time.monotonic() > drain_deadline:
                        break
                    for key, _ in selector.select(0.1):
                        if key.data == "input":
                            try:
                                offset += os.write(
                                    key.fileobj.fileno(),
                                    payload[offset : offset + 65536],
                                )
                            except BlockingIOError:
                                continue
                            except BrokenPipeError:
                                if reason is None:
                                    reason = "stdin_closed"
                                offset = len(payload)
                            if offset == len(payload):
                                selector.unregister(key.fileobj)
                                key.fileobj.close()
                            continue
                        try:
                            data = os.read(key.fileobj.fileno(), 65536)
                        except BlockingIOError:
                            continue
                        if not data:
                            selector.unregister(key.fileobj)
                            continue
                        output.write(data)
                        output.flush()
                        pending += data
                        while b"\n" in pending:
                            raw, pending = pending.split(b"\n", 1)
                            self.observe(
                                raw, task_id, status, expected_thread=thread_id
                            )
                process.wait(timeout=10)
            except Exception as failure:
                reason = reason or "execution_error"
                status["error"] = str(failure)
            finally:
                selector.close()
                if not process.stdin.closed:
                    process.stdin.close()
                process.stdout.close()
                if not termination_sent:
                    try:
                        self.terminate(process)
                    except (OSError, subprocess.TimeoutExpired) as failure:
                        status["termination_error"] = str(failure)
                        self.stop.set()
            if pending:
                try:
                    self.observe(pending, task_id, status, expected_thread=thread_id)
                except ValueError as failure:
                    reason = reason or "execution_error"
                    status["error"] = str(failure)
            status.update(
                returncode=process.returncode, finished=time.time(), reason=reason
            )
        (directory / "status.json").write_text(json.dumps(status, indent=2))
        if task:
            self.store.execution(task_id, **status)
        with self.store.connect() as db:
            self.store.event(
                db,
                task_id,
                "native_finished",
                {"name": name, "returncode": process.returncode, "reason": reason},
            )
        return status

    def observe(self, raw, task_id, status, expected_thread=None):
        try:
            event = json.loads(raw)
        except (ValueError, UnicodeDecodeError):
            return
        kind = event.get("type")
        if kind == "thread.started":
            if expected_thread and event.get("thread_id") != expected_thread:
                status["session_mismatch"] = {
                    "expected": expected_thread,
                    "observed": event.get("thread_id"),
                }
                raise ValueError(
                    "native resumed a different session; original session ID preserved"
                )
            status["thread_id"] = event.get("thread_id")
            if task_id:
                self.store.execution(task_id, thread_id=status["thread_id"])
        if kind in ("turn.started", "turn.completed", "turn.failed"):
            with self.store.connect() as db:
                self.store.event(
                    db,
                    task_id,
                    "native_" + kind,
                    {"thread_id": status.get("thread_id"), "usage": event.get("usage")},
                )

    @staticmethod
    def terminate(process):
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            process.wait(timeout=1)
        except subprocess.TimeoutExpired:
            pass
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait(timeout=2)
