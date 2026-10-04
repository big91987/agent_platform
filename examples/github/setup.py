import argparse
import getpass
import http.cookiejar
import json
import os
import re
import urllib.request
from pathlib import Path

parser = argparse.ArgumentParser(
    description="Register this external pipeline's MCP tools and stage Agents"
)
parser.add_argument("--config", required=True, type=Path)
parser.add_argument(
    "--network-defaults-only",
    action="store_true",
    help="Allow networking and permission requests for registered pipeline Agents without replacing other settings",
)
parser.add_argument(
    "--tools-only",
    action="store_true",
    help="Refresh tools without replacing Agent instructions, Skills or native settings",
)
parser.add_argument(
    "--browser-source",
    type=Path,
    help="Owner-managed pinned Harness checkout for the browser runtime; defaults to the bundled examples/github/tooling snapshot",
)
parser.add_argument(
    "--observer-ref",
    help="Owner-selected workflow branch/tag for result observer rollout; defaults to main",
)
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
configpath = args.config.resolve()
c = json.loads(configpath.read_text())
settings = c["pipeline"]
namespace = settings.get("namespace", "")
if namespace and not re.fullmatch(r"[a-zA-Z0-9_-]+", namespace):
    raise ValueError(
        "namespace must contain only letters, digits, underscores or hyphens"
    )
prefix = namespace + "-" if namespace else ""
name_prefix = "[" + namespace + "] " if namespace else ""
if args.observer_ref:
    settings["observer_ref"] = args.observer_ref
if args.browser_source:
    source = args.browser_source.resolve()
    if not (source / "full_harness/browser/check.cjs").is_file():
        raise ValueError(
            "browser-source must contain the trusted Harness browser runtime"
        )
    settings["browser_source"] = str(source)
settings["tool_binary"] = str(root / "bin/pipeline-tool")
settings.setdefault("agents", {})
settings.setdefault("network_access", True)
password = os.environ.get("PLATFORM_ADMIN_PASSWORD") or getpass.getpass(
    "Platform admin password: "
)
jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))


def api(path, value=None, method=None):
    req = urllib.request.Request(
        c["base_url"] + path,
        data=None if value is None else json.dumps(value).encode(),
        headers={"Content-Type": "application/json", "Origin": c["base_url"]},
        method=method,
    )
    with opener.open(req, timeout=60) as r:
        return json.load(r)


api("/api/login", {"username": "admin", "password": password})
if args.network_defaults_only:
    for agent in api("/api/agents"):
        if agent["id"] in settings["agents"].values():
            agent.pop("resolved_tools", None)
            agent["network_access"] = True
            agent["allow_elevation"] = True
            api("/api/agents/" + agent["id"], agent, "PUT")
    configpath.write_text(json.dumps(c, indent=2))
    configpath.chmod(0o600)
    print(
        "Pipeline Agents allow networking and per-operation permission requests; existing conversations unchanged"
    )
    raise SystemExit(0)
registry = Path(settings["registry"])
Path(settings["workspaces"]).mkdir(parents=True, exist_ok=True)
registry.mkdir(parents=True, exist_ok=True)
for stage in ["requirements", "design", "development", "qa"]:
    sid = prefix + "pipeline-" + stage
    server = api(
        "/api/tool-servers",
        {
            "id": sid,
            "name": {
                "requirements": "需求交接",
                "design": "设计交接",
                "development": "研发交付",
                "qa": "QA 验收交接",
                "review": "代码审查交接",
            }[stage],
            "enabled": True,
            "connection": {
                "command": str(root / "examples/pipeline-tool/run.sh"),
                "args": ["--registry", str(registry), "--stage", stage],
            },
        },
    )
    api("/api/tool-servers/" + sid + "/discover", {})
for stage in ("design", "development", "qa", "review"):
    bid = prefix + "browser-" + stage
    api(
        "/api/tool-servers",
        {
            "id": bid,
            "name": stage + " 真实浏览器",
            "enabled": True,
            "connection": {
                "command": settings["python"],
                "args": [
                    str(root / "examples/github/browser_tool.py"),
                    "--config",
                    str(configpath),
                    "--stage",
                    stage,
                ],
            },
        },
    )
