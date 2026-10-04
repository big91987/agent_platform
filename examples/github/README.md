# GitHub → 本机 Agent 平台

新仓库接入请从 [完整复现说明](GETTING_STARTED.md) 开始；验证工具已随 `tooling/` 分发，可选部署见 `local-preview/`。

## 单阶段接入（可选）

以下 `requirements.yml` 是早期单阶段示例，不与完整 Pipeline 同时安装。完整研发流程请使用上面的复现说明。

Runner 只负责请求、结果查询和 Issue 回复；Agent 在本机平台后台执行。

1. 创建平台用户，在 Agent 配置中勾选该用户。
2. 在用户页面生成此用户的 API Token，保存到本机忽略的配置文件；执行 `scripts/setup-runner.sh` 安装 Python SDK，工作流使用该虚拟环境。
3. 将 requirements.yml 安装到业务仓库的 .github/workflows/agent-platform.yml，配置仓库变量 AGENT_PLATFORM_ROOT 为本机平台目录。
4. 为任务准备本机项目目录（并发任务各用独立 Git worktree），在工作流 workspace_path 输入或私有配置中填它的绝对路径。此目录位于平台机器上，不是 GitHub 网页路径；Runner 和平台此版同机。平台直接使用目录，不克隆、不覆盖项目文件。
5. 在 Actions 手动运行，issue 填产品 Issue 编号。首次提交标题和正文；已有会话且 message 为空时只读取结果，message 非空时接续原会话。

私有配置示例（不得提交）：

```json
{"base_url":"<service-endpoint>","agent_id":"<agent-id>","token":"<user-api-token>","workspace_path":"<prepared-project-workspace>"}
```

Runner 从 Token 确定平台用户，不把任意 Issue 作者字符串当作平台身份。Issue 与会话关联记录在可信作者评论的隐藏标记中；平台仍是会话、输入和原生历史的事实源。首次请求使用稳定业务键去重，后续请求使用运行 ID。

回复包含 Agent 最终消息、固定 conversation_url、会话 ID 和账号登录说明。链接不含 Token、无到期时间、不需刷新。用户未登录时输入用户名密码，随后自动回到原会话。message 留空读取结果不会重放 Agent。

已有来源级凭据升级后失效，管理员为对应平台账号生成用户 Token，更新上述私有配置。网页账号和 API Token 属于同一用户；控制台撤销 Token 不影响网页登录和会话记录。

首次缺少工作目录时，业务接入脚本明确报错，不再启动空项目会话。会话创建后固定目录，message 接续不重复传路径；换项目或目录需创建新会话。原来的空目录会话不会被偷偷切换到业务仓库。

适配器调用统一 Python SDK，平台仍负责认证和授权。SDK 不自动重放请求。脚本按本次输入 ID 选择对应回复；等待到期不贴上一轮回复冒充本轮结果。Issue 关联搜索覆盖分页评论，不受前 100 条限制。

准备工作目录示例（由调用方执行，Agent 不负责创建分支）：

```sh
git -C <local-project-repository> fetch origin
git -C <local-project-repository> worktree add -b codex/<task-name> <prepared-project-workspace> origin/main
gh workflow run agent-platform.yml --repo <owner>/<repository> -f issue=<issue-number> -f workspace_path=<prepared-project-workspace>
```

工作目录需要持续保留；不要把会被 Runner 清理的 checkout 临时目录传给异步平台。可在本机私有配置中设置 workspace_path 默认值；工作流输入优先。该 API 面向受信同机调用方，目录参数不提供租户文件系统隔离。

## Pipeline with independent QA and code review

`pipeline.yml` replaces the single-stage workflow when requirements, design and
implementation should hand off through externally registered stdio MCP tools.
The platform remains generic. The integration lives in this directory and
`examples/pipeline-tool`; it uses existing platform APIs without adding pipeline
fields to the platform's schema.

On the same machine as the platform and self-hosted Runner:

1. Build `go build -o bin/pipeline-tool ./examples/pipeline-tool`.
2. Run `bash examples/github/install-tooling.sh` to install the bundled browser
   runtime, SDK, MCP binary and locked quality dependencies; no separate Harness checkout is required.
3. Extend the private Runner JSON (base_url, token, agent_id) with `pipeline`:

   ```json
   {
     "repository": "owner/repository",
     "checkout": "<persistent-product-clone>",
     "namespace": "owner-project",
     "workspaces": "<task-workspace-root>",
     "registry": "<private-integration-state-root>",
     "delivery_token_file": "<private-config-root>/github-delivery-token",
     "python": "<platform-root>/.data/runner-venv/bin/python",
     "skills_root": "<skill-assets-root>",
     "agents": {}
   }
   ```

