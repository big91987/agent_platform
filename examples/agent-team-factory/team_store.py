"""Scratch, single-user Agent Team state. No software-development stage graph."""

import contextlib
import json
import sqlite3
import time
import uuid
from pathlib import Path

SKILLS = ("research", "prd", "architecture", "development", "testing")


def identifier(prefix):
    return prefix + "-" + uuid.uuid4().hex[:12]


def unpack(row):
    if row is None:
        raise ValueError("记录不存在")
    value = dict(row)
    for key in ("execution", "depends_on", "submission", "artifacts"):
        if key in value:
            value[key] = json.loads(value[key])
    return value


def text(value, name, limit=40000):
    if not isinstance(value, str) or not value.strip() or len(value) > limit:
        raise ValueError(name + "不能为空或超过长度限制")
    return value.strip()


class TeamStore:
    def __init__(self, root):
        self.root = Path(root).resolve()
        self.root.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.path = self.root / "playground.sqlite3"
        with self.connect() as db:
            db.executescript("""
            CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK(id=1), config TEXT NOT NULL,
                paused INTEGER NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 0,
                leader_seen INTEGER NOT NULL DEFAULT 0);
            CREATE TABLE IF NOT EXISTS requests (id TEXT PRIMARY KEY, client_id TEXT UNIQUE NOT NULL,
                body TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', result TEXT NOT NULL DEFAULT '',
                created REAL NOT NULL, updated REAL NOT NULL);
            CREATE TABLE IF NOT EXISTS actors (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'idle',
                active_turn TEXT, execution TEXT NOT NULL DEFAULT '{}', error TEXT NOT NULL DEFAULT '');
            CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, request_id TEXT NOT NULL REFERENCES requests(id),
                task_key TEXT NOT NULL, title TEXT NOT NULL, instructions TEXT NOT NULL, skill TEXT NOT NULL,
                depends_on TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'queued', worker TEXT,
                submission TEXT NOT NULL DEFAULT 'null', result TEXT NOT NULL DEFAULT '',
                artifacts TEXT NOT NULL DEFAULT '[]', worktree TEXT, branch TEXT, base_commit TEXT,
                candidate TEXT, integration_commit TEXT, created REAL NOT NULL, finished REAL,
                UNIQUE(request_id,task_key));
            CREATE TABLE IF NOT EXISTS turns (id TEXT PRIMARY KEY, actor TEXT NOT NULL REFERENCES actors(id),
                request_id TEXT REFERENCES requests(id), task_id TEXT REFERENCES tasks(id),
                status TEXT NOT NULL DEFAULT 'running', claimed_revision INTEGER,
                execution TEXT NOT NULL DEFAULT '{}', reply TEXT NOT NULL DEFAULT '',
                created REAL NOT NULL, finished REAL);
            CREATE TABLE IF NOT EXISTS messages (seq INTEGER PRIMARY KEY, request_id TEXT REFERENCES requests(id),
                sender TEXT NOT NULL, recipient TEXT NOT NULL, body TEXT NOT NULL, created REAL NOT NULL);
            CREATE TABLE IF NOT EXISTS events (seq INTEGER PRIMARY KEY, actor TEXT, turn_id TEXT,
                request_id TEXT, task_id TEXT, kind TEXT NOT NULL, detail TEXT NOT NULL, created REAL NOT NULL);
            """)
        self.path.chmod(0o600)

    @contextlib.contextmanager
    def connect(self):
        db = sqlite3.connect(self.path, timeout=15)
        db.row_factory = sqlite3.Row
        db.execute("PRAGMA foreign_keys=ON")
        try:
            with db:
                yield db
        finally:
            db.close()

    def initialize(self, config):
        if (
            type(config["concurrency"]) is not int
            or not 1 <= config["concurrency"] <= 20
        ):
            raise ValueError("Worker数量应为1到20")
        with self.connect() as db:
            if db.execute("SELECT 1 FROM settings").fetchone():
                raise ValueError("已有运行库，请使用原配置启动")
            db.execute(
                "INSERT INTO settings(id,config) VALUES (1,?)", (json.dumps(config),)
            )
            for actor in [
                "leader",
                *[f"worker-{n + 1}" for n in range(config["concurrency"])],
            ]:
                db.execute("INSERT INTO actors(id) VALUES (?)", (actor,))

    def settings(self):
        with self.connect() as db:
            row = db.execute("SELECT config FROM settings WHERE id=1").fetchone()
            if not row:
                raise ValueError("请先初始化实验台")
            return json.loads(row[0])

    def event(self, db, actor, kind, detail, request_id=None, task_id=None):
        row = db.execute(
            "SELECT active_turn FROM actors WHERE id=?", (actor,)
        ).fetchone()
        turn_id = row[0] if row else None
        if turn_id:
            turn = db.execute("SELECT * FROM turns WHERE id=?", (turn_id,)).fetchone()
            request_id = request_id or turn["request_id"]
            task_id = task_id or turn["task_id"]
        db.execute(
            "INSERT INTO events(actor,turn_id,request_id,task_id,kind,detail,created) VALUES (?,?,?,?,?,?,?)",
            (
                actor,
                turn_id,
                request_id,
                task_id,
                kind,
                json.dumps(detail, ensure_ascii=False),
                time.time(),
            ),
        )

    @staticmethod
    def wake(db):
        db.execute("UPDATE settings SET revision=revision+1 WHERE id=1")

    def add_request(self, body, client_id):
        body = text(body, "需求")
        client_id = text(client_id, "提交标识", 100)
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            old = db.execute(
                "SELECT * FROM requests WHERE client_id=?", (client_id,)
            ).fetchone()
            if old:
                if old["body"] != body:
                    raise ValueError("同一次提交内容发生变化")
                return unpack(old)
            request_id = identifier("req")
            now = time.time()
            db.execute(
                "INSERT INTO requests(id,client_id,body,created,updated) VALUES (?,?,?,?,?)",
                (request_id, client_id, body, now, now),
            )
            db.execute(
                "INSERT INTO messages(request_id,sender,recipient,body,created) VALUES (?,'user','leader',?,?)",
                (request_id, body, now),
            )
            self.event(db, "user", "request_submitted", {}, request_id)
            self.wake(db)
            return unpack(
                db.execute(
                    "SELECT * FROM requests WHERE id=?", (request_id,)
                ).fetchone()
            )

    def message(self, sender, recipient, body, request_id):
        body = text(body, "消息")
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            unpack(
                db.execute(
                    "SELECT * FROM requests WHERE id=?", (request_id,)
                ).fetchone()
            )
            if (
                recipient not in ["user", "leader"]
                and not db.execute(
                    "SELECT 1 FROM actors WHERE id=?", (recipient,)
                ).fetchone()
            ):
                raise ValueError("消息接收方不存在")
            db.execute(
                "INSERT INTO messages(request_id,sender,recipient,body,created) VALUES (?,?,?,?,?)",
                (request_id, sender, recipient, body, time.time()),
            )
            if sender == "user":
                db.execute(
                    "UPDATE requests SET status='active',updated=? WHERE id=?",
                    (time.time(), request_id),
                )
            self.event(db, sender, "message_sent", {"recipient": recipient}, request_id)
            if recipient == "leader":
                self.wake(db)
            return {"saved": True}

    def create_task(self, args):
        request_id = args["request_id"]
        key = text(args["key"], "任务key", 80)
        title = text(args["title"], "标题", 160)
        instructions = text(args["instructions"], "任务说明")
        skill = args["skill"]
        deps = args.get("depends_on", [])
        if (
            skill not in SKILLS
            or not isinstance(deps, list)
            or any(not isinstance(x, str) for x in deps)
            or len(set(deps)) != len(deps)
        ):
            raise ValueError("Skill或依赖不合法")
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            req = unpack(
                db.execute(
                    "SELECT * FROM requests WHERE id=?", (request_id,)
                ).fetchone()
            )
            if req["status"] == "completed":
                raise ValueError("需求已完成；请用户补充后再派发")
            old = db.execute(
                "SELECT * FROM tasks WHERE request_id=? AND task_key=?",
                (request_id, key),
            ).fetchone()
            if old:
                value = unpack(old)
                if (
                    value["title"],
                    value["instructions"],
                    value["skill"],
                    value["depends_on"],
                ) != (title, instructions, skill, deps):
                    raise ValueError("同一任务key对应的内容不同")
                return value
            for dep in deps:
                row = db.execute(
                    "SELECT request_id FROM tasks WHERE id=?", (dep,)
                ).fetchone()
                if not row or row[0] != request_id:
                    raise ValueError("依赖必须是本需求中已存在的任务")
            task_id = identifier("task")
            db.execute(
                "INSERT INTO tasks(id,request_id,task_key,title,instructions,skill,depends_on,created) VALUES (?,?,?,?,?,?,?,?)",
                (
                    task_id,
                    request_id,
                    key,
                    title,
                    instructions,
                    skill,
                    json.dumps(deps),
                    time.time(),
                ),
            )
            self.event(
                db,
                "leader",
                "task_created",
                {"skill": skill, "title": title},
                request_id,
                task_id,
            )
            return unpack(
                db.execute("SELECT * FROM tasks WHERE id=?", (task_id,)).fetchone()
            )

    def task(self, task_id):
        with self.connect() as db:
            return unpack(
                db.execute("SELECT * FROM tasks WHERE id=?", (task_id,)).fetchone()
            )

    def claim(self, actor):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            config = db.execute("SELECT * FROM settings WHERE id=1").fetchone()
            worker = db.execute("SELECT * FROM actors WHERE id=?", (actor,)).fetchone()
            if config["paused"] or not worker or worker["status"] != "idle":
                return None
            for row in db.execute(
                "SELECT t.* FROM tasks t JOIN requests r ON t.request_id=r.id WHERE t.status='queued' AND r.status!='completed' ORDER BY t.created,t.id"
            ).fetchall():
                task = unpack(row)
                if any(
                    db.execute(
                        "SELECT status FROM tasks WHERE id=?", (dep,)
                    ).fetchone()[0]
                    != "accepted"
                    for dep in task["depends_on"]
                ):
                    continue
                db.execute(
                    "UPDATE tasks SET status='running',worker=? WHERE id=?",
                    (actor, task["id"]),
                )
                turn_id = self._turn(db, actor, task["request_id"], task["id"])
                self.event(
                    db, actor, "task_claimed", {}, task["request_id"], task["id"]
                )
                return {
                    "turn_id": turn_id,
                    "task": unpack(
                        db.execute(
                            "SELECT * FROM tasks WHERE id=?", (task["id"],)
                        ).fetchone()
                    ),
                }
            return None

    def _turn(self, db, actor, request_id=None, task_id=None, revision=None):
        turn_id = identifier("turn")
        db.execute(
            "INSERT INTO turns(id,actor,request_id,task_id,claimed_revision,created) VALUES (?,?,?,?,?,?)",
            (turn_id, actor, request_id, task_id, revision, time.time()),
        )
        db.execute(
            "UPDATE actors SET status='running',active_turn=?,execution='{}',error='' WHERE id=?",
            (turn_id, actor),
        )
        return turn_id

    def claim_leader(self):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            cfg = db.execute("SELECT * FROM settings").fetchone()
            actor = db.execute("SELECT * FROM actors WHERE id='leader'").fetchone()
            if (
                cfg["paused"]
                or actor["status"] != "idle"
                or cfg["revision"] <= cfg["leader_seen"]
            ):
                return None
            turn_id = self._turn(db, "leader", revision=cfg["revision"])
            inputs = [
                x[0]
                for x in db.execute("SELECT id FROM requests WHERE status!='completed'")
            ]
            execution = json.dumps({"request_ids": inputs})
            db.execute("UPDATE turns SET execution=? WHERE id=?", (execution, turn_id))
            db.execute("UPDATE actors SET execution=? WHERE id='leader'", (execution,))
            return turn_id

    def authorize(self, actor, turn_id):
        with self.connect() as db:
            row = db.execute("SELECT * FROM actors WHERE id=?", (actor,)).fetchone()
            if not row or row["active_turn"] != turn_id or row["status"] != "running":
                raise ValueError("本轮执行身份已失效")
            return unpack(
                db.execute("SELECT * FROM turns WHERE id=?", (turn_id,)).fetchone()
            )

    def execution(self, actor, **fields):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            row = unpack(
                db.execute("SELECT * FROM actors WHERE id=?", (actor,)).fetchone()
            )
            execution = {**row["execution"], **fields}
            db.execute(
                "UPDATE actors SET execution=? WHERE id=?",
                (json.dumps(execution), actor),
            )
            if row["active_turn"]:
                db.execute(
                    "UPDATE turns SET execution=? WHERE id=?",
                    (json.dumps(execution), row["active_turn"]),
                )

    def handoff(self, task_id, args):
        if args["status"] not in ["completed", "blocked"]:
            raise ValueError("交接状态无效")
        text(args["message"], "交接说明")
        artifacts = args.get("artifacts", [])
        if not isinstance(artifacts, list) or any(
            not isinstance(x, str) or Path(x).is_absolute() or ".." in Path(x).parts
            for x in artifacts
        ):
            raise ValueError("产物必须是工作区内的相对文件路径")
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            task = unpack(
                db.execute("SELECT * FROM tasks WHERE id=?", (task_id,)).fetchone()
            )
            if task["status"] != "running":
                raise ValueError("任务已不在执行")
            if task["submission"] and task["submission"] != args:
                raise ValueError("已经交接，不覆盖原结果")
            db.execute(
                "UPDATE tasks SET submission=? WHERE id=?",
                (json.dumps(args, ensure_ascii=False), task_id),
            )
            self.event(
                db,
                task["worker"],
                "handoff_received",
                {"status": args["status"]},
                task["request_id"],
                task_id,
            )
            return {
                "saved": True,
                "instruction": "交接已保存。结束本轮，不再编辑文件。原生退出后才确认任务结果。",
            }

    def finish_task(self, task_id, status, result, artifacts=None, candidate=None):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            task = unpack(
                db.execute("SELECT * FROM tasks WHERE id=?", (task_id,)).fetchone()
            )
            db.execute(
                "UPDATE tasks SET status=?,result=?,artifacts=?,candidate=?,finished=? WHERE id=?",
                (
                    status,
                    result,
                    json.dumps(artifacts or []),
                    candidate,
                    time.time(),
                    task_id,
                ),
            )
            self.event(
                db,
                task["worker"],
                "task_" + status,
                {"result": result},
                task["request_id"],
                task_id,
            )
            self.wake(db)

    def finish_turn(self, turn_id, result, reply):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            turn = unpack(
                db.execute("SELECT * FROM turns WHERE id=?", (turn_id,)).fetchone()
            )
            error = result.get("reason") or (
                "native exit " + str(result["returncode"])
                if result.get("returncode")
                else ""
            )
            db.execute(
                "UPDATE turns SET status=?,reply=?,finished=? WHERE id=?",
                ("failed" if error else "completed", reply, time.time(), turn_id),
            )
            stopped = error and (
                turn["actor"] == "leader" or result.get("termination_error")
            )
            db.execute(
                "UPDATE actors SET status=?,active_turn=NULL,error=? WHERE id=?",
                ("error" if stopped else "idle", error, turn["actor"]),
            )
            if turn["actor"] == "leader":
                db.execute(
                    "UPDATE settings SET leader_seen=? WHERE id=1",
                    (turn["claimed_revision"],),
                )

    def snapshot(self):
        with self.connect() as db:
            cfg = db.execute("SELECT * FROM settings").fetchone()
            return {
                "requests": [
                    unpack(x)
                    for x in db.execute("SELECT * FROM requests ORDER BY created DESC")
                ],
                "tasks": [
                    unpack(x)
                    for x in db.execute("SELECT * FROM tasks ORDER BY created")
                ],
                "actors": [
                    unpack(x) for x in db.execute("SELECT * FROM actors ORDER BY id")
                ],
                "turns": [
                    unpack(x)
                    for x in db.execute("SELECT * FROM turns ORDER BY created")
                ],
                "messages": [
                    dict(x) for x in db.execute("SELECT * FROM messages ORDER BY seq")
                ],
                "events": [
                    {**dict(x), "detail": json.loads(x["detail"])}
                    for x in db.execute(
                        "SELECT * FROM events ORDER BY seq DESC LIMIT 300"
                    )
                ],
                "paused": bool(cfg["paused"]),
                "captured": time.time(),
            }

    def pause(self, value):
        if type(value) is not bool:
            raise ValueError("暂停值无效")
        with self.connect() as db:
            db.execute("UPDATE settings SET paused=? WHERE id=1", (int(value),))

    def continue_leader(self):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            actor = db.execute("SELECT * FROM actors WHERE id='leader'").fetchone()
            if actor["active_turn"]:
                raise ValueError("Leader还在执行")
            db.execute("UPDATE actors SET status='idle',error='' WHERE id='leader'")
            self.wake(db)
