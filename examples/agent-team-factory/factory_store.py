"""Persistent task ownership and messages for one isolated factory experiment."""

import contextlib
import json
import re
import sqlite3
import time
from pathlib import Path


class Store:
    def __init__(self, root):
        self.root = Path(root).resolve()
        self.root.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.path = self.root / "factory.sqlite3"
        with self.connect() as db:
            db.executescript("""
                CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
                CREATE TABLE IF NOT EXISTS tasks (
                    id TEXT PRIMARY KEY, definition TEXT NOT NULL,
                    status TEXT NOT NULL DEFAULT 'pending', attempt INTEGER NOT NULL DEFAULT 0,
                    created REAL NOT NULL, started REAL, finished REAL,
                    submission TEXT, result TEXT, execution TEXT NOT NULL DEFAULT '{}'
                );
                CREATE TABLE IF NOT EXISTS messages (
                    seq INTEGER PRIMARY KEY, sender TEXT NOT NULL,
                    recipient TEXT NOT NULL, body TEXT NOT NULL, created REAL NOT NULL
                );
                CREATE TABLE IF NOT EXISTS events (
                    seq INTEGER PRIMARY KEY, task_id TEXT, kind TEXT NOT NULL,
                    detail TEXT NOT NULL, created REAL NOT NULL
                );
            """)
        self.path.chmod(0o600)

    @contextlib.contextmanager
    def connect(self):
        db = sqlite3.connect(self.path, timeout=30)
        db.row_factory = sqlite3.Row
        db.execute("PRAGMA foreign_keys=ON")
        try:
            with db:
                yield db
        finally:
            db.close()

    def initialize(self, settings):
        limit = settings.get("concurrency", 3)
        if type(limit) is not int or not 1 <= limit <= 20:
            raise ValueError("concurrency must be an integer between 1 and 20")
        settings = {**settings, "concurrency": limit}
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            old = db.execute("SELECT value FROM settings WHERE key='config'").fetchone()
            if old and json.loads(old[0]) != settings:
                raise ValueError("factory already initialized with different settings")
            db.execute(
                "INSERT OR IGNORE INTO settings VALUES ('config', ?)",
                (json.dumps(settings),),
            )

    def settings(self):
        with self.connect() as db:
            row = db.execute("SELECT value FROM settings WHERE key='config'").fetchone()
        if not row:
            raise ValueError("initialize this factory first")
        return json.loads(row[0])

    @staticmethod
    def validate_task(task):
        if not isinstance(task, dict) or not re.fullmatch(
            r"[a-z][a-z0-9-]{0,63}", task.get("id", "")
        ):
            raise ValueError(
                "task id must be a lowercase slug of at most 64 characters"
            )
        for field in ("title", "instructions"):
            if not isinstance(task.get(field), str) or not task[field].strip():
                raise ValueError(f"task {field} is required")
        for field in ("depends_on", "write_paths"):
            if not isinstance(task.get(field), list) or any(
                not isinstance(x, str) for x in task[field]
            ):
                raise ValueError(f"task {field} must be an array of strings")
        for pattern in task["write_paths"]:
            if (
                not pattern
                or pattern.startswith(("/", "~"))
                or ".." in pattern.split("/")
                or pattern.split("/")[0] == ".git"
                or "\\" in pattern
            ):
                raise ValueError("write_paths must stay inside the task worktree")

    def add_tasks(self, tasks):
        if not isinstance(tasks, list) or not tasks:
            raise ValueError("plan must contain tasks")
        for task in tasks:
            self.validate_task(task)
        ids = [task["id"] for task in tasks]
        if len(ids) != len(set(ids)):
            raise ValueError("duplicate task ids")
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            existing = {
                row["id"]: json.loads(row["definition"])
                for row in db.execute("SELECT id, definition FROM tasks")
            }
            if set(ids) & existing.keys():
                raise ValueError(
                    "task already exists; existing tasks are not overwritten"
                )
            combined = {**existing, **{t["id"]: t for t in tasks}}
            visiting, visited = set(), set()

            def visit(task_id):
                if task_id not in combined or task_id in visiting:
                    raise ValueError("unknown or cyclic task dependency")
                if task_id in visited:
                    return
                visiting.add(task_id)
                for dependency in combined[task_id]["depends_on"]:
                    visit(dependency)
                visiting.remove(task_id)
                visited.add(task_id)

            for task_id in combined:
                visit(task_id)
            for task in tasks:
                db.execute(
                    "INSERT INTO tasks(id, definition, created) VALUES (?, ?, ?)",
                    (task["id"], json.dumps(task), time.time()),
                )
                self.event(db, task["id"], "queued", {})

    @staticmethod
    def event(db, task_id, kind, detail):
        db.execute(
            "INSERT INTO events(task_id, kind, detail, created) VALUES (?, ?, ?, ?)",
            (task_id, kind, json.dumps(detail), time.time()),
        )

    @staticmethod
    def decode(row):
        task = {**json.loads(row["definition"]), **dict(row)}
        del task["definition"]
        for field in ("submission", "result", "execution"):
            task[field] = json.loads(task[field]) if task[field] else None
        return task

    def get(self, task_id):
        with self.connect() as db:
            row = db.execute("SELECT * FROM tasks WHERE id=?", (task_id,)).fetchone()
        if not row:
            raise ValueError("unknown task")
        return self.decode(row)

    def claim(self):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            config = json.loads(
                db.execute("SELECT value FROM settings WHERE key='config'").fetchone()[
                    0
                ]
            )
            running = db.execute(
                "SELECT count(*) FROM tasks WHERE status='running'"
            ).fetchone()[0]
            if running >= config["concurrency"]:
                return None
            states = {
                row[0]: row[1] for row in db.execute("SELECT id, status FROM tasks")
            }
            for row in db.execute(
                "SELECT * FROM tasks WHERE status='pending' ORDER BY created, id"
            ):
                task = self.decode(row)
                if any(
                    states.get(dependency) != "merged"
                    for dependency in task["depends_on"]
                ):
                    continue
                db.execute(
                    "UPDATE tasks SET status='running', attempt=attempt+1, started=?, finished=NULL WHERE id=?",
                    (time.time(), task["id"]),
                )
                self.event(db, task["id"], "started", {"attempt": task["attempt"] + 1})
                return self.decode(
                    db.execute(
                        "SELECT * FROM tasks WHERE id=?", (task["id"],)
                    ).fetchone()
                )
        return None

    def execution(self, task_id, **values):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            row = db.execute(
                "SELECT execution FROM tasks WHERE id=?", (task_id,)
            ).fetchone()
            if not row:
                raise ValueError("unknown task")
            detail = {**json.loads(row[0]), **values}
            db.execute(
                "UPDATE tasks SET execution=? WHERE id=?", (json.dumps(detail), task_id)
            )

    def submit(self, task_id, report, attempt=None):
        if (
            not isinstance(report, dict)
            or report.get("status") not in ("completed", "blocked")
            or not isinstance(report.get("message"), str)
            or not report["message"].strip()
            or not isinstance(report.get("artifacts"), list)
            or any(not isinstance(x, str) for x in report["artifacts"])
        ):
            raise ValueError("handoff requires status, message and artifact paths")
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            row = db.execute("SELECT * FROM tasks WHERE id=?", (task_id,)).fetchone()
            if (
                not row
                or row["status"] != "running"
                or (attempt is not None and row["attempt"] != attempt)
            ):
                raise ValueError("this execution no longer owns the task")
            if row["submission"] and json.loads(row["submission"]) != report:
                raise ValueError(
                    "different handoff already submitted; original preserved"
                )
            if not row["submission"]:
                db.execute(
                    "UPDATE tasks SET submission=? WHERE id=?",
                    (json.dumps(report), task_id),
                )
                self.event(db, task_id, "handoff", report)

    def finish(self, task_id, status, result):
        if status not in (
            "ready",
            "failed",
            "blocked",
            "interrupted",
            "merged",
            "conflict",
        ):
            raise ValueError("invalid completion state")
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            row = db.execute(
                "SELECT status FROM tasks WHERE id=?", (task_id,)
            ).fetchone()
            if not row or (
                row[0] != "running"
                and not (
                    row[0] in ("ready", "conflict") and status in ("merged", "conflict")
                )
            ):
                raise ValueError("task is not eligible for this state change")
            db.execute(
                "UPDATE tasks SET status=?, result=?, finished=? WHERE id=?",
                (status, json.dumps(result), time.time(), task_id),
            )
            self.event(db, task_id, status, result)

    def message(self, sender, recipient, body):
        if not isinstance(body, str) or not body.strip() or len(body) > 50000:
            raise ValueError("message must contain between 1 and 50000 characters")
        with self.connect() as db:
            known = {row[0] for row in db.execute("SELECT id FROM tasks")} | {
                "leader",
                "user",
            }
            if sender not in known or recipient not in known:
                raise ValueError("message participant is not part of this factory")
            db.execute(
                "INSERT INTO messages(sender, recipient, body, created) VALUES (?, ?, ?, ?)",
                (sender, recipient, body, time.time()),
            )

    def snapshot(self):
        with self.connect() as db:
            tasks = [
                self.decode(row)
                for row in db.execute("SELECT * FROM tasks ORDER BY created, id")
            ]
            messages = [
                dict(row) for row in db.execute("SELECT * FROM messages ORDER BY seq")
            ]
            events = [
                {**dict(row), "detail": json.loads(row["detail"])}
                for row in db.execute("SELECT * FROM events ORDER BY seq")
            ]
        active = peak = native_active = native_peak = 0
        for event in events:
            if event["kind"] == "started":
                active += 1
                peak = max(peak, active)
            elif event["kind"] in ("ready", "failed", "blocked", "interrupted"):
                active -= 1
            if event["task_id"] and event["kind"] == "native_started":
                native_active += 1
                native_peak = max(native_peak, native_active)
            elif event["task_id"] and event["kind"] == "native_finished":
                native_active -= 1
        return {
            "tasks": tasks,
            "messages": messages,
            "events": events,
            "peak_claimed": peak,
            "peak_native_processes": native_peak,
            "running": sum(t["status"] == "running" for t in tasks),
        }