4. Run `python3 examples/github/setup.py --config <private-runner-json>`.
   It prompts for the existing platform admin password, registers/discovers MCP
   servers, and creates or updates five independent stage Agents. Existing conversations
   retain their snapshots; use a new Issue to exercise updated bindings.
5. Install `pipeline.yml` as `.github/workflows/agent-platform.yml`, set the
   `AGENT_PLATFORM_ROOT` and per-repository `AGENT_PLATFORM_CONFIG` variables, and keep legacy workflows
   disabled. The local `gh` identity must have dispatch and delivery permission
   for the configured repository. `run.sh` reads this identity at process startup;
   tokens never enter model arguments, URLs or committed files. For report delivery,
   provision the existing integration credential in `delivery_token_file` (absolute
   path, mode 0600, outside the task workspace/repository). The unattended Runner
   reads that file instead of the desktop login keychain. The credential needs
   contents and pull-request write access to the configured repository; renew the
   private file when rotating it. Never put its value in the Runner JSON or logs.
6. Open a product Issue as owner. A non-owner Issue is started by an owner through
   workflow_dispatch (`after=start`). The Runner creates its task branch/worktree
   and publishes a platform conversation link immediately after saving input.
7. Clarify and approve requirements in the platform UI. The Agent calls
   `submit_handoff`; the tool snapshots documents and dispatches GitHub with the
   source stage and digest. GitHub waits for the previous turn to finish, verifies
   the handoff, and starts a distinct design conversation in the same workspace.
8. Review the prototype, architecture and contracts; approve in the design chat.
   Its MCP call starts development. Development has a native Stop check and real
   browser tool and implements/verifies the feature. It presents the deliverable
   and waits for explicit user confirmation before handing off to QA.
9. QA independently exercises the real product and writes `qa.md` with AC results,
   reproduction steps, screenshots and limitations. For a demonstrated product
   defect, QA chooses `target_stage=requirements`, `design` or `development` in
   `submit_handoff`. No extra approval is required just to return a defect. Tool
   or environment failures remain blocked/recoverable and are not product defects.
10. The receiver records the return on the original Issue and resumes the target
    role's own conversation with the QA feedback and current documents. Subsequent
    stages run again in new Workflow Runs, using the same task branch/workspace.
    Requirement or design changes are reviewed normally; unchanged decisions are
    not approved again. Passing QA requires human confirmation to hand off with
    `target_stage=report` and create a draft PR.
11. The report Job re-runs existing and feature browser journeys, syntax and diff
    checks, commits/pushes the task branch, and creates a draft PR. It never merges.
12. Code review is separate: click **Ready for review** on a pipeline-delivered
    PR to start the independent Reviewer when its GitHub merge reference includes
    this workflow version. Older PR references may still carry the previous
    workflow; use the manual entry below in that case. The Runner resolves its original Issue
    from the delivery record and verifies the registered checkout matches the PR
    head. A conversation link and the final findings appear on that PR.
    To request another review manually, run the workflow with `after=code_review` and the
    original Issue number after PR creation. An independent Agent inspects the
    branch and writes `code-review.md`; its final findings are posted to the PR
    and Issue. It has no handoff tool. Humans decide fixes and merging. The QA
    feedback loop does not use review as an automatic pass/fail routing stage.

Runner-owned registrations select credentials, destination and receipt paths.
The Agent supplies summary, relative UTF-8 document paths and an optional target
stage. Every product stage may return to an earlier stage. QA must explicitly choose this target; omission is rejected rather than
defaulting to delivery. A QA return increments the task round; every later stage gets a new input
and handoff receipt but retains its own conversation. Old reports remain in
immutable handoff snapshots. Previous verification does not approve changed code.

A dispatch carries source stage, receipt path and digest. The receiver verifies
that these belong to the active task round and snapshots match workspace files.
It persists the accepted transition before invoking the next Agent with a stable
request ID. Retrying that same transition reconnects instead of issuing another
input; callbacks from superseded stages or rounds fail. Duplicate queued Runs
may appear after a network-uncertain retry, but they cannot advance stale state.
The Issue-level concurrency group serializes CI Runs; the receiver also waits
for the source Agent to finish before handing the workspace to the next role.

