"""Task-driven experiment execution; no fixed software-development role graph."""

import concurrent.futures
import fcntl
import json
import signal
import threading
import time
from pathlib import Path

from factory_store import Store
from git_workspace import Workspace
from native import Native


class Factory:
    def __init__(self, root):
        self.store = Store(root)
        self.settings = self.store.settings()
        self.repo = Path(self.settings["repository"])
        self.workspace = Workspace(
            self.repo, self.store.root, self.settings["integration_branch"]
        )
        self.stop = threading.Event()

    def plan(self, request, task_count):
        if not 1 <= task_count <= 100:
            raise ValueError("task count must be between 1 and 100")
        self.workspace.check_target()
        name = f"leader-{time.time_ns()}"
        snapshot = self.store.snapshot()
        prompt = f"""# 软件工厂 Leader 实验

你是本实验的协调 Agent。阅读当前仓库 AGENTS.md、实际需求/架构和相关代码后，拆解下面的用户目标。你可以使用原生 Skills 与工具研究项目，但本回合只制定任务，不修改代码、不提交、不创建外部 Issue、不启动旧流水线。

用户已经授权独立 agent-team 实验分支。本轮不要照搬 PM/开发/QA 固定岗位；按实际任务选择工作方法。项目原先的工作流入口约定不要求本实验启动旧平台。

## 用户目标
{request}

## 已有任务和结果
{json.dumps(snapshot["tasks"], ensure_ascii=False)}

## 已有协作消息
{json.dumps(snapshot["messages"], ensure_ascii=False)}

## 拆解约束
- 建议本轮不超过 {task_count} 个可验收的实际任务；不为凑并发制造空任务、无意义文件或重复审查。
- 尽量安排可独立执行的任务，明确允许修改的具体路径或路径前缀。write_paths=[] 表示不得修改任何文件。
- 任务 id 用英文小写 slug，不复用已有任务 id。依赖必须显式列出且无环。
- 依赖任务只在前置成果已合入实验分支后执行；需要同版检查的任务应明确候选与可复验的入口。
- instructions 使用 Markdown，写清对象、目标、材料、执行方法/适用 Skills、验证和交付要求。
- 所有产品实现、项目测试和实操复核交给相应任务的 Worker；本回合不代做。
- 每个 Worker 使用独立 worktree；提交、推送和合并由控制器处理，不能由 Worker 执行。
- 不部署、不合 main/master、不改真实凭据或供应商配置。发现权限/环境阻塞应报告，不绕行。
- 输出严格遵循已提供的计划 JSON Schema；summary 简明说明本轮的并行方式与依赖。
"""
        result = Native(self.store, self.stop).run(
            name,
            self.repo,
            prompt,
            schema=Path(__file__).parent / "schemas" / "plan.json",
        )
        if result["returncode"] or result["reason"]:
            raise RuntimeError(
                f"Leader did not complete; original logs preserved: {name}"
            )
        reply = Path(result["native_directory"]) / "reply.md"
        plan = json.loads(reply.read_text())
        self.store.add_tasks(plan["tasks"])
        return {
            "leader": result,
            "summary": plan["summary"],
            "task_ids": [task["id"] for task in plan["tasks"]],
        }

    def execute(self, task):
        try:
            thread_id = None
            if task["execution"].get("resume"):
                from git_workspace import git

                previous = task["execution"]
                work = {
                    "path": Path(previous["worktree"]),
                    "branch": previous["branch"],
                    "base": previous["base_commit"],
                }
                if (
                    git(work["path"], "branch", "--show-current") != work["branch"]
                    or git(work["path"], "rev-parse", "HEAD") != work["base"]
                ):
                    raise ValueError(
                        "preserved worktree has a different branch or commit; inspect rather than replay"
                    )
                thread_id = previous["thread_id"]
            else:
                work = self.workspace.prepare(task["id"], task["attempt"])
            self.store.execution(
                task["id"],
                worktree=str(work["path"]),
                branch=work["branch"],
                base_commit=work["base"],
            )
            prompt = f"""# Agent Team 实验任务

你负责一个具体任务，不代表永久岗位。本任务来自用户已授权的软件工厂实验；使用当前仓库的项目规则和可用 Skills，不启动旧平台流程。

## 任务
{task["title"]}

{task["instructions"]}

## 工作边界
- 当前 worktree 专属于本任务。允许修改的路径：{json.dumps(task["write_paths"], ensure_ascii=False)}。空列表表示仅研究/复核，不修改任何文件。
- 只完成本任务；不要改其他 worktree、共享任务库、凭据或执行配置。
- 不执行 git commit/push/merge/checkout/switch，不创建外部 Issue/PR，不部署。控制器负责版本操作。
- 使用 agent_team_factory.get_task 查看任务、依赖成果和消息。
- 缺输入、执行失败或权限不足时，使用 send_message(to="leader", message="具体问题、证据、需要的输入")；不能绕过审批或扩权。
- 完成后使用 agent_team_factory.handoff，status=completed，message 写明改动、实际验证、证据和局限；artifacts 为已存在的 worktree 相对路径。
- 无法完成则 handoff(status=blocked, ...)，保留现场。任务未实测、仅静态检查或只生成文件时必须如实说明。
- handoff 成功后结束本轮，不再修改文件。正常退出不是自动验收；控制器还会检查范围并提交候选。
"""
            result = Native(self.store, self.stop).run(
                f"{task['id']}-{task['attempt']}",
                work["path"],
                prompt,
                task=task,
                thread_id=thread_id,
            )
            if result["reason"] == "interrupted":
                self.store.finish(
                    task["id"],
                    "interrupted",
                    {"message": "native execution interrupted; workspace preserved"},
                )
                return
            if result["returncode"] or result["reason"]:
                raise RuntimeError(
                    f"native execution did not complete: exit={result['returncode']}, reason={result['reason']}"
                )
            submitted = self.store.get(task["id"])["submission"]
            if not submitted:
                raise RuntimeError("native process exited without a task handoff")
            for relative in submitted["artifacts"]:
                artifact = (work["path"] / relative).resolve()
                if (
                    not artifact.is_relative_to(work["path"].resolve())
                    or not artifact.is_file()
                ):
                    raise ValueError("handoff artifact is missing or outside worktree")
            if submitted["status"] == "blocked":
                self.store.finish(task["id"], "blocked", submitted)
                return
            commit = self.workspace.commit(task, work)
            self.store.finish(
                task["id"],
                "ready",
                {
                    **submitted,
                    "commit": commit,
                    "base_commit": work["base"],
                    "branch": work["branch"],
                    "has_changes": commit != work["base"],
                },
            )
        except Exception as error:
            detail = {"message": str(error), "type": type(error).__name__}
            self.store.finish(task["id"], "failed", detail)

    def run(self):
        with (self.store.root / "controller.lock").open("a") as lock:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError as error:
                raise ValueError("a controller is already running") from error
            if self.store.snapshot()["running"]:
                raise ValueError(
                    "unfinished execution records exist; inspect original processes and evidence before recovery"
                )
            if any(
                task["execution"].get("termination_error")
                for task in self.store.snapshot()["tasks"]
            ):
                raise ValueError(
                    "a native process group has unresolved termination evidence; inspect before scheduling more work"
                )
            self.workspace.check_target()
            old_handlers = {}
            if threading.current_thread() is threading.main_thread():
                for sig in (signal.SIGINT, signal.SIGTERM):
                    old_handlers[sig] = signal.signal(sig, lambda *_: self.stop.set())
            try:
                with concurrent.futures.ThreadPoolExecutor(
                    max_workers=self.settings["concurrency"]
                ) as pool:
                    pending = set()
                    while not self.stop.is_set():
                        while len(pending) < self.settings["concurrency"]:
                            self.workspace.check_target()
                            task = self.store.claim()
                            if not task:
                                break
                            pending.add(pool.submit(self.execute, task))
                        if not pending:
                            break
                        finished, pending = concurrent.futures.wait(
                            pending,
                            timeout=0.5,
                            return_when=concurrent.futures.FIRST_COMPLETED,
                        )
                        for future in finished:
                            future.result()
            finally:
                for sig, handler in old_handlers.items():
                    signal.signal(sig, handler)
        return self.store.snapshot()

    def integrate(self, task_id):
        task = self.store.get(task_id)
        if task["status"] != "ready" or not task["result"]:
            raise ValueError("only an explicitly selected ready task can be integrated")
        try:
            commit = self.workspace.integrate(task["result"]["commit"])
        except Exception:
            from git_workspace import git

            if git(self.repo, "diff", "--name-only", "--diff-filter=U"):
                self.store.finish(task_id, "conflict", task["result"])
            raise
        self.store.finish(
            task_id, "merged", {**task["result"], "integration_commit": commit}
        )
        return self.store.get(task_id)
