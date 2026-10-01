# GitHub → 本机 Agent 平台

Runner 只负责请求、结果查询和 Issue 回复；Agent 在本机平台后台执行。

1. 创建平台用户，在 Agent 配置中勾选该用户。
2. 在用户页面生成此用户的 API Token，保存到本机忽略的配置文件。
3. 将 requirements.yml 安装到业务仓库的 .github/workflows/agent-platform.yml，配置仓库变量 AGENT_PLATFORM_ROOT 为本机平台目录。
4. 在 Actions 手动运行，issue 填产品 Issue 编号。首次提交标题和正文；已有会话且 message 为空时只读取结果，message 非空时接续原会话。

私有配置示例（不得提交）：

```json
{"base_url":"<service-endpoint>","agent_id":"<agent-id>","token":"<user-api-token>"}
```

Runner 从 Token 确定平台用户，不把任意 Issue 作者字符串当作平台身份。Issue 与会话关联记录在可信作者评论的隐藏标记中；平台仍是会话、输入和原生历史的事实源。首次请求使用稳定业务键去重，后续请求使用运行 ID。

回复包含 Agent 最终消息、固定 conversation_url、会话 ID 和账号登录说明。链接不含 Token、无到期时间、不需刷新。用户未登录时输入用户名密码，随后自动回到原会话。message 留空读取结果不会重放 Agent。

已有来源级凭据升级后失效，管理员为对应平台账号生成用户 Token，更新上述私有配置。网页账号和 API Token 属于同一用户；控制台撤销 Token 不影响网页登录和会话记录。
