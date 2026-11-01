"""Fixed consumers and event-driven Leader turns. Agents own all stage choices."""

import contextlib
import fcntl
import json
import subprocess
import threading
import time
from pathlib import Path

from git_workspace import Workspace, git
from native import Native
from team_store import SKILLS, TeamStore, text, unpack


def skill_body(name):
    if name not in SKILLS:
        raise ValueError("未知Skill")
    return (Path(__file__).parent / "team_skills" / name / "SKILL.md").read_text()


class Team:
    def __init__(self, root):
        self.store = TeamStore(root)
        self.config = self.store.settings()
        self.root = self.store.root
        self.repo = Path(self.config["repository"])
        self.workspace = Workspace(
            self.repo, self.root, self.config["integration_branch"]
        )
        self.stop = threading.Event()
        self.threads = []

    @contextlib.contextmanager
    def git_lock(self):
        with (self.root / "git.lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            try:
                yield
            finally:
                fcntl.flock(lock, fcntl.LOCK_UN)

    def start(self):
        # Hold a process lifetime lock. Restart never silently replays running turns.
        self.lock = (self.root / "service.lock").open("a")
        try:
            fcntl.flock(self.lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise ValueError("已有实验服务在运行") from error
        state = self.store.snapshot()
        if any(x["active_turn"] for x in state["actors"]):
            raise ValueError("保留了未结束执行，拒绝重启重放；先核验原进程和轮次")
        for actor in [x["id"] for x in state["actors"]]:
            thread = threading.Thread(target=self.consume, args=(actor,), daemon=True)
            thread.start()
            self.threads.append(thread)

    def close(self):
        self.stop.set()
        for thread in self.threads:
            thread.join(timeout=15)
        if hasattr(self, "lock"):
            self.lock.close()

    def consume(self, actor):
        while not self.stop.is_set():
            turn_id = None
            task = None
            try:
                if actor == "leader":
                    turn_id = self.store.claim_leader()
                else:
                    claimed = self.store.claim(actor)
                    if claimed:
                        turn_id, task = claimed["turn_id"], claimed["task"]
                if turn_id:
                    self.execute(actor, turn_id, task)
                    continue
            except Exception as error:
                if task:
                    self.store.finish_task(task["id"], "failed", str(error))
                if turn_id:
                    self.store.finish_turn(
                        turn_id, {"reason": str(error), "returncode": 1}, ""
                    )
                with self.store.connect() as db:
                    self.store.event(
                        db, actor, "execution_error", {"error": str(error)}
                    )
            self.stop.wait(0.5)

    def execute(self, actor, turn_id, task):
        from team_mcp import tools_for

        thread_id = None
        execution_env = None
        if task:
            with self.git_lock():
                work = self.workspace.prepare(task["id"], 1)
            with self.store.connect() as db:
                db.execute(
                    "UPDATE tasks SET worktree=?,branch=?,base_commit=? WHERE id=?",
                    (str(work["path"]), work["branch"], work["base"], task["id"]),
                )
            prompt = self.worker_prompt(actor, task)
            cwd = work["path"]
            cache = cwd / ".cache"
            cache.mkdir(exist_ok=True)
            execution_env = {
                "GOCACHE": str(cache / "go-build"),
                "GOMODCACHE": str(cache / "go-mod"),
                "npm_config_cache": str(cache / "npm"),
                "UV_CACHE_DIR": str(cache / "uv"),
            }
        else:
            with self.store.connect() as db:
                last = db.execute(
                    "SELECT execution FROM turns WHERE actor='leader' AND id!=? ORDER BY created DESC LIMIT 1",
                    (turn_id,),
                ).fetchone()
            if last:
                old = json.loads(last[0])
                thread_id = old.get("thread_id")
                if not thread_id and old.get("native_directory"):
                    raise ValueError("上一轮Leader缺少原生会话标识，不能静默新建替代")
            prompt = self.leader_prompt()
            cwd = self.repo
        mcp = {
            "name": "team_playground",
            "entry": Path(__file__).with_name("team_mcp.py").resolve(),
            "args": ["--root", str(self.root), "--actor", actor, "--turn", turn_id],
            "tools": [x["name"] for x in tools_for(actor)],
        }
        result = Native(self.store, self.stop).run(
            turn_id,
            cwd,
            prompt,
            task={"id": actor, "attempt": 1, "write_paths": ["**"] if task else []},
            thread_id=thread_id,
            mcp=mcp,
            execution_env=execution_env,
        )
        reply_path = Path(result["native_directory"]) / "reply.md"
        reply = reply_path.read_text() if reply_path.exists() else ""
        if task:
            value = self.store.task(task["id"])
            submission = value["submission"]
            if result.get("reason") or result["returncode"]:
                self.store.finish_task(
                    task["id"],
                    "failed",
                    json.dumps(
                        {"exit": result["returncode"], "reason": result.get("reason")},
                        ensure_ascii=False,
                    ),
                )
            elif not submission:
                self.store.finish_task(
                    task["id"],
                    "failed",
                    "原生退出，但没有真实handoff；原始输出和工作区已保留",
                )
            elif submission["status"] == "blocked":
                self.store.finish_task(
                    task["id"],
                    "blocked",
                    submission["message"],
                    submission["artifacts"],
                )
            else:
                for relative in submission["artifacts"]:
                    p = (cwd / relative).resolve()
                    if not p.is_relative_to(cwd.resolve()) or not p.is_file():
                        raise ValueError("交接产物不存在或超出工作区")
                with self.git_lock():
                    candidate = self.workspace.commit(
                        {**task, "write_paths": ["**"]}, work
                    )
                self.store.finish_task(
                    task["id"],
                    "review",
                    submission["message"],
                    submission["artifacts"],
                    candidate,
                )
        self.store.finish_turn(turn_id, result, reply)

    def leader_prompt(self):
        state = self.store.snapshot()
        requests = [
            {k: x[k] for k in ["id", "body", "status", "result"]}
            for x in state["requests"]
            if x["status"] != "completed"
        ]
        return """# Agent Team Leader
你是这个团队的持续协调者。下面是真实用户在页面提交的需求，不是演示数据。
你拥有编排决定权；基础设施只管持久化、分配空闲Worker和执行生命周期。
先通过原生工具发现入口查找名称包含team_playground的工具，再真实调用get_board。
工具可能延迟加载；若有exec/ALL_TOOLS，找到真实名称后通过tools对象调用，不用Shell/Python读写任务库。

## 如何工作
- 理解需求和仓库，决定是否需要研究、PRD、架构、开发、测试，以及它们的先后/并行；没有固定阶段。
- create_task把具体工作写入队列，指定一个research/prd/architecture/development/testing Skill。Worker消费者已启动，你不用启动Worker。
- 用每个需求内稳定、可读的key防止同一任务重复创建。依赖只引用已返回的任务ID；前置成果accepted后才会分配。
- 每个Worker有独立工作区。你只读集成仓，通过accept_task明确接受并合入成果；不接受未检查的自报，也不自己编写产品实现或代替测试执行。
- 有冲突时查看原候选与集成版本，创建有具体材料的解决任务；取消已被替代的非运行任务。不要反复调用失败的接受动作。
- 用send_message给用户/Worker沟通具体问题和安排，不编造用户要求。Worker的讨论不是用户授权。
- 等待Worker时结束本轮即可；结果或新消息会自动唤醒你、接续本会话，不要忙等、sleep或轮询消耗模型。
- 成果满足需求后用finish_request向用户交付；需要用户信息时标needs_input并写具体问题。
- 不创建外部Issue/PR，不推送GitHub，不部署到外部服务，不使用旧Agent Platform流程。
- 原生消息、工具调用、结果和本轮实际输入会在私有页面展示。思考摘要有就展示，不补造隐含思维。

## 本轮仍在处理的真实需求
""" + json.dumps(requests, ensure_ascii=False, indent=2)

    def worker_prompt(self, actor, task):
        return f"""# Agent Team Worker
你是固定消费者{actor}，当前执行一个具体任务，岗位不固定。
先通过原生工具目录发现team_playground.get_task并真实调用，获取任务、依赖成果和消息。
若工具延迟加载，使用原生exec/ALL_TOOLS发现准确名称再通过tools对象调用，不自行读写任务库。

## 任务
{task["title"]}
{task["instructions"]}

## 本任务实际加载的Skill：{task["skill"]}
{skill_body(task["skill"])}

## 执行约定
- 当前工作区专属于本任务。按仓库AGENTS.md和真实需求工作，只修改任务需要的内容。
- 不操作其他工作区或共享运行库。不自行git commit/push/merge/checkout/switch，不创建外部Issue/PR，不部署外部服务。
- 原始日志、截图、凭据和数据库保存在ignored目录；交付文档只引用必要的脱敏证据。
- 问题或协作使用send_message，接收方可为leader、user或其他worker-N；get_task可再次读取消息。
- 完成后真实调用handoff(status=completed,message=实际结果及验证,artifacts=存在的相对文件路径)；缺输入或执行受阻则blocked。
- handoff后立即结束，不再修改文件。控制器只机械检查和保存候选，接受成果由Leader决定。
- 不把静态检查或模型自报当作真实执行结果，不以修改断言/绕过门禁使结果通过。
"""

    def board_for_leader(self):
        s = self.store.snapshot()
        return {
            "requests": s["requests"],
            "tasks": s["tasks"],
            "messages": s["messages"],
            "workers": [
                {"id": a["id"], "status": a["status"], "active_turn": a["active_turn"]}
                for a in s["actors"]
            ],
            "skills": list(SKILLS),
            "integration_head": git(self.repo, "rev-parse", "HEAD"),
        }

    def call(self, actor, turn_id, name, args):
        turn = self.store.authorize(actor, turn_id)
        with self.store.connect() as db:
            self.store.event(
                db,
                actor,
                "team_tool",
                {"name": name, "arguments": args},
                args.get("request_id"),
                args.get("task_id"),
            )
        if actor == "leader":
            if name == "get_board":
                return self.board_for_leader()
            if name == "create_task":
                return self.store.create_task(args)
            if name == "accept_task":
                return self.accept(args["task_id"])
            if name == "cancel_task":
                text(args["reason"], "取消原因")
                with self.store.connect() as db:
                    db.execute("BEGIN IMMEDIATE")
                    task = unpack(
                        db.execute(
                            "SELECT * FROM tasks WHERE id=?", (args["task_id"],)
                        ).fetchone()
                    )
                    if task["status"] in ["running", "accepted"]:
                        raise ValueError("执行中或已接受的任务不能取消")
                    db.execute(
                        "UPDATE tasks SET status='cancelled',result=?,finished=? WHERE id=?",
                        (args["reason"], time.time(), task["id"]),
                    )
                    self.store.event(
                        db,
                        actor,
                        "task_cancelled",
                        args,
                        task["request_id"],
                        task["id"],
                    )
                return {"cancelled": True}
            if name == "finish_request":
                with self.store.connect() as db:
                    db.execute("BEGIN IMMEDIATE")
                    unpack(
                        db.execute(
                            "SELECT * FROM requests WHERE id=?", (args["request_id"],)
                        ).fetchone()
                    )
                    if (
                        args["status"] == "completed"
                        and db.execute(
                            "SELECT 1 FROM tasks WHERE request_id=? AND status NOT IN ('accepted','cancelled')",
                            (args["request_id"],),
                        ).fetchone()
                    ):
                        raise ValueError("尚有未接受或未处理的任务，不能声称需求完成")
                    db.execute(
                        "UPDATE requests SET status=?,result=?,updated=? WHERE id=?",
                        (
                            args["status"],
                            text(args["message"], "结果说明"),
                            time.time(),
                            args["request_id"],
                        ),
                    )
                    self.store.event(
                        db, actor, "request_" + args["status"], args, args["request_id"]
                    )
                return {"saved": True}
        else:
            task = self.store.task(turn["task_id"])
            if name == "get_task":
                state = self.store.snapshot()
                return {
                    "task": task,
                    "request": next(
                        x for x in state["requests"] if x["id"] == task["request_id"]
                    ),
                    "dependencies": [self.store.task(x) for x in task["depends_on"]],
                    "skill": skill_body(task["skill"]),
                    "messages": [
                        x
                        for x in state["messages"]
                        if x["request_id"] == task["request_id"]
                        and x["recipient"] in [actor, "user"]
                    ],
                }
            if name == "handoff":
                return self.store.handoff(task["id"], args)
            if args.get("request_id") != task["request_id"]:
                raise ValueError("Worker只能在当前需求内通信")
        if name == "send_message":
            return self.store.message(
                actor, args["to"], args["message"], args["request_id"]
            )
        raise ValueError("工具不属于本执行身份")

    def accept(self, task_id):
        task = self.store.task(task_id)
        if task["status"] == "accepted":
            return task
        if task["status"] != "review" or not task["candidate"]:
            raise ValueError("任务尚无可接受的真实候选")
        with self.git_lock():
            self.workspace.check_target()
            try:
                git(self.repo, "merge-tree", "--write-tree", "HEAD", task["candidate"])
            except subprocess.CalledProcessError as error:
                raise ValueError(
                    "候选与当前集成版本有冲突；集成目录未修改。创建解决任务并提供候选SHA "
                    + task["candidate"]
                ) from error
            head = self.workspace.integrate(task["candidate"])
            with self.store.connect() as db:
                db.execute(
                    "UPDATE tasks SET status='accepted',integration_commit=? WHERE id=?",
                    (head, task_id),
                )
                self.store.event(
                    db,
                    "leader",
                    "task_accepted",
                    {"integration_commit": head},
                    task["request_id"],
                    task_id,
                )
        return self.store.task(task_id)
