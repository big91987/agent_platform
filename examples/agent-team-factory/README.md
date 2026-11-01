# Agent Team 软件工厂实验

## 当前用户试用入口

用户自主提交需求、Leader持续派发与固定Worker池的独立原型使用[playground.py](playground.py)，说明见[用户试用说明](PLAYGROUND.md)。它有独立的运行库与实时页面，启动为空，不导入以下已结束的Model Relay批次实验或补造用户需求。五个Skill的唯一维护源在`team_skills/`，由Worker任务输入实际加载对应正文。

## 早期批次实验记录

本例验证一个独立的软件工厂：真实 Leader Agent 拆解需求，多个真实原生 Agent 在独立 Git worktree 中执行任务，共享持久化任务、消息和结果，成果只合入专用实验分支。它不启动或修改已有 Agent Platform 工作流，也不部署 Model Relay。

这是实验原型，已实测20个原生Worker同时执行静态审查；首轮17项有效交接、3项失败。它不能代表20个功能并发交付或生产可用。实际执行、恢复及质量结论见[实验报告](../../docs/03-delivery/agent-team-factory-experiment-2026-11-01.md)，隔离进程探针与真实Agent分别记录。

## 边界

- 维护源仅在本目录；运行数据、原生日志、模型会话和实验 checkout 放在 Git 忽略的独立目录。
- 平台源码分支：`codex/agent-team-factory-experiment`。
- 测试仓汇合分支：`agent-team/experiment`。任务分支统一为 `agent-team/task-<task-id>-<attempt>`，worktree 目录含 `agent-team` 标记。
- 任务和执行状态分开记录；原生进程退出不单独代表任务完成。提交结果必须来自 Worker，协调者不代写产品实现或代 QA 判通过。
- Worker 不负责提交、推送、合并、部署或改变其他任务。控制器只提交该任务的允许路径；合并目标固定为实验分支，拒绝 main/master。
- 保留原生任务沙箱，不全局批准 Shell 或外部工具。`exec` 非交互模式无法展示审批，因此仅为本例已授权、绑定当前任务的 `get_task`、`send_message`、`handoff` 配置逐工具预授权；不启用 auto_review。其他权限拒绝、模型限额、超时、中断和未知结果保留为失败/阻塞证据，不自动重复执行。
- 先验证小规模真实并发，再逐级提高。配置支持最多 20 个并发执行位置，配置值不等于实测容量。
- 任务 MCP 是必需依赖，原生启动/恢复初始化失败必须报错，启动等待上限30秒；Worker先取得真实get_task回执再工作。进程exit0或生成报告都不能代替handoff。本轮曾出现20并发中3个会话自报工具未暴露，原始失败保留；仅凭该自报不能确定是MCP启动超时还是模型未正确使用工具。
- 当前固定Native的MCP采用延迟发现，Worker先通过原生工具目录查找实际名称，不能把未直接列出的工具当作未配置。平台保存的本任务用户补充在首次调用前直接进入原生输入，恢复建议不再依赖失效的get_task才能读取；其他任务输入和Peer讨论仍不作为用户补充注入。该投递不改变沙箱或审批权限。
- `user`与`leader`保留为系统消息身份，禁止作为任务ID。已有安装若有同名任务，消息来源已存在歧义，用户补充投递、消息写入及恢复都会明确拒绝；不猜测来源、不自动改写旧消息或更名。

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

