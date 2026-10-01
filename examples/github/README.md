# GitHub → 本机 Agent 平台

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
