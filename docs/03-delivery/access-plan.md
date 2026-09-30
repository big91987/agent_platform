# 用户与调用方会话入口实施计划

目标：管理员 admin/admin 配置 Agent、调用方和用户；Runner 通过 API 创建会话并生成有时效的授权链接；发起者从 Issue 打开指定会话并与原 Agent 接续。

沿用 Go 单体、SQLite、原生 Session 和队列。新增用户（admin/caller）、持久化登录及限定一个会话的临时授权。调用方账户绑定已有调用凭据和稳定 user_id；每次授权重读账户及调用方启用状态。会话 ID 是 conversation_id，不要求调用方知道原生 thread_id。

- [x] auth.go / auth_test.go：账号密码登录、角色、账号管理、禁用立即生效；迁移保留已有会话，默认 admin/admin 尚待自动审批要求的直接确认，当前保留原随机密码。
- [x] http.go / store.go：限定 conversation_id/user_id 的临时链接（默认 1 小时、最多 24 小时）；授权不可跨会话/用户/调用方；页面使用同源 HttpOnly cookie，链接 token 放 URL fragment 并在兑换后去掉。
- [x] web/app.js / style.css：参考 saibotan 和 sifamily 的蓝白管理界面；管理端展示配置/用户/调用方/API 文档；调用方仅展示授权会话、历史、输入和产物；明确同一 Agent 会话接续。
- [x] API 页面/文档/Runner 示例：参数、示例、返回值、查询消息、签发入口和接续；示例不嵌入凭据。
- [x] 实操：管理员 UI 创建 Agent 和调用方；Runner 真实请求 Codex requirements 澄清；Issue 链接访问、回复、恢复同一原生 thread；越权/过期/禁用/刷新与重启验证；手机及桌面 UI。

拒绝额外研发状态机、任务 ID、消息队列、远程执行器或审批框架。临时链接具备授权效力；调用系统只提供给相应发起者，平台不把公开链接等同于 GitHub 身份登录。

待办：默认 admin/admin 的独立确认。其他已实施事项及真实证据见 access-verification.md。