需要 Python 3.10+、Git 2.38+、已登录的原生 Codex CLI。Git需要支持独立计算合并树的`merge-tree --write-tree`，旧版会在进入执行前明确报错；本次控制端Git2.50.1、执行端2.55.0，能力依据见[Git 2.38发布说明](https://github.com/git/git/blob/v2.38.0/Documentation/RelNotes/2.38.0.txt)。本次使用 Codex CLI 0.151.0；程序不复制凭据、不更改全局配置，不指定模型时使用原生配置。

可以在独立目录安装本实验已验证的精确执行器版本，再使用该绝对路径初始化。安装不修改全局 CLI；需要已有的原生登录状态，不能通过复制凭据文件替代登录。

```sh
npm install --prefix /private/factory-native --save-exact @openai/codex@0.151.0
/private/factory-native/node_modules/.bin/codex --version
/private/factory-native/node_modules/.bin/codex login status
```

目标仓库的语言工具链仍按其锁文件准备；本轮 Model Relay 实测 Go1.25.6 与 Node22。下方 init 增加 `--codex /private/factory-native/node_modules/.bin/codex` 可使用上述固定版本。原生文件沙箱策略与逐工具审批配置以本版本为验证依据，不声称任意旧版 CLI 可运行。

在独立目录完整检出已创建的实验分支。项目验证可能读取跟踪的历史证据，不能仅凭文件类型做 sparse checkout；本次实际门禁已证明只检出 Markdown 会漏掉必须的基线日志。

```sh
git clone --branch agent-team/experiment \
  https://github.com/big91987/model-relay.git model-relay
```

任务运行目录与集成 checkout 必须为彼此独立的目录。以下从本维护源运行；request.md 写本轮用户目标：

任务输入必须给出可读的实际材料和版本。跨仓任务应分别列出各仓的固定提交、只读源码入口及交付位置；测试仓的 worktree 不包含工厂维护源。需要逐项时序或证据作图时，提供原始逐项记录及摘要，不能只给总数、中位数而要求 Worker 反推。缺输入应真实 blocked，再通过 `resume` 补充并接续原会话，不填占位数据。

```sh
python3 examples/agent-team-factory/factory.py --root /private/runtime init \
  --repository /private/model-relay --concurrency 3 --timeout-seconds 1800
python3 examples/agent-team-factory/factory.py --root /private/runtime plan \
  --request-file /private/request.md --task-count 4
python3 examples/agent-team-factory/factory.py --root /private/runtime run
python3 examples/agent-team-factory/factory.py --root /private/runtime status
python3 examples/agent-team-factory/factory.py --root /private/runtime set-concurrency --limit 8
python3 examples/agent-team-factory/factory.py --root /private/runtime resume --task TASK_ID --message '已处理的原因与新的输入'
python3 examples/agent-team-factory/factory.py --root /private/runtime serve --port 8977
python3 examples/agent-team-factory/factory.py --root /private/runtime integrate --task TASK_ID
```

`run` 的退出表示本批可执行任务已返回，不代表产品通过。`ready` 只是 Worker 已交接、原生进程退出、控制器已检查范围并保存候选；`integrate` 是显式选择后的实验分支汇合，不包含产品验收或部署。依赖等待前置任务实际合入实验分支。

汇合使用 squash：原生候选及接续历史留在私有执行仓，实验分支只接收最终内容，避免已清理的私有诊断经旧候选父提交再次公开。`result.commit`关联私有候选，`result.integration_commit`是公开实验版本，验收与祖先检查使用后者。清理最终内容仍是发布前必要检查；squash不会识别或清理内容。旧实验已发布历史保留为事实，不重写。分支冲突仍保留现场，空内容汇合不造提交。

控制器在merge hook之前独立计算预期树，提交后再核分支、工作区、实际树及唯一公开父提交。冲突、中断或hook改变结果会保留私有`integration-pending.json`，阻止后续派发、汇合和bundle导出；不自动删除记录、重放或撤销Git修改。当前没有该类不确定集成的自动恢复入口，不能把保留现场当作已经恢复。hooks仍执行，本检查不代表隔离或信任任意第三方hook。

`Ctrl+C` / `SIGTERM` 请求停止控制器及其原生进程组，保留任务、会话和文件。启动时遇到旧 running 记录拒绝重放。`resume` 仅接受已确认原生进程退出、保留会话及原工作区的失败/阻塞/中断任务或尚未合并的候选，下一次 `run` 从同一原生会话和工作区接续，旧日志不改写；缺会话、原进程组仍在、提交已改变或终止结果未知则拒绝。恢复有隔离协议回归，真实原生接续须以本次实际回执为准。

冲突解决及崩溃后不确定进程的核验入口仍未完成：冲突保留现场、阻止新任务领取，不能把“保留并拒绝重试”描述成完整恢复。通过 `message --to TASK_ID --text '补充说明'` 可留存输入，但不会单独启动 Agent。`set-concurrency` 只在无运行任务时允许显式修改，并记录变更。

发现任务依赖划错时，只能在任务从未领取且仍 pending 时修正；已经执行或恢复过的任务定义保持冻结。管理员使用 `set-dependencies --task TASK_ID --depends-on-json '["INPUT_TASK"]' --reason '具体修正依据'` 显式替换依赖；清空须明确传 `[]`。入口与调度共用控制器锁，拒绝未知任务、重复依赖及循环，保留变更前后与原因。修正排队关系不代表计划评审或产品验收通过；实际执行仍须满足该任务的业务门槛。

部署到其他机器时，从已提交源码生成归档，记录精确提交和 SHA-256，再调用本目录 `install.py --archive ... --root ... --revision ... --sha256 ...`。安装到不可变的 `releases/<revision>/`，从返回的 entry 调用；升级使用新提交，不覆盖运行中的源文件和 MCP 工具。

## 验证

```sh
python3 -m unittest discover -s examples/agent-team-factory -p 'test_*.py' -v
ruff check examples/agent-team-factory
ruff format --check examples/agent-team-factory
```

进程生命周期测试与 HTTP 测试需要真实子进程和回环监听权限。受限沙箱内的失败应区分环境原因；沙箱也可能提前杀死探针后代而掩盖生命周期问题，因此须在实际执行主机再跑。`test_runner.py` 的临时可执行文件是明确标识的协议探针，不是模型、产品实现或真实 Agent 吞吐证据。

## 执行依赖与测试网络

维护者在无运行任务时通过正式入口配置非凭据环境变量及网络策略；配置持久化，后续任务及显式 resume 使用同一配置，不改全局 shell 或在途会话。

```sh
python3 factory.py --root /private/runtime configure-execution \
  --env-json '{"PATH":"/pinned/go/bin:/pinned/node/bin:/usr/local/bin:/usr/bin:/bin","GOTOOLCHAIN":"local","GOWORK":"off"}' \
  --network enabled
```

启用网络时，可写任务继承原生 `:workspace` 文件系统保护，显式允许网络（含回环测试服务与外网）。仅研究的只读任务保持原策略。关闭使用 `--network disabled`；没有配置时保持原默认。此配置不会批准 Shell 提权或启用全文件系统访问。本实验在 112 上实测原生沙箱回环监听成功、树内可写、树外不可写。项目检查仍由 Worker 在该沙箱执行；没有让 MCP 在宿主执行仓库代码的工具。

Go 版本须满足目标仓库 go.mod；安装完成后固定 PATH，使项目 Makefile 的 GOTOOLCHAIN=local 使用正确版本。仓库门禁及浏览器依赖准备遵循项目入口，不跳过历史证据、不改测试断言。依赖缓存、网络等待与验证时间分别计入实验报告。

候选在环境修复或审查反馈后可通过 `resume` 从原会话、原工作树及已保存候选提交接续；已合并任务不能重开。每次回执和原生输入/输出单独保留。重新验证不自动改变首次失败结论。

## 执行机与控制端分离发布

执行机可以只持有原生 Agent 登录，GitHub 写凭据保留在可信控制端。Worker 不能调用发布工具；只有操作控制器的人在任务已收敛后选择精确实验 HEAD，经正式入口导出增量 Git bundle：

```sh
python3 factory.py --root /private/runtime bundle --expected-head FULL_40_HEX_COMMIT
```

该入口需要干净的实验分支与其 origin 同名跟踪基线，和调度/恢复/汇合共用控制器锁；输出 bundle 路径、SHA-256、目标分支及精确 HEAD。通过受信通道把 bundle 复制到控制端的 Git 忽略目录，核对摘要后发布：

```sh
python3 publish.py --bundle /private/evidence/candidate.bundle \
  --sha256 FULL_64_HEX_SHA256 --expected-head FULL_40_HEX_COMMIT \
  --branch agent-team/experiment \
  --remote https://github.com/OWNER/REPOSITORY.git --github-auth
```

`--github-auth` 复用控制端已有的 `gh` 认证助手，不输出、复制或保存令牌到执行机。其他 Git 服务使用控制端既有 Git 凭据配置；HTTPS URL 不接受内嵌凭据。代理使用 Git 标准环境变量（如 `HTTPS_PROXY`），不修改全局配置。

发布器固定已校验字节的私有副本，核验单一实验 ref 与导入 HEAD，再按授权 SHA 普通非强制 push，并读回远端。摘要不符、主线目标或远端分叉均拒绝。每次独立回执保存在 bundle 同目录 `publish-receipts/`；push 前原子保存 `unknown`，中断或响应丢失不记成失败未发生。再次调用先核远端，已是目标 SHA 时返回 `already_published`；远端无法确认时保留未知，不自动重试或强推。不要把 unknown 回执当作可重放授权。

本地 Git 夹具覆盖提交SHA保持、主线不变、远端分叉、摘要/HEAD不符、校验后输入文件替换、真实push后响应丢失与SIGTERM；真实GitHub发布仍须以远端读回为准。