The MCP tool saves evidence before HTTP dispatch. A successful dispatch is never
resent. After a rejected or uncertain request, repeat the same tool call; the
receiver deduplicates the receipt. Changed content cannot overwrite a receipt.
Saving a file alone does not trigger GitHub: the external tool explicitly calls
`workflow_dispatch`. No polling service or platform-specific pipeline state is
introduced.

The JSON registry contains integration receipts, not native session histories.
Native threads, messages and queue state remain owned by Agent Platform. This
example uses a shared local filesystem; it does not imply remote sandbox support.

Before enabling development, install the pinned quality tools with
`npm ci --ignore-scripts --prefix examples/github/quality`. The Agent receives
`verify.py --fix` for formatting/autofix and `verify.py` for checks. Native Stop
and delivery use the same check path (Prettier, ESLint, syntax, diff and browser
journeys). Do not replace real functional assertions with a formatter pass.

Runner report evidence is saved under `runner-checks/`; the Agent's own
`delivery-checks/` logs remain intact. A draft PR is a review handoff, not a claim
that outstanding human acceptance evidence has passed.

Product regression plans belong in `tests/browser/core.json`, where they can be
reviewed and updated with UI changes. When absent, the integration uses the
legacy `.harness/reading-core.json`; an invalid or empty product plan fails rather
than falling back. Migrate by preserving the existing business actions and
assertions, updating only obsolete locators, and include the plan in the product
PR. Harness execution controls remain protected. The feature-specific plan is
still required and is checked independently by the delivery Runner.

QA receipts bind to the product/test digest actually inspected. Missing QA evidence
or a later code change prevents publication. A human acceptance requirement is
an Agent instruction, not a machine-verifiable approval record; the integration
checks handoff integrity, not whether prose constitutes approval. Existing
conversations keep their original Agent configuration snapshots. Use a new Issue
to validate newly installed role/tool instructions.

## Re-verify after fixes

For manually applied repairs, the owner may select `after=verify` and the Issue
number once participating conversations are idle. This starts a fresh QA round
in its original conversation. It is not acceptance of QA findings or permission
to create a PR. Automatic QA return uses `target_stage` instead; it does not
require this manual entry.


## User-requested and Agent-proposed returns

At the active product stage, a user can request an earlier stage in natural
language. The Agent resolves the destination; an explicit user request needs no
second approval. In strict mode, an Agent-proposed return first needs human confirmation, except
QA may return an evidenced product defect autonomously. Same-stage corrections
stay in the conversation. Independent code review remains separate.

Use the same submit_handoff tool with target_stage. Its summary records the
reason, requested changes, affected conclusions and the user request/confirmation
(or QA evidence); artifacts preserve supporting documents. The receiver validates
the registered source and route, increments the round on any backward handoff,
and resumes the recipient's own conversation. Subsequent stages must run again;
old round callbacks cannot advance the new round. Natural-language approval is
interpreted by the Agent, not by keyword matching in the framework.

## 管理已发出的交接

`submit_handoff` 的成功回执包含 GitHub `run_id`、`run_url`。已有的 Run ID
就是后续管理句柄，不增加另一套任务 ID。GitHub dispatch 固定使用 API
`2022-11-28` 和 `return_run_details: true`；接收方会为响应丢失的交接补回 Run ID。

每个阶段注册三个外部 MCP 工具：

- `submit_handoff(summary, artifacts, target_stage?)`：完成当前阶段并交接。
- `supplement_handoff(run_id, content)`：向这次交接对应的目标会话追加内容。
  目标执行中时使用通用引导接口；尚未启动时保存到交接，由启动输入带入。
- `replace_handoff(run_id, content, target_stage)`：撤回旧交接，停止目标 Agent，
  取消尚未结束的 GitHub Run，再向指定阶段重新 dispatch。旧文件、历史和补充保留。

例如用户在原 QA 会话说“刚才回设计不对，请撤回，回需求重新确定范围”，
QA Agent 使用此前返回的 Run ID 调用 `replace_handoff(..., target_stage="requirements")`。
用户不必进入目标会话。接收 Run 在同一个 Issue 回复并复用任务工作区、分支与目标阶段会话。
本工具只管理 requirements/design/development/qa；不撤销已发布 PR 或代码合并。

新安装默认自动允许提交与补充，撤回使用 `confirm`，网页批准本次调用后执行。
审批策略属于通用工具绑定配置；`setup.py --tools-only` 只刷新既有 Agent 的工具绑定，
保留已配置的审批、模型、提示词和 Skill。既有会话使用创建时的工具快照。
部署 SDK 变更时重新执行 `scripts/setup-runner.sh`，再构建 MCP 工具。

