# Agent Team 软件工厂实验

本例验证一个独立的软件工厂：真实 Leader Agent 拆解需求，多个真实原生 Agent 在独立 Git worktree 中执行任务，共享持久化任务、消息和结果，成果只合入专用实验分支。它不启动或修改已有 Agent Platform 工作流，也不部署 Model Relay。

这是实验版，不代表已经验证 20 并发或生产可用。实际执行、并发峰值、失败和交付情况以实验报告为准。隔离测试中的进程探针与真实 Agent 执行分别记录。

## 边界

- 维护源仅在本目录；运行数据、原生日志、模型会话和实验 checkout 放在 Git 忽略的独立目录。
- 平台源码分支：`codex/agent-team-factory-experiment`。
- 测试仓汇合分支：`agent-team/experiment`。任务分支统一为 `agent-team/task-<task-id>-<attempt>`，worktree 目录含 `agent-team` 标记。
- 任务和执行状态分开记录；原生进程退出不单独代表任务完成。提交结果必须来自 Worker，协调者不代写产品实现或代 QA 判通过。
- Worker 不负责提交、推送、合并、部署或改变其他任务。控制器只提交该任务的允许路径；合并目标固定为实验分支，拒绝 main/master。
- 不绕过原生审批或沙箱。权限拒绝、模型限额、超时、中断和未知结果保留为失败/阻塞证据，不自动重复执行。
- 先验证小规模真实并发，再逐级提高。配置支持最多 20 个并发执行位置，配置值不等于实测容量。

## 最小组成

| 部分 | 责任 |
|---|---|
| Leader | 阅读项目与需求，提出任务目标、依赖和允许修改路径 |
| SQLite 任务库 | 持久化需求、任务、领取、原生会话、消息、结果和时间记录 |
| 调度器 | 原子领取、并发限额、worktree 准备、原生进程生命周期与补位 |
| Worker | 使用原生工具和项目 Skills 实现任务，通过 MCP 交接结果或报告阻塞 |
| 看板与汇合 | 展示真实状态和原生结果；把已提交候选合入实验分支，保留冲突 |

任务池是状态事实源；项目长期知识仍保存在仓库 AGENTS.md、docs 和 Skills 中。消息不自动等于任务完成或用户授权。

## 验证目标

1. 真实 Leader 从 Model Relay 文档与代码提出可并行的实际任务。
2. 多个原生 Worker 的运行时间真实重叠，各有独立会话、worktree、分支和结果。
3. 依赖任务不会提前执行，队列自动补位，同一任务不会重复领取。
4. Worker 的结果与具体提交关联；独立复核和实际项目验证由实验 Agent 执行并保存证据。
5. 中断后保留现场，不把失联任务重放成成功；汇合冲突显式呈现。
6. 报告区分等待、Agent 执行、验证、集成耗时及 Token/资源信息；不把 PR 数或 Agent 数当作质量结论。

## 运行

需要 Python 3.10+、Git、已登录的原生 Codex CLI。本次使用 Codex CLI 0.151.0；程序不复制凭据、不更改全局配置，不指定模型时使用原生配置。

在独立目录检出已创建的实验分支。Model Relay 历史 `docs/workflow` 含大量证据文件，可用标准 sparse checkout 保留项目代码与 Markdown 文档：

```sh
git clone --depth 1 --filter=blob:none --no-checkout \
  --branch agent-team/experiment https://github.com/big91987/model-relay.git model-relay
git -C model-relay sparse-checkout set --no-cone '/**' '!/docs/workflow/**' '/docs/workflow/**/*.md'
git -C model-relay read-tree -mu HEAD
```

任务运行目录与集成 checkout 必须为彼此独立的目录。以下从本维护源运行；request.md 写本轮用户目标：

```sh
python3 examples/agent-team-factory/factory.py --root /private/runtime init \
  --repository /private/model-relay --concurrency 3 --timeout-seconds 1800
python3 examples/agent-team-factory/factory.py --root /private/runtime plan \
  --request-file /private/request.md --task-count 4
python3 examples/agent-team-factory/factory.py --root /private/runtime run
python3 examples/agent-team-factory/factory.py --root /private/runtime status
python3 examples/agent-team-factory/factory.py --root /private/runtime serve --port 8977
python3 examples/agent-team-factory/factory.py --root /private/runtime integrate --task TASK_ID
```

`run` 的退出表示本批可执行任务已返回，不代表产品通过。`ready` 只是 Worker 已交接、原生进程退出、控制器已检查范围并保存候选；`integrate` 是显式选择后的实验分支汇合，不包含产品验收或部署。依赖等待前置任务实际合入实验分支。

`Ctrl+C` / `SIGTERM` 请求停止控制器及其原生进程组，保留任务、会话和文件。启动时遇到旧 running 记录拒绝重放。本版尚未提供完整的显式原生会话接续或冲突解决入口：冲突保留现场、阻止新任务领取，不能把“保留并拒绝重试”描述成恢复已经完成。通过 `message --to TASK_ID --text '补充说明'` 可留存输入，但不会单独启动 Agent。

部署到其他机器时，从已提交源码生成归档，记录精确提交和 SHA-256，再调用本目录 `install.py --archive ... --root ... --revision ... --sha256 ...`。安装到不可变的 `releases/<revision>/`，从返回的 entry 调用；升级使用新提交，不覆盖运行中的源文件和 MCP 工具。

## 验证

```sh
python3 -m unittest discover -s examples/agent-team-factory -p 'test_*.py' -v
ruff check examples/agent-team-factory
ruff format --check examples/agent-team-factory
```

进程生命周期测试与 HTTP 测试需要真实子进程和回环监听权限。受限沙箱内的失败应区分环境原因；沙箱也可能提前杀死探针后代而掩盖生命周期问题，因此须在实际执行主机再跑。`test_runner.py` 的临时可执行文件是明确标识的协议探针，不是模型、产品实现或真实 Agent 吞吐证据。
