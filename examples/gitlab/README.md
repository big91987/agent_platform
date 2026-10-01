# GitLab Runner 接入

Runner 准备工作目录，使用 Python SDK 调用平台；平台维护授权、会话及原生执行。目录由 Runner 和执行 Agent 的 Runtime 共享，平台数据库和 CODEX_HOME 不放进 Runner 的共享项目目录。

## 准备

1. 在平台创建调用账号并授权目标 Agent，生成该账号的 API Token。
2. 执行 `scripts/setup-runner.sh`。将平台源码、安装好的 Runner 虚拟环境和 SDK 提供给自托管 Runner。
3. Runner 与 Runtime 都挂载同一个持久项目卷，例如 `/workspaces`，并保证路径一致及文件权限可用。本机实验无需挂载，双方使用同一个目录即可。
4. 配置 CI/CD 变量：

| 变量 | 含义 |
|---|---|
| AGENT_PLATFORM_URL | Runner 可访问的平台服务地址 |
| AGENT_PLATFORM_TOKEN | 平台用户 Token，作为 masked/protected secret 配置 |
| AGENT_PLATFORM_AGENT_ID | 获授权的 Agent |
| AGENT_PLATFORM_ROOT | Runner 看到的平台安装目录 |
| AGENT_PLATFORM_WORKSPACE_ROOT | 持久项目卷根目录 |
| PLATFORM_MESSAGE | 本次交给 Agent 的工作或问题 |
| PLATFORM_CONVERSATION_ID | 可选，接续已有会话；省略时创建 |
| PLATFORM_REQUEST_ID | 可选，外部输入事件键；默认项目、Pipeline 和 Job 名组合，Job retry 不换键 |

5. 将 `agent-platform.yml` 合入现有 `.gitlab-ci.yml`。示例限定 protected ref 的手动 Web Pipeline，使用管理员准备的 `agent-platform` Runner 标签。不要在能访问平台凭据和项目卷的 Runner 上执行不受信代码。

GitLab 原生权限决定谁能启动 protected Pipeline、谁能读取变量；平台 Token 决定该请求属于哪个用户以及可调用哪些 Agent。SDK 不自行授予权限。多个用户需要各自身份时，应使用各自的 Token，而不是通过 `user_id` 冒充另一个用户。

## 目录准备与交接

示例将当前提交独立克隆到 `<workspace-root>/<project-id>/<pipeline-id>`，外层 Runner 创建 `codex/pipeline-<pipeline-id>` 分支，再提交该目录。独立 clone 不依赖 Runner checkout 中的 `.git`，因此 CI 临时目录清理后，任务目录仍可使用。

`prepare.sh` 使用不含凭据的项目 URL 作为 remote；不复制 CI checkout 的认证配置。已有目录且输入基线匹配时保留目录，不 reset、不 checkout、不覆盖 Agent 修改；不匹配或准备中断时明确拒绝覆盖，需要检查该目录。工作目录不能在 Agent 执行或等待用户期间被 CI 清理。

前序 Job 下载的、经筛选的文件可在提交之前复制进工作区。示例支持 `handoff/`：在准备前将所需 PRD、设计或其他文件放入它，Agent 可直接读取；Owner 可以调整该拷贝位置。不得整包复制 CI 凭据或平台私有目录。

共享卷只提供文件可见性，不提供写入协调。Runner 在提交后不再修改这个目录；平台对相同目录的 Agent 执行串行。不同任务使用独立目录，避免跨任务互相覆盖。

接续传已有 `PLATFORM_CONVERSATION_ID` 时跳过目录准备，使用平台保存的原路径；新 Pipeline 的目录变量不会替换原会话目录。新输入使用新的请求键。原 Pipeline 同一 Job retry 会取得相同接收结果，不重复创建会话或执行。

## 结果与 CI 状态

`invoke.py` 先校验用户凭据，提交后立即保存 `agent-platform-result.json` 中的 receipt，然后等待本轮，补充 API 快照。日志打印会话链接、输入状态及对应 Agent 回复；下载包不包含 Token。

本次输入执行失败、停止，或尚未完成且会话已失败、停止或关闭时 Job 返回非零。已完成输入不会因为后续其他输入失败而被误报失败。观察超时只表示后台还在执行，Job 保留 receipt 和当前快照；用户通过返回的固定链接继续查看，不能将此 Job 成功等同于业务验收通过。

GitLab artifact 保存会话入口和结果快照，持久项目卷保留真实代码及阶段文件，平台保留 Session。不要用有过期时间的 CI artifact 代替全部会话存储。

示例使用 GitLab 官方 [Job artifacts](https://docs.gitlab.com/ci/jobs/job_artifacts/) 机制。前序文件由 GitLab 下载到 Job 工作目录，再由 Runner 显式选取交接；Docker Runner 的项目卷挂载参考 [Docker executor storage](https://docs.gitlab.com/runner/executors/docker/)。目前仅验证同机共享目录及脚本实际行为，尚未接入一台真实 GitLab Runner，也未实现云端沙箱隔离。