交接后，上游会话切成通用只读工作区模式，可与下游写任务并行处理管理消息；
回到该阶段时恢复写权限。读写模式不是租户沙箱，外部 MCP 权限由工具注册与审批控制。
同一工作区仍最多一个写任务，全局执行并发上限照常生效。

撤回首先持久化失效标记；只有旧 Agent 确认停止才下发新 Run。停止操作带观察到的
最新输入 ID；此后新增输入会导致冲突，不会被相同请求重试顺带取消。撤回移除旧队列的
待执行资格，但保留消息记录。任何失败均保留可重试状态，完全相同的管理请求返回原结果。
旧句柄不能取消已经推进的其他阶段。目标已回到调用者自身阶段时，继续当前会话并用
正常 `submit_handoff` 推进，不允许管理工具停止自己的调用进程。

交接已被接收、但目标调用回执因中断尚未保存时，管理工具会拒绝猜测目标；先重试接收
Workflow，由稳定请求键恢复同一次调用，再管理。GitHub HTTP 响应不确定时可能出现
重复 Run，但接收方按冻结交接文件和摘要去重，不重复启动 Agent。


## Issue 自主推进与 PR 自动复验

新 Issue 专属 `### 自主推进` 区块中的 `- [x] 按推荐方案自主推进`
启用任务级自主模式；默认关闭。创建时保存，不随 Issue 后续编辑隐式变更。
所有阶段和返工沿用该策略：普通澄清选择有依据的推荐项，完成产物和实际验证后自动交接；
目标无法判断或 P0 重大风险仍请求人工决定。不得伪造用户确认或跳过 QA。
最终 PR 合并与高风险部署不属于自动授权。

安装 `pipeline.yml` 与 `pr-refresh.yml` 到项目工作流目录，并配置本地适配器。
Ready for review 和 main push 触发已登记 Ready PR 的自动同步；Draft/fork 不处理。
框架 merge 精确 main commit，研发 Agent 只编辑普通产品冲突，框架完成提交，再独立 QA 和代码审查。
`pipeline/refresh` 成功仅表示这一版本完成验证与审查，不代表人工批准或自动合并。
审查发现仍交给人判断。主线在验证中变化会重新启动同步；失败保留现场可重跑。
阶段工作流使用 `queue: max` 保留排队交接，不能用新刷新挤掉已提交的阶段回调。

`approval_policy.py` 只属于 GitHub 集成；通用平台的 Agent/工具/会话协议不增加研发阶段或审批语义。

### Browser runtime and delayed results

The browser MCP and verification gate use `pipeline.browser_source` when configured,
otherwise the bundled `examples/github/tooling` snapshot. Run
`bash examples/github/install-tooling.sh` to install its locked dependencies.
An optional owner-managed fixed Harness checkout can still be registered with
`setup.py --tools-only --browser-source <harness-checkout>` after preparing that
runtime. Keep overrides outside task workspaces. `pipeline.checkout` is only
the persistent product clone, not the default tool installation.

`check` supports real hover/pointer movement, touch contexts and taps, geometry
snapshots, and storage write observations. `verify` runs the owner quality gate on
the host and returns the result before handoff, avoiding Agent shell port restrictions.
Existing conversations retain their tool bindings; updated tools remain available
through the same `check` connection, while newly attached `verify` is available in
new conversations. A failed verification overwrites the current checks receipt with
its actual failure instead of leaving a previous success behind.

Deploy the matching `pipeline.yml` before relying on delayed-result continuation.
For a staged rollout, `setup.py --tools-only --observer-ref <workflow-branch>`
selects the owner-controlled observer workflow ref; the default is `main`.
This selects only result observation, not product stage or merge behavior.
After a Runner wait expires, `after=observe` continues receiving the original
conversation/message and posts its final reply to the same Issue. It never calls
`invoke`, never approves a stage, and never recreates a native Session. Replaced
inputs and previous iterations are ignored; comments are deduplicated by input ID.

### 框架维护 PR 的合并前验证

产品研发 Agent 不负责修改框架、工作流或部署器。未在任务交付记录中登记的同仓库 Ready PR，刷新扫描会明确上报 `pipeline/refresh` 失败并提示维护者验证，不再静默跳过；这不代表检查已运行失败，也不会启动产品 Agent。