# Discovery requires only shared config, never a task or existing conversation.
c["pipeline"] = settings
configpath.write_text(json.dumps(c, indent=2))
configpath.chmod(0o600)
for stage in ["design", "development", "qa", "review"]:
    api("/api/tool-servers/" + prefix + "browser-" + stage + "/discover", {})
agents = api("/api/agents")
if args.tools_only:
    for stage in ("requirements", "design", "development", "qa", "review"):
        agent = next(a for a in agents if a["id"] == settings["agents"][stage])
        agent.pop("resolved_tools", None)
        agent.setdefault("tool_servers", [])
        if stage != "review":
            registration = next(
                r
                for r in agent["tool_servers"]
                if r["server_id"] == prefix + "pipeline-" + stage
            )
            for name, mode in (
                ("submit_handoff", "auto"),
                ("supplement_handoff", "auto"),
                ("replace_handoff", "confirm"),
            ):
                if name not in registration["tools"]:
                    registration["tools"].append(name)
                registration.setdefault("approvals", {}).setdefault(name, mode)
        if stage in ("design", "development", "qa", "review"):
            browser_id = prefix + "browser-" + stage
            browser = next(
                (r for r in agent["tool_servers"] if r["server_id"] == browser_id), None
            )
            if browser is None:
                browser = {"server_id": browser_id, "tools": [], "approvals": {}}
                agent["tool_servers"].append(browser)
            for name in ("check", "verify"):
                if name not in browser["tools"]:
                    browser["tools"].append(name)
                browser.setdefault("approvals", {}).setdefault(name, "auto")
        api("/api/agents/" + agent["id"], agent, "PUT")
        print(stage, "tools refreshed")
    raise SystemExit(0)
