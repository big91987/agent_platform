# 用户 Token 与账号会话入口整改

已确认方向：Agent 配置授权用户；每个用户有固定 User ID、账号密码和一个 API Token；网页只提供固定会话链接，登录后自动回到原会话。取消临时登录 Token、链接期限和刷新操作。

实施内容：

- Agent authorized_users 与用户 Token，统一网页和 API 的会话归属；拒绝通过 user_id 冒充其他用户。
- 用户页面生成、重置、撤销 Token；Agent 页面选择授权用户。
- 移除临时签发、兑换接口与对应页面，保留正常密码登录和固定 URL。
- 事务迁移旧账号、授权及会话；保留原生 Session 和产物。旧来源 Token 不自动升级为用户身份。
- GitHub Runner 只贴固定链接，不发临时凭证或刷新入口。
- 验证用户隔离、API/网页续接、撤销、迁移、登录跳转和真实 Issue 入口。

验证结果记入 access-verification.md。原有队列和原生执行逻辑继续沿用。