由维护者审查工程改动、同步最新 main，并在可信宿主的私有 Runner 配置中设置 `pipeline.maintenance`：

```json
{
  "paths": [".github/workflows/deploy-local.yml", "scripts/local_deploy.py", "tests/local_deploy_test.py", "docs/local-deployment.md"],
  "checks": [["python3", "-m", "unittest", "discover", "-s", "tests", "-p", "local_deploy_test.py", "-v"]],
  "timeout_seconds": 600
}
```

路径是区分大小写的仓库相对 glob；优先列出具体文件，不要用 `**` 允许所有产品代码。检查是 argv 数组，不经过 shell，必须针对实际改动选择；上例仅为部署器单元/进程回归，不等于真实部署验收。涉及 Workflow 时还需配置 YAML/脚本检查，并验证真实 Actions 入口。可用 `["<platform-root>/.data/runner-venv/bin/python", "<platform-root>/examples/github/workflow_check.py", ".github/workflows/pr-refresh.yml"]` 检查 YAML 和 bash/sh 语法；其他 shell 明确报不支持。`install-tooling.sh` 安装锁定的 PyYAML 依赖。配置应保存在产品工作区外，不由产品 Agent 或待验 PR 修改。配置缺失时保持阻塞，不提供自动豁免。

使用干净、位于 PR 精确 head 的 checkout，由维护者执行：

```sh
GH_REPO=owner/project-a PYTHONPATH=sdk/python:examples/github \
  python3 examples/github/pr_refresh.py --config '<private-runner-json>' \
  --maintenance-pr <pr-number> --workspace '<clean-pr-checkout>'
```

命令先检查最新 main 已包含在 PR head 中、实际变更全部属于允许范围，再执行配置的所有检查；最后重新核对 PR、main 和工作区未变化才回报成功。失败可修复后重跑。不会合并代码、不会执行 Agent、不会替代维护者对测试充分性的审查；已登记产品 PR 不能走此入口代替 QA/Review。

本地执行日志和 head/main/检查配置凭证写入私有 `registry/maintenance/`，不提交仓库。主线、PR head 或检查配置变更后凭证失效；下一次扫描要求重新验证。分支规则保留 `pipeline/refresh` 并启用要求分支保持最新，防止主线更新与异步扫描之间的空窗。

升级：先更新可信宿主的这些源码（包含 `maintenance.py`），再同步 `pr-refresh.yml` 到目标仓库、配置维护范围与检查，最后手动运行 Refresh ready PRs 验证路由。旧配置无需迁移：已登记产品继续原有流程，未登记 PR 默认明确阻塞。不要伪造登记记录、人工补成功状态或以管理员强合作为标准流程。当前框架收敛期间，维护者验证由框架开发者负责，不增加工程维护 Agent。

### 后端及仓库特有的交付文件

默认产品范围为 `app/`、`tests/`、`.trellis/spec/`、`README.md`、`deploy/`，另可交付 `docs/` 文档。产品后端若在其他位置，由维护者在可信 Runner 配置设置 `pipeline.extra_product_files`，例如：

```json
{"extra_product_files": ["scripts/service.py", "scripts/install_service.py"]}
```

这是仓库级持久配置，与 Issue、会话无关。只允许精确的仓库相对文件路径，不允许目录、通配符、链接或 Harness 控制文件。先审查文件的职责和改动，再配置；不能为了通过发布而开放整个 `scripts/`。部署控制器等工程文件由维护者检查，配置交付范围不等于将框架维护授权给产品 Agent，也不代表安装服务、启用采集或放行合并。脚本冲突仍交维护者处理。

新增文件范围同时参与 QA 指纹、PR 发布和代码审查范围；内容或配置变化使旧 QA 失效。Runner 和开发 Stop Hook 都运行 `tests/*_test.py` 的 Python 回归（包括部署与后端），以及已有 Node 和浏览器检查。发布验证不能改动已通过 QA 的代码。默认配置的旧 QA 指纹兼容；增加范围后，通过 workflow_dispatch 的 `after=verify` 重新执行 QA，再由交接触发 report，不编辑旧检查点、不直接重跑旧 report 来复用旧结论。

升级时更新可信宿主源码（包括 `product_scope.py`、`pipeline.py`、`pr_refresh.py`、`verify.py`），记录采用的版本和仓库配置。新仓库安装相同版本并配置自己的额外文件即可获得同样的行为；无需修改产品 Workflow YAML。回退此配置同样需要重新 QA，不能用于忽略已交付的后端。
