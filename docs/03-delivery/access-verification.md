# 管理员、调用方与网页澄清验证

日期：2026-10-01。功能已实现，管理员登录为 admin/admin。

## 真实角色操作

- 管理员从真实网页登录，创建“需求澄清助手”、选择原生 grill-me Skill，并在接入页面创建 GitHub requirements Runner 调用方凭据。
- 管理员从用户页面创建 github-alice 调用方账号，绑定 Runner 来源与 github:alice。独立浏览器登录成功，只有“我的会话”导航，不能看到另一用户的 Issue 会话。
- 未登录的发起者从 GitHub Issue 里的临时链接进入，页面标记“临时会话访问”，只显示该 Agent 对话、产物及过程。fragment 兑换后移除，无管理员密码或 API key。

## 真实 GitHub 与原生执行

- 测试仓库安装 PR：https://github.com/big91987/reading_list/pull/62 （已按测试仓库授权合并）。
- 产品功能 Issue：https://github.com/big91987/reading_list/issues/63 （为每本书增加读书笔记）。
- 初次 Runner 作业：https://github.com/big91987/reading_list/actions/runs/36785279484 （success）。
- Agent 自主询问每书笔记数量及删书规则；发起者点击 Issue 链接，从平台提交明确规则。
- 两轮 native thread.started 的 thread_id 相同，原生渐进读取 selected-0/SKILL.md，实际符号链接指向 grill-me。
- 第二轮写出真实 prd.md（约 8.6 KB），明确已确认规则及 16 项验收标准，没有写产品代码。
- 再次 Runner 作业：https://github.com/big91987/reading_list/actions/runs/36786036929 （success）。message 留空读取同一会话结果，回贴 PRD 交接回复并刷新链接，原生轮数仍是两轮。
- 服务重启后临时页面授权仍有效，对话历史和 PRD 可查看。授权到期不因重启延长。

## 页面实操

- Agent 卡片的“会话 (1)”进入按该 Agent 过滤的列表，能打开 Issue 的原会话。
- 发起者查看真实 prd.md 内容成功；后台刷新不再重建未变化的产物按钮。
- 调用方账号直接打开另一用户会话，实际返回 403，页面显示“无权访问这段会话”及返回入口。
- 桌面 1440×1000 与手机 390×844 实际检查通过；手机 documentWidth 与 viewport 均为 390，回复区及产物入口可滚动到达。
- 截图：output/playwright/api-docs.png、caller-desktop.png、caller-mobile.png、caller-mobile-reply.png（均为实际浏览器渲染，忽略存储）。

## 检查与证据边界

Go 测试覆盖用户角色、账号停用、作用域、跨用户／来源／会话、临时入口过期、网页授权续接及禁止再授权；原有竞态、停止、重启、去重检查继续通过。Node 检查覆盖重复请求身份保留与输出格式的 HTML 转义。go fmt、go vet、race tests 和构建通过。

私有原生事件、真实请求及界面截图保存在忽略目录：`.data/access-evidence.json`、`output/playwright/`，不提交账号、口令、API key 或临时授权 token。

当前不声称支持远程 Runner、GitHub SSO、多人公网部署或强恶意代码隔离。管理员已通过正式账号管理 API 改为 admin/admin，接口登录返回 200，真实网页进入管理员工作台；正式平台重启后登录仍返回 200。临时全新数据目录首次启动也实测 admin/admin 返回 200。账号登录持久保存在数据库，重启不重置密码。

## 过期入口回归与 GitHub 账号

原 Issue 临时授权按 1 小时失效，调用方仍启用，会话和 PRD 均保留。已修复过期链接强制显示登录页的路由错误：有效账号继续按原权限访问，无有效账号显示持久的过期和登录说明；没有续期或绕过临时授权。Node 回归覆盖有效登录和匿名入口，Go race 检查通过。

正式 Runner 刷新作业 https://github.com/big91987/reading_list/actions/runs/36797481532 成功，回复 https://github.com/big91987/reading_list/issues/63#issuecomment-5922385515 包含临时、固定及刷新入口，消息总数保持 6，未再次执行 Agent。

按用户要求通过正式用户管理接口创建 github 调用方账号，绑定现有 GitHub Runner 来源及 github:big91987；实际登录和查询原 Issue 会话成功。口令仅交给用户，不写入本仓库。
