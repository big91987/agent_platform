#!/usr/bin/env python3
"""External GitHub pipeline. The platform knows only Agents and conversations."""

import argparse
import fcntl
import hashlib
import json
import os
import re
import shlex
import subprocess
import sys
from pathlib import Path
from urllib.parse import quote

import pr_refresh
from agent_platform_client import Client
from approval_policy import approval_policy, read_autonomous
from network_policy import issue_network
from requirements import github
from tooling import tooling_source

STAGES = ("requirements", "design", "development", "qa")
FORWARD = {
    "requirements": "design",
    "design": "development",
    "development": "qa",
    "qa": "report",
}


def run(argv, cwd=None):
    return subprocess.run(
        argv, cwd=cwd, text=True, capture_output=True, check=True, timeout=180
    ).stdout.rstrip("\n")


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    temporary = path.with_suffix(".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2))
    temporary.chmod(0o600)
    temporary.replace(path)


def load_handoff(path, expected, workspace, verify):
    workspace = workspace.resolve()
    if not path.is_file():
        raise ValueError("Missing handoff receipt; complete the preceding stage first")
    value = json.loads(path.read_text())
    if not expected or value.get("sha256") != expected:
        raise ValueError("Handoff does not match this dispatch")
    verify(path, expected)
    for name, content in value["documents"].items():
        document = (workspace / name).resolve()
        if not document.is_relative_to(workspace) or document.read_text() != content:
            raise ValueError(
                "Handed-off document changed or escaped workspace: " + name
            )
    return value


def task_directory(settings, workspace):
    return (
        Path(settings["registry"])
        / hashlib.sha256(str(workspace.resolve()).encode()).hexdigest()
    )


def issue_for_review(settings, repo, pr_number):
    """Resolve a delivered PR through trusted task records, never its title."""
    pr = github(f"repos/{repo}/pulls/{pr_number}")
    if pr["state"] != "open" or pr["draft"] or pr["head"]["repo"]["full_name"] != repo:
        raise ValueError("Review requires a ready, same-repository PR")
    for path in (Path(settings["registry"]) / "issues").glob("*.json"):
        task = json.loads(path.read_text())
        workspace = Path(task["workspace"])
        receipt = task_directory(settings, workspace) / "delivery.json"
        if (
            not receipt.exists()
            or json.loads(receipt.read_text())["pr_url"] != pr["html_url"]
        ):
            continue
        if (
            pr["head"]["ref"] != task["branch"]
            or run(["git", "branch", "--show-current"], workspace) != task["branch"]
            or run(["git", "rev-parse", "HEAD"], workspace) != pr["head"]["sha"]
        ):
            raise ValueError("Registered checkout does not match the PR head")
        if run(
            [
                "git",
                "status",
                "--porcelain",
                "--untracked-files=all",
                "--",
                "app",
                "tests",
                ".trellis/spec",
                "README.md",
                "deploy",
            ],
            workspace,
        ):
            raise ValueError("Registered checkout has uncommitted product changes")
        return int(path.stem)
    raise ValueError("PR has no registered pipeline delivery")


def result_path(directory, task, stage):
    suffix = task.get("round", task.get("verification_run", ""))
    return directory / (stage + ("-" + suffix if suffix else "") + "-result.json")


def accept_handoff(task, directory, stage, source, file, digest, workspace, verify):
    """Consume one immutable handoff; retries cannot issue a second stage input."""
    transition = {"file": file, "sha256": digest, "stage": stage}
    saved = json.loads(Path(file).read_text())
    if saved.get("revoked"):
        raise ValueError("This handoff was withdrawn")
    if task.get("replacement") and source != "redirect":
        raise ValueError("A replacement is in progress; this callback is stale")
    if task.get("transition") == transition and task.get("active_stage") == stage:
        return json.loads(Path(file).read_text())
    if source == "redirect":
        if task.get("replacement") != transition:
            raise ValueError("Unregistered replacement callback")
        handoff = load_handoff(Path(file), digest, workspace, verify)
        if stage not in STAGES or handoff["handoff"].get("target_stage") != stage:
            raise ValueError("Replacement target mismatch")
        task["active_stage"] = stage
        task["transition"] = transition
        task.pop("replacement")
        return handoff
    if source not in FORWARD or task.get("active_stage", source) != source:
        raise ValueError("Stale callback: this is not the active stage")
    if file != str(result_path(directory, task, source)):
        raise ValueError("Stale callback: use the current round's handoff")
    handoff = load_handoff(Path(file), digest, workspace, verify)
    target = handoff["handoff"].get("target_stage")
    if source == "qa" and not target:
        raise ValueError("QA must explicitly set target_stage; no default delivery")
    target = target or FORWARD[source]
    if target != stage or (
        target != FORWARD[source] and target not in STAGES[: STAGES.index(source)]
    ):
        raise ValueError("Unregistered handoff route")
    if target in STAGES[: STAGES.index(source)]:
        task["round"] = str(
            int(task.get("round", task.get("verification_run", "0")) or "0") + 1
        )
    task["active_stage"] = stage
    task["transition"] = transition
    return handoff


def recover_run_handle(file, handoff, repository, run_id):
    # Called under the issue lock after validating/accepting this transition.
    # The dispatch response may have been lost even though this Run was started.
    if not handoff.get("run_id") and run_id.isdigit():
        handoff.update(
            run_id=int(run_id),
            run_url=f"https://github.com/{repository}/actions/runs/{run_id}",
            delivery="accepted",
        )
        save(Path(file), handoff)


def start_reverification(task, client, run_id):
    if not run_id.isdigit():
        raise ValueError("Reverification requires a GitHub workflow run ID")
    if "development" not in task["stages"]:
        raise ValueError("Complete development before requesting QA")
    if task.get("verification_run") == run_id:
        if task.get("active_stage") != "qa" or task.get("round") != run_id:
            raise ValueError("Stale verification retry: the task has already advanced")
        return
    if task.get("verification_run") and int(run_id) < int(task["verification_run"]):
        raise ValueError("Stale verification retry: a newer verification has started")
    for stage in ("development", "qa", "review"):
        previous = task["stages"].get(stage)
        if (
            previous
            and client.conversation(previous["conversation_id"])["conversation"][
                "status"
            ]
            != "idle"
        ):
            raise ValueError(
                f"Finish or recover the {stage} conversation before starting fresh verification"
            )
    task["verification_run"] = run_id
    task["round"] = run_id
    task["active_stage"] = "qa"
    task.pop("transition", None)


def prepare_registration(settings, task, stage, number):
    workspace = Path(task["workspace"])
    directory = task_directory(settings, workspace)
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    sys.path.insert(0, str(tooling_source(settings)))
    from full_harness.common import controls

    save(
        directory / (stage + ".json"),
        {
            "workspace": str(workspace),
            "task_file": str(Path(settings["registry"]) / "issues" / f"{number}.json"),
            "control_command": [
                settings["python"],
                str(Path(__file__).with_name("handoff_control.py")),
                "--config",
                settings["config_path"],
                "--issue",
                str(number),
            ],
            "controls": controls(workspace),
            "result_file": str(result_path(directory, task, stage)),
            "dispatch_url": f"https://api.github.com/repos/{settings['repository']}/actions/workflows/agent-platform.yml/dispatches",
            "token_env": "PIPELINE_TOKEN",
            "ref": "main",
            "inputs": {"issue": str(number), "after": stage},
        },
    )
    return directory


def stage_prompt(
    stage, number, issue, handoff=None, *, autonomous=False, integrating=False
):
    context = (
        f"这是 Harness CI/CD 托管工作区，任务是 Issue #{number}。Runner 已创建任务分支；"
        "你在当前工作区工作，不创建/切换分支，不提交、推送或创建 PR。\n"
        f"你负责 {stage} 阶段。首次进入时了解适用指令、docs/README.md 和本任务文件，遵循本阶段 Skill；已注入或已读取的未变更内容无需重复读取。"
        "与用户交流使用自然语言，不输出 next_state/message/artifacts JSON。"
        "对已确认的上游决定不重复审批。\n"
    )
    if stage != "review":
        context += (
            "阶段交接通过已注册的 submit_handoff 工具完成，不是通过回复中的状态字段。"
            "用户可用自然语言要求退回任何更早阶段：目标明确就执行，不重复要求批准；目标不明才澄清。"
            "你发现上游问题时说明建议回退目标和影响，按本轮阶段确认策略判断是否需要人工确认。"
            "同阶段可解决的问题留在本阶段。回退调用 submit_handoff，target_stage 为更早阶段，"
            "summary 记录用户请求/确认或 QA 缺陷依据、原因、修改要求、受影响结论；artifacts 附交接文档。"
            "回退后重新推进受影响阶段与验证，不沿用旧放行结论。"
            "交接失败要明确报告，不能声称下一阶段已开始。"
            "交接后用户补充或纠正：不再编辑共享工作区，使用 supplement_handoff(run_id,content) 追加；"
            "用户要求撤回或改变目标时使用 replace_handoff(run_id,content,target_stage)，由工具停止旧执行并重派。"
            "使用交接工具返回的 run_id；不要自行调用 GitHub、修改回执或去下游会话发消息。"
            "交接成功后用一句自然语言说明工具确认的结果，并给出原始 Issue 链接；"
            "不要因旧工作流的静默交接约定而返回空回复，也不重复整份确认摘要。\n"
        )
    if stage == "requirements":
        context += "需求明确后产出 PRD、自查与决策记录；按本轮阶段确认策略交接设计。\n"
    elif stage == "design":
        context += "产出可运行原型、HLD、数据契约与设计索引，完成实际验证后按本轮阶段确认策略交接研发。\n"
    elif stage == "development":
        context += (
            "实现已确认设计，运行功能与格式/lint检查，修复发现的问题。真实浏览器使用已挂载的 check 工具。"
            "产品核心回归计划使用 tests/browser/core.json；已有旧 .harness/reading-core.json 时迁移其断言，否则依据本项目核心旅程建立，"
            "产品 UI 变化时维护定位信息，保留原有业务动作和断言，不修改受保护的 Harness 配置。"
            f"新增功能浏览器验收计划放在 docs/05-validation/tasks/{number}/browser-plan.json，"
            f"验证报告放在 docs/05-validation/tasks/{number}/validation.md。"
            "完成后展示可运行成果、变更范围、验证结果和未解决问题，按本轮阶段确认策略交给独立 QA。"
            "通过 submit_handoff 交接；这次交接只启动 QA，不创建 PR。不得直接合入主线。\n"
        )
    elif stage == "qa":
        context += (
            "你是独立 QA/Verifier。按已确认的 PRD 和设计检查开发交付，不把开发自测报告当作验证结果。"
            "放行门槛必须能追溯到已批准条款；未承诺的专项兼容性检查列为未测范围，不自行升级为阻塞，也不把未测写成通过。"
            "用已挂载的 check 工具操作真实浏览器，覆盖核心旅程、新功能、异常路径、桌面和手机。"
            "如果产品有登录功能，实际验证登录成功、错误凭据、退出及未登录访问边界；"
            "没有登录功能则注明不适用及理由，不虚构账号或通过证据。凭据不得写入报告或截图。"
            f"将逐项验收结果、复现步骤、截图和日志链接写入 docs/05-validation/tasks/{number}/qa.md。"
            "只写验证计划和证据文档，不修改产品代码和既有测试来使其通过。"
            "发现产品问题时，自主判断责任阶段：需求遗漏/冲突回 requirements，设计缺陷回 design，实现错误回 development。"
            "报告必须记录复现证据、影响及退回理由，然后调用 submit_handoff 并设置 target_stage；返工无需另请用户批准。"
            "环境/工具故障先恢复或报告阻塞，不能伪装成产品缺陷退回。"
            "全部必需验收通过后展示报告和截图，按本轮阶段确认策略交付 PR；"
            "满足交接条件后调用 submit_handoff，target_stage=report。所有 QA 交接 artifacts 必须包含 qa.md。\n"
        )
    elif stage == "review":
        context += (
            "你是独立 Code Reviewer。审查任务分支相对基线的全部产品与测试差异，包括未跟踪的新文件；"
            "检查正确性、回归、安全边界、错误处理和测试覆盖，交叉核对 PRD、设计与 QA 证据。"
            "不得只复述 QA 或开发报告，不修改产品实现。"
            f"将结论及带文件行号、影响与修复建议的问题写入 docs/05-validation/tasks/{number}/code-review.md。"
            "将审查发现与证据交给人判断，不自主推进、返工或合并，不调用交接工具。"
            "这是独立于产品 QA 返工链的审查流程；有无问题都如实给出结论。\n"
        )
    if stage in STAGES:
        context += approval_policy(
            stage, autonomous=autonomous, integrating=integrating
        )
        context += "本策略由 Runner 根据任务创建时保存的配置提供；覆盖 Skill/仓库中普通阶段确认的默认要求，保留方法、产物和验证要求。\n"
    context += "\n原始 Issue：" + issue.get("html_url", "")
    context += "\n原始需求：\n" + issue["title"] + "\n" + (issue.get("body") or "")
    if handoff:
        context += (
            "\n\n上游交接（不是要求重新确认）：\n" + handoff["handoff"]["summary"]
        )
        context += "\n交接文件（返工时以问题报告为准）：\n" + "\n".join(
            handoff["documents"]
        )
    if handoff:
        for addition in handoff.get("supplements", []):
            context += "\n交接后补充：\n" + addition["content"]
    return context


def comment_once(repo, number, marker, content):
    comments = github(
        f"repos/{repo}/issues/{number}/comments?per_page=100", paginate=True
    )
    if any(
        marker in c.get("body", "")
        and c["user"]["login"] in (repo.split("/")[0], "github-actions[bot]")
        for c in comments
    ):
        return
    github(f"repos/{repo}/issues/{number}/comments", {"body": marker + "\n" + content})


def delivery_token(settings):
    name = settings.get("delivery_token_file")
    if not name:
        raise ValueError(
            "Configure pipeline.delivery_token_file for unattended PR delivery"
        )
    path = Path(name)
    if not path.is_absolute() or not path.is_file() or path.stat().st_mode & 0o077:
        raise ValueError(
            "Delivery credential must be an absolute private file (mode 0600)"
        )
    value = path.read_text().strip()
    if not value:
        raise ValueError("Delivery credential file is empty")
    return value


def product_digest(workspace):
    """Bind independent verification to the product and tests actually inspected."""
    files = run(
        [
            "git",
            "ls-files",
            "-co",
            "--exclude-standard",
            "-z",
            "--",
            "app",
            "tests",
            ".trellis/spec",
            "README.md",
            "deploy",
        ],
        workspace,
    ).split("\0")
    digest = hashlib.sha256()
    for name in sorted(set(filter(None, files))):
        path = workspace / name
        if path.is_symlink():
            content = ("symlink:" + os.readlink(path)).encode()
        elif path.is_file():
            content = path.read_bytes()
        else:
            content = b"<deleted>"
        digest.update(name.encode() + b"\0" + hashlib.sha256(content).digest())
    return digest.hexdigest()


def require_verified_product(task, workspace):
    current = product_digest(workspace)
    for stage in ("qa",):
        if task["stages"].get(stage, {}).get("product_sha256") != current:
            raise ValueError(
                f"Product changed or {stage} verification is missing; obtain fresh verification before publishing"
            )


def pull_request_content(workspace, number, repository, branch):
    path = workspace / f"docs/05-validation/tasks/{number}/pull-request.md"
    if not path.resolve().is_relative_to(workspace.resolve()) or not path.is_file():
        raise ValueError("Missing PR description: ask QA to prepare " + str(path))
    text = path.read_text().strip()
    title, _, body = text.partition("\n")
    title = title.removeprefix("# ").strip()
    if (
        not text.startswith("# ")
        or not title
        or len(title) > 120
        or re.fullmatch(r"(?:实现|Implement)\s*Issue\s*#?\d+", title, re.IGNORECASE)
    ):
        raise ValueError(
            "PR description needs a concrete feature title, not an Issue number"
        )
    sections = dict(re.findall(r"^## ([^\n]+)\n(.*?)(?=^## |\Z)", body, re.M | re.S))
    for section in ("背景", "实现内容", "验证结果", "风险与限制", "界面效果"):
        if not sections.get(section, "").strip():
            raise ValueError("PR description missing section: " + section)
    base = f"https://github.com/{repository}/blob/{quote(branch, safe='')}/"
    references = []
    for label, name in (
        ("需求", f"docs/04-implementation/tasks/{number}/prd.md"),
        ("研发验证", f"docs/05-validation/tasks/{number}/validation.md"),
        ("独立 QA", f"docs/05-validation/tasks/{number}/qa.md"),
    ):
        if (workspace / name).is_file():
            references.append(f"- [{label}]({base}{name})")
    return {
        "title": title,
        "body": body.strip()
        + f"\n\nCloses #{number}\n"
        + ("\n### 验证与需求资料\n\n" + "\n".join(references) if references else "")
        + "\n\n草稿交付不代表集成 QA 或代码审查已经完成；以当前提交的检查结果为准，最终由人决定合并。\n",
    }


def publish(settings, task, number, directory):
    workspace = Path(task["workspace"])
    if run(["git", "branch", "--show-current"], workspace) != task["branch"]:
        raise ValueError("Workspace is no longer on its registered task branch")
    require_verified_product(task, workspace)
    if task.get("integration"):
        pr = pr_refresh.current_pr(settings, task)
        if pr["head"]["sha"] not in (
            task["integration"]["expected_head"],
            run(["git", "rev-parse", "HEAD"], workspace),
        ):
            raise ValueError(
                "PR changed during verification; refusing to publish stale work"
            )
    prs = github(
        f"repos/{settings['repository']}/pulls?head={settings['repository'].split('/')[0]}:{task['branch']}&state=all"
    )
    content = (
        None
        if prs
        else pull_request_content(
            workspace, number, settings["repository"], task["branch"]
        )
    )
    # The unattended Runner must not depend on the desktop login keychain.
    os.environ["GH_TOKEN"] = delivery_token(settings)
    # Re-run the same reusable development gate outside the model, before publishing.
    run(
        [
            settings["python"],
            str(Path(__file__).with_name("verify.py")),
            "--config",
            settings["config_path"],
            "--workspace",
            str(workspace),
            "--issue",
            str(number),
            "--evidence-name",
            "runner-checks",
        ]
    )
    names = run(
        ["git", "status", "--porcelain", "--untracked-files=all"], workspace
    ).splitlines()
    for entry in names:
        name = entry[3:]
        if not (
            name.startswith(("app/", "docs/", "tests/", ".trellis/spec/", "deploy/"))
            or name == "README.md"
        ):
            raise ValueError(
                "Unexpected delivery change outside product/doc scope: " + name
            )
    paths = [
        name
        for name in ("app", "docs", "tests", ".trellis/spec", "README.md", "deploy")
        if (workspace / name).exists()
    ]
    run(["git", "add", "--", *paths], workspace)
    if run(["git", "diff", "--cached", "--name-only"], workspace):
        run(
            ["git", "commit", "-m", f"Implement reading-list Issue #{number}"],
            workspace,
        )
    run(["git", "push", "-u", "origin", task["branch"]], workspace)
    if prs:
        pr = prs[0]
    else:
        pr = github(
            f"repos/{settings['repository']}/pulls",
            {
                **content,
                "head": task["branch"],
                "base": "main",
                "draft": True,
            },
        )
    save(
        directory / "delivery.json",
        {
            "pr_url": pr["html_url"],
            "head": run(["git", "rev-parse", "HEAD"], workspace),
        },
    )
    comment_once(
        settings["repository"],
        number,
        "<!-- platform-delivery -->",
        f"代码和复验已完成，已提交任务分支：[查看草稿 PR]({pr['html_url']})。",
    )
    print("Delivery PR: " + pr["html_url"], flush=True)


def skip_closed_issue(issue, number):
    if issue["state"] == "open":
        return False
    if issue["state"] != "closed" or "pull_request" in issue:
        raise ValueError("Expected an open product Issue")
    message = (
        f"Issue #{number} is closed. Skipped late pipeline work; "
        "no Agent invocation or publication. This is not a QA pass. "
        "Existing evidence and verification status are unchanged."
    )
    print("::notice::" + message, flush=True)
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with Path(summary).open("a") as file:
            file.write("## Pipeline skipped\n\n" + message + "\n")
    return True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument(
        "--stage", choices=(*STAGES, "review", "report", "integrate"), required=True
    )
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())
    settings = {**config["pipeline"], "config_path": str(Path(args.config).resolve())}
    repo = os.environ["GH_REPO"]
    if repo != settings["repository"]:
        raise ValueError("Unregistered repository")
    pr_number = os.environ.get("PR_NUMBER", "")
    if pr_number and args.stage != "review":
        raise ValueError("PR events can only start code review")
    number = (
        issue_for_review(settings, repo, int(pr_number))
        if pr_number
        else int(os.environ["ISSUE_NUMBER"])
    )
    if number <= 0:
        raise ValueError("Invalid Issue number")
    client = Client(config["base_url"], config["token"])
    issue = github(f"repos/{repo}/issues/{number}")
    if "pull_request" in issue:
        raise ValueError("Expected an open product Issue")
    path = Path(settings["registry"]) / "issues" / f"{number}.json"
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    with path.with_suffix(".lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        task = json.loads(path.read_text()) if path.exists() else None
        if task is None and issue["state"] != "open":
            raise ValueError("Expected an open product Issue")
        if skip_closed_issue(issue, number):
            return
        if task is None:
            if args.stage != "requirements":
                raise ValueError(
                    "Missing durable task registration; start requirements first"
                )
            workspace = (Path(settings["workspaces"]) / f"issue-{number}").resolve()
            branch = f"codex/issue-{number}-platform"
            checkout = Path(settings["checkout"])
            run(["git", "fetch", "origin", "main"], checkout)
            # The task branch/workspace is created exactly once by the Runner.
            run(
                ["git", "worktree", "add", "-b", branch, str(workspace), "origin/main"],
                checkout,
            )
            task = {
                "workspace": str(workspace),
                "branch": branch,
                "stages": {},
                "autonomous": read_autonomous(issue.get("body") or ""),
                "network_access": issue_network(
                    issue.get("body") or "", settings.get("network_access", True)
                ),
            }
            save(path, task)
        task.setdefault("network_access", settings.get("network_access", True))
        save(path, task)
        workspace = Path(task["workspace"])
        directory = task_directory(settings, workspace)
        handoff = None
        integrating_start = args.stage == "integrate"
        if integrating_start:
            if task.get("integration", {}).get("state") == "review":
                expected_round = "review-" + task["integration"]["published_head"]
                if task["stages"].get("review", {}).get("round") != expected_round:
                    os.environ["GH_TOKEN"] = delivery_token(settings)
                    pr_refresh.dispatch(settings, number, "code_review")
                return
            next_stage = pr_refresh.start(
                settings, task, path, client, os.environ.get("GITHUB_RUN_ID", "")
            )
            if not next_stage:
                print("Refresh is already current or in progress.", flush=True)
                return
            args.stage = next_stage
        reverify = os.environ.get("REVERIFY_RUN", "")
        if reverify:
            if args.stage != "qa":
                raise ValueError("Fresh verification must start with QA")
            start_reverification(task, client, reverify)
            save(path, task)
        source = os.environ.get("AFTER_STAGE", "")
        if (
            not source
            and args.stage not in ("requirements", "review")
            and not reverify
            and not integrating_start
        ):
            source = (
                "qa" if args.stage == "report" else STAGES[STAGES.index(args.stage) - 1]
            )
        if (
            source
            and source not in ("start", "verify", "code_review", "integrate")
            and not reverify
        ):
            incoming = os.environ.get("RESULT_FILE") or str(
                result_path(directory, task, source)
            )
            if not Path(incoming).is_file():
                raise ValueError(
                    "Missing handoff receipt; complete the preceding stage first"
                )
            incoming_value = json.loads(Path(incoming).read_text())
            if incoming_value.get("revoked"):
                raise ValueError("This handoff was withdrawn")
            actual_source = incoming_value.get("source_stage", source)
            prior = task["stages"].get(actual_source)
            if not prior:
                raise ValueError(f"Missing {source} handoff")
            # Never hold the task lock while a human/Agent is still working.
            fcntl.flock(lock, fcntl.LOCK_UN)
            result = client.wait(prior["conversation_id"], timeout=600)
            fcntl.flock(lock, fcntl.LOCK_EX)
            task = json.loads(path.read_text())
            # A human may merge/close the Issue while this Run waits for QA.
            if skip_closed_issue(github(f"repos/{repo}/issues/{number}"), number):
                return
            if result["conversation"]["status"] != "idle":
                raise ValueError(
                    "Previous Agent has not finished; inspect its conversation"
                )
            expected = os.environ["RESULT_SHA256"]
            incoming = os.environ.get("RESULT_FILE") or str(
                result_path(directory, task, source)
            )
            handoff = accept_handoff(
                task,
                directory,
                args.stage,
                source,
                incoming,
                expected,
                workspace,
                lambda file, digest: run(
                    [
                        settings["tool_binary"],
                        "--verify-result",
                        str(file),
                        "--expected-sha256",
                        digest,
                    ]
                ),
            )
            recover_run_handle(
                incoming, handoff, repo, os.environ.get("GITHUB_RUN_ID", "")
            )
            if actual_source != args.stage:
                client.workspace_access(prior["conversation_id"], read_only=True)
            if source == "qa":
                report_path = f"docs/05-validation/tasks/{number}/qa.md"
                if not handoff["documents"].get(report_path, "").strip():
                    raise ValueError("Missing independent QA report: " + report_path)
                if prior.get("product_sha256") != product_digest(workspace):
                    # Retry after the recipient has already edited the product is
                    # allowed; it only reconnects to its saved invocation.
                    current = task["stages"].get(args.stage, {})
                    if current.get("round", "") != task.get("round", "") or not current:
                        raise ValueError(
                            "Product changed after QA; obtain fresh verification"
                        )
            if (
                source == "development"
                and args.stage == "qa"
                and task.get("integration")
            ):
                pr_refresh.finish_conflicts(task, workspace, path)
            save(path, task)
            if source == "redirect" or args.stage in STAGES[: STAGES.index(source)]:
                comment_once(
                    repo,
                    number,
                    f"<!-- platform-rework:{task['round']}:{expected} -->",
                    f"[harness] **{actual_source} → {args.stage}**：{'已撤回旧交接并重新下发' if source == 'redirect' else '已按交接要求回退'}，继续使用本 Issue、任务分支和目标阶段会话。\n\n"
                    + handoff["handoff"]["summary"],
                )
        elif args.stage == "review":
            os.environ["GH_TOKEN"] = (
                delivery_token(settings)
                if task.get("integration")
                else os.environ.get("GH_TOKEN", "")
            )
            delivery_path = directory / "delivery.json"
            if not delivery_path.exists():
                raise ValueError(
                    "Create the draft PR before requesting independent code review"
                )
            # Independent review may be repeated after human-requested fixes.
            for name, previous in task["stages"].items():
                if (
                    client.conversation(previous["conversation_id"])["conversation"][
                        "status"
                    ]
                    != "idle"
                ):
                    raise ValueError(f"Finish {name} before reviewing the workspace")
        elif not reverify and not integrating_start:
            if task.get("active_stage", "requirements") != "requirements":
                raise ValueError("Task already advanced; do not restart requirements")
            task.setdefault("active_stage", "requirements")
            save(path, task)
        if args.stage == "report":
            publish(settings, task, number, directory)
            if task.get("integration"):
                pr_refresh.published(
                    settings,
                    task,
                    path,
                    number,
                    run(["git", "rev-parse", "HEAD"], workspace),
                )
            return
        if args.stage != "review":
            prepare_registration(settings, task, args.stage, number)
        receipt = task["stages"].get(args.stage)
        invocation_round = (
            (
                "review-" + task["integration"]["published_head"]
                if task.get("integration", {}).get("published_head")
                else os.environ.get("GITHUB_RUN_ID", "manual-review")
            )
            if args.stage == "review"
            else task.get("round", task.get("verification_run", ""))
        )
        fresh_verification = receipt and receipt.get("round", "") != invocation_round
        if not receipt or fresh_verification:
            prompt = (
                issue["title"]
                + "\n\n"
                + stage_prompt(
                    args.stage,
                    number,
                    issue,
                    handoff,
                    autonomous=task.get("autonomous", False),
                    integrating=bool(task.get("integration")),
                )
            )
            if task.get("integration"):
                prompt += (
                    "\n当前为 Ready PR 自动同步复验，主线基线："
                    + task["integration"]["base_sha"]
                    + "。保留既有需求和设计范围，检查主线变化影响并重新验证。\n"
                )
                if task["integration"].get("conflicts"):
                    prompt += (
                        "框架已执行 merge，以下文件冲突需按既有产品行为编辑解决（不执行Git写操作）：\n"
                        + "\n".join(task["integration"]["conflicts"])
                        + "\n框架在交给QA前暂存并完成合并提交。\n"
                    )
            if args.stage == "development":
                quality = shlex.join(
                    [
                        settings["python"],
                        str(Path(__file__).with_name("verify.py")),
                        "--config",
                        str(Path(args.config).resolve()),
                        "--issue",
                        str(number),
                    ]
                )
                prompt += (
                    "\n共享质量命令（在当前工作目录执行）：\n格式化与自动修复："
                    + quality
                    + " --fix\n最终复验："
                    + quality
                    + "\n"
                )
            if args.stage == "review":
                base = run(["git", "merge-base", "HEAD", "origin/main"], workspace)
                prompt += f"\n审查基线 commit：{base}。使用 git diff {base} -- app tests .trellis/spec README.md deploy，并检查 git ls-files --others --exclude-standard 的新增产品文件。\n"
            inspected_product = (
                product_digest(workspace) if args.stage in ("qa", "review") else None
            )
            invoke_options = (
                {"conversation_id": receipt["conversation_id"]}
                if fresh_verification
                else {
                    "agent_id": settings["agents"][args.stage],
                    "workspace_path": str(workspace),
                    "network_access": task["network_access"],
                }
            )
            if invocation_round:
                prompt += "\n这是本任务新一轮执行。读取当前交接与最新文档，处理反馈并重新验证，不沿用之前完成或交接成功的结论。需求方向变化才请用户决定，不重复批准未变化的决定。\n"
            if fresh_verification:
                client.network_access(
                    receipt["conversation_id"], task["network_access"]
                )
                client.workspace_access(receipt["conversation_id"], read_only=False)
            if (
                fresh_verification
                and client.conversation(receipt["conversation_id"])["conversation"][
                    "status"
                ]
                == "stopped"
            ):
                client.continue_queue(receipt["conversation_id"])
            receipt = client.invoke(
                prompt,
                **invoke_options,
                request_id=f"{repo}:issue:{issue['id']}:{args.stage}"
                + (":" + invocation_round if invocation_round else ""),
            )
            receipt["round"] = invocation_round
            receipt["run_id"] = os.environ.get("GITHUB_RUN_ID", "")
            if inspected_product:
                receipt["head_sha"] = run(["git", "rev-parse", "HEAD"], workspace)
                receipt["product_sha256"] = inspected_product
                receipt["verification_run"] = task.get("verification_run", "")
            task["stages"][args.stage] = receipt
            save(path, task)
        conversation = receipt["conversation_id"]
        url = config["base_url"].rstrip("/") + "/conversations/" + conversation
        comment_once(
            repo,
            number,
            f"<!-- platform-stage:{args.stage}:{receipt['message_id']} -->",
            f"[{args.stage}：打开 Agent 会话]({url})\n\n在平台里查看实时输出、回复和确认。会话 ID：`{conversation}`。",
        )
        if args.stage == "review":
            delivery = json.loads((directory / "delivery.json").read_text())
            review_pr = int(delivery["pr_url"].rstrip("/").split("/")[-1])
            comment_once(
                repo,
                review_pr,
                f"<!-- platform-review-start:{receipt['message_id']} -->",
                f"独立 Code Review Agent 已启动。[查看实时审查]({url})。\n\n审查结果将回贴本 PR，由人工决定修改与合并。",
            )
        print(f"{args.stage}: {url}", flush=True)
        fcntl.flock(lock, fcntl.LOCK_UN)
        try:
            result = client.wait(
                conversation, message_id=receipt["message_id"], timeout=900
            )
        except TimeoutError:
            from observe import dispatch_observer, matching

            fcntl.flock(lock, fcntl.LOCK_EX)
            current = json.loads(path.read_text())
            if matching(current, args.stage, receipt["message_id"]):
                dispatch_observer(settings, number, args.stage, receipt["message_id"])
                comment_once(
                    repo,
                    number,
                    f"<!-- platform-wait:{receipt['message_id']} -->",
                    f"{args.stage} 仍在执行，已安排继续接收同一轮结果。[查看实时进度]({url})",
                )
            return
        fcntl.flock(lock, fcntl.LOCK_EX)
        current = json.loads(path.read_text())
        if (
            current.get("replacement")
            or (args.stage != "review" and current.get("active_stage") != args.stage)
            or current.get("stages", {}).get(args.stage, {}).get("message_id")
            != receipt["message_id"]
            or (
                source
                and source not in ("start", "verify", "code_review", "integrate")
                and json.loads(Path(incoming).read_text()).get("revoked")
            )
        ):
            print(
                "Execution superseded; no obsolete reply will be published.", flush=True
            )
            return
        if result["conversation"]["status"] in ("failed", "stopped"):
            raise RuntimeError(
                "Agent did not finish; inspect the conversation, do not create a replacement"
            )
        replies = [
            m
            for m in result["messages"]
            if m["role"] == "agent"
            and m["kind"] == "reply"
            and m.get("parent_id") == receipt["message_id"]
        ]
        if args.stage == "review" and replies:
            delivery = json.loads((directory / "delivery.json").read_text())
            pr_number = int(delivery["pr_url"].rstrip("/").split("/")[-1])
            comment_once(
                repo,
                pr_number,
                f"<!-- platform-code-review:{receipt['message_id']} -->",
                replies[-1]["content"]
                + f"\n\n[独立审查会话]({url})\n\n由人工决定修改与合并。",
            )
        if args.stage == "review" and replies and current.get("integration"):
            require_verified_product(current, workspace)
            if receipt.get("product_sha256") != product_digest(workspace):
                raise ValueError("Product changed during independent review")
            pr_refresh.reviewed(settings, current, path, number, receipt["head_sha"])
        if replies:
            comment_once(
                repo,
                number,
                f"<!-- platform-reply:{receipt['message_id']} -->",
                replies[-1]["content"] + f"\n\n[在平台继续]({url})",
            )
        summary = os.environ.get("GITHUB_STEP_SUMMARY")
        if summary:
            with open(summary, "a") as output:
                output.write(f"[{args.stage} 会话]({url})\n")


if __name__ == "__main__":
    main()