base = next(
    a
    for a in agents
    if a["id"] == settings["agents"].get("requirements", c["agent_id"])
)
backup = registry / "requirements-agent-before-handoff.json"
backup.write_text(json.dumps(base))
backup.chmod(0o600)
source = Path(settings["skills_root"])
stages = {
    "requirements": ["resumable-batch-grilling", "defining-platform-products-cn"],
    "design": ["platform-architecture-v2-cn", "reviewing-design-and-plans-cn"],
    "qa": [],
    "review": [],
    "development": [
        "managing-engineering-delivery-cn",
        "trellis-before-dev",
        "trellis-check",
        "trellis-update-spec",
    ],
}
for stage, names in stages.items():
    a = dict(base)
    a["network_access"] = True
    a["allow_elevation"] = True
    a.pop("resolved_tools", None)
    a["name"] = (
        name_prefix
        + {
            "requirements": "需求澄清助手",
            "design": "设计助手",
            "development": "研发助手",
            "qa": "QA 验证助手",
            "review": "代码审查助手",
        }[stage]
    )
    a["skills"] = [str(source / name) for name in names]
    a["tool_servers"] = [
        {
            "server_id": prefix + "pipeline-" + stage,
            "tools": ["submit_handoff", "supplement_handoff", "replace_handoff"],
            "approvals": {
                "submit_handoff": "auto",
                "supplement_handoff": "auto",
                "replace_handoff": "confirm",
            },
        }
    ]
    if stage == "review":
        a["tool_servers"] = []
    if stage in ("design", "development", "qa", "review"):
        a["tool_servers"].append(
            {
                "server_id": prefix + "browser-" + stage,
                "tools": ["check", "verify"],
                "approvals": {"check": "auto", "verify": "auto"},
            }
        )
    a["instructions"] = (
        f"你是 {stage} 阶段的 Agent。遵循当前项目 AGENTS.md 和本阶段 Skills，独立完成本阶段工作。与人对话使用自然语言，原样呈现真实进展和结果，明确是否需要用户回答。不要输出框架 JSON。读取已确认的任务文档，不重复询问已经确认的决定。使用已注册的工具及既有执行权限内的任务级测试脚本，依据真实调用结果说明进展。工具动作不足时可用 check.page_script 执行任务页面断言，不修改公共 Harness 或绕过质量检查。不要自行调用 GitHub，不操作 Git 分支/提交/推送。"
    )
    if stage == "requirements":
        a["instructions"] += (
            "产出 PRD 和可验证验收标准，完成自查并展示成果；按 Runner 的阶段确认策略通过工具交给设计阶段。"
        )
    elif stage == "design":
        a["instructions"] += (
            "交付原型、HLD、契约及索引，用 check 工具验证原型；展示产物和验证证据，按 Runner 的阶段确认策略通过工具交给研发。"
        )
    elif stage == "development":
        a["instructions"] += (
            "实现并完成必需自测后展示成果和验证证据；按 Runner 的阶段确认策略调用工具交给独立 QA，不直接创建 PR。"
        )
    elif stage == "qa":
        a["instructions"] += (
            "独立做真实浏览器验证（含适用的登录与权限路径），保存报告、截图和日志。按已接受的 PRD、设计及验收条款判断放行；未承诺的专项测试如实列为未测，不新增阻塞门槛。不修改产品实现。发现产品问题时自主判断退回 requirements、design 或 development，报告证据后调用 submit_handoff，target_stage 指定退回阶段；返工无需额外确认。环境故障先恢复或报告阻塞，不误判成产品问题。所有必需验收通过后，按 Runner 的阶段确认策略以 target_stage=report 交接，由交付流程处理 PR。"
        )
    elif stage == "review":
        a["instructions"] += (
            "独立审查代码差异及新增文件，报告带文件行号的具体问题和证据。只写审查报告，不修改产品实现，不调用交接工具，不触发后续流程或合并；由人判断如何处理审查意见。"
        )
    if stage != "review":
        a["instructions"] += (
            "阶段确认以 Runner 在当前任务中提供的阶段确认策略为准，覆盖本阶段 Skills 中默认的形式审批要求；缺省为严格模式，先取得明确用户确认再交接。"
            "自主模式下，普通澄清采用有证据的推荐方案和可逆默认，记录依据，完成必需检查后自动交接及普通返工，不再追加形式审批。"
            "已 ready PR 的集成授权仅覆盖同步主线、冲突修复和 QA 重跑，不扩大已接受范围；不能由既有决定确定的上游产品取舍不得猜测。"
            "目标无法判断、互斥目标无法取舍，或 P0 级重大不可逆数据损失、安全/隐私风险、重大外部财务承诺时等待用户。"
            "不得跳过测试、将未测或失败写成通过，或伪造用户批准。最终 PR 合并和高风险部署始终保留人工授权。"
            "用户自然语言明确要求回退到更早阶段时，直接调用 submit_handoff 并指定 target_stage；不重复审批。"
            "目标不明确才追问，同阶段能解决的留在本阶段。Agent 建议的普通上游返工按 Runner 的阶段确认策略执行；严格模式先请用户确认，QA 已证实缺陷除外。"
            "已经交接后，用户补充用 supplement_handoff，用户撤回或改目标用 replace_handoff；传返回的 run_id。"
            "交接后不要修改共享文件，先通过工具处理变更，不自行去下游会话发消息。"
            "交接 summary 如实记录回退依据、已有用户请求/确认或自主决策依据、修改要求和受影响结论，artifacts 附对应文档。"
        )
    a["native_config"] = ""
    a["trust_hooks"] = False
    if stage == "development":
        import shlex

        command = shlex.join(
            [
                settings["python"],
                str(root / "examples/github/verify.py"),
                "--config",
                str(configpath),
                "--hook",
            ]
        )
        a["native_config"] = (
            '[[hooks.Stop]]\n[[hooks.Stop.hooks]]\ntype="command"\ncommand='
            + json.dumps(command)
            + "\ntimeout=600\n"
        )
        a["trust_hooks"] = True
    if stage == "requirements":
        saved = api("/api/agents/" + a["id"], a, "PUT")
    else:
        found = next((v for v in agents if v["name"] == a["name"]), None)
        if found:
            a["id"] = found["id"]
            saved = api("/api/agents/" + a["id"], a, "PUT")
        else:
            a.pop("id", None)
            saved = api("/api/agents", a)
    settings["agents"][stage] = saved["id"]
    print(stage, saved["id"])
c["pipeline"] = settings
configpath.write_text(json.dumps(c, indent=2))
configpath.chmod(0o600)
