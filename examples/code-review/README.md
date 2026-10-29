# 独立代码审查

同一份仓库变更可以用两种方式审查：`cli` 由 OCR 自己调用模型和工具；`delegate` 由 Codex 使用官方 Skill 审查，OCR 只提供文件选择及规则。两种方式各有一张平台 Workflow，复用普通执行节点和 Agent 节点，不增加平台专用节点类型。

## 安装与使用

1. 从明确提交使用 `examples/platform-workflows/export.py --destination <新目录> --ref <提交>` 导出模板。复制到项目的 `.harness/agent-platform`，审查项目定制，不在 CI 中读取平台开发工作树。
2. 平台部署的私有 `runtime.env` 配置 `CODE_REVIEW_URL`（完整 Chat Completions URL）、`CODE_REVIEW_TOKEN`、`CODE_REVIEW_MODEL`、`CODE_REVIEW_PROTOCOL=openai`。保持0600；不要写入图或仓库。按 `deploy/platform/README.md` 的标准安装激活流程应用环境，先确认没有执行中的任务。只在 CLI 审查节点通过 `env_refs` 引用；Git 凭据只进入 prepare，Codex 不继承这两类凭据。
3. 在平台主机执行项目副本中的安装器：

```sh
python3 .harness/agent-platform/examples/code-review/install.py \
  --platform-url <平台地址> --token-file <私有管理员API令牌文件> \
  --repository <owner/repository> --credential-ref <后端Git凭据名称> \
  --skill <绝对路径>/.harness/agent-platform/examples/code-review/skill \
  --manifest <私有安装清单.json> --authorized-user <CI用户ID> \
  --proxy <可选代理地址>
```

安装保存两张图的正式回执；重入复用原 ID，配置漂移拒绝覆盖，升级前阻止执行中或等待中的 Run。Skill 是官方 v1.13.3 原件，许可证和摘要见 `skill/`；平台从该目录将选中的 Skill 传给原生 Codex。

4. CI 私有 JSON 由项目管理，含 `repository`、`base_url`、`token_file`、`review_workflow_ids`（安装返回的 cli/delegate ID），可选 `git_proxy`；文件及 Token 保持在 Runner 的私有目录。GitHub 仓库变量 `AGENT_PLATFORM_CODE_REVIEW_CONFIG` 指向该文件。Token 对应用户需要能调用这两张图。
5. 把 `code-review.yml` **复制**到项目 `.github/workflows/agent-platform-code-review.yml` 并调整 Runner 标签。本例沿用可连接私有平台的自托管 Runner。PR 入口只从可信 base checkout 适配器，拒绝 fork、非 Owner 和已关闭 PR，不在 Runner 上执行 PR 代码。两个 matrix job 分别提交相同 base/head。可通过 workflow_dispatch 指定开放 PR；`engine` 选择两种一起运行或单独一种，不必为诊断重复消耗另一模型。

平台 Run 按 `PR/base/head/engine` 去重。后续提交或基线移动产生新的范围，旧报告不会标记新提交通过。此模板不改分支保护、不自动合并、不部署。

自动 PR 事件排队期间，如果 PR 已关闭或合并，入口跳过过期事件，不创建 Run、不覆盖提交状态，也不将跳过称作审查通过。手动请求关闭的 PR 仍明确拒绝。

## 研发图内使用

`workflows.add_cli_review(graph, model_refs)` 给开发模板显式插入 `tests → code_review → qa`。review 是普通执行节点；发现问题回 development，审查不完整或执行错误停止，不把基础设施失败交给产品开发盲修。审查前用独立 Git index 生成只在本工作区存在的候选 commit，不改 HEAD 或用户 index，覆盖未提交的实现；同版源码在审查前后逐文件摘要核对。固定测试和独立 QA 保留。

本函数是模板组合入口，不是平台硬编码行为；实际采用哪种审查、门槛和模型，由项目维护其已安装图。

## 适用的信任范围

标准入口只接收仓库 Owner 本人发起、同一仓库内的开放 PR；fork、其他作者和其他仓库拒绝。此例用于团队自己的可信仓库试点，不提供恶意仓库的安全评测环境。

当前 OpenSandbox 后端要求显式允许联网，不支持按角色禁网或只读；配置禁网会在启动前被拒绝。代理用于连接模型和已配置服务，不是出网目的地白名单。因此不能把容器隔离解释为阻止源码外传；Git/独立CLI模型密钥不提供给 Codex 也不能消除此风险。需要处理不可信内容或更严格的数据边界时，应先提供并验证环境出网约束，不能直接扩大此模板的适用范围。审查材料仍按不可信文本处理，不作为执行指令。

## 结果与边界

OCR 固定 `v1.13.3`、官方发布二进制 SHA256；完整模式 low effort、并发2、总 token 预算阈值3000000（OCR按回合检查，实际可能超出）。模型单次请求最多600秒、文件分组最多30分钟、整条OCR命令最多45分钟，外层执行节点50分钟；CI观察60分钟、GitHub job 75分钟，为准备及报告留出时间。独立审查准备节点最多600秒，其中固定base/head历史拉取最多480秒；不会自动重试失败拉取。OCR需要merge-base，保留真实历史。Agent的 `turn_timeout_seconds` 限制原生在途回合，Delegate默认1800秒；节点的 `execution_timeout_seconds` 仍只限制回合结束后的自动接续，两者分开。凭据不会进入模型报告，OCR 进程不继承 Git 认证。可信准备、审查及报告校验以 `python3 -I` 启动，避免从PR工作区加载同名Python模块或启动钩子；这不改变用户代码节点的导入方式。

`rules.json` 显式纳入测试，叠加系统规则；排除的历史大报告列在 preview，不能当已审查。PR 内的 OCR 配置不会替换受信规则。报告保留严重度、位置和依据；默认所有已报告发现均需人工核实。**clean** 表示覆盖完整且没有报告发现，**findings** 表示有待核实的问题，**incomplete** 表示漏项/警告/无法分类，**error** 表示执行或源文件变动。CLI exit0 或空 comments 均不能独立判通过。Codex 报告使用宿主自己的文件覆盖表，不伪造 OCR 原生清单。

报告、原始 JSON、stderr、文件选择、规则和源摘要按 Run 归档；CI 提供版本绑定的 commit status 与报告入口。平台页面需要登录；GitHub 附件是下载文件，不是部署在线看板。模型输出是建议，代码审查不替代测试、独立产品 QA 或人工合并。

## 当前验证

2026-10-26，用户指定 GLM 模型真实连接 HTTP200。完整官方 OCR 在独立认证缺陷样例上完成工具循环，91.275秒、20375 token，报告预埋的 null/null 放行问题及一条原先存在的低级建议。仅为 Harness 接入探针；不是产品验收、完整PR审查或模型质量评测。平台/PR入口及两种模式完整对照的正式回执见交付验证记录。
