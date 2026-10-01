# 本机项目工作目录验证

验证日期：2026-10-01。范围为受信调用方与平台同机执行；云端沙箱、仓库检出服务与对象存储未实现。

## 已验证行为

- 首次请求传已有项目目录的 `workspace_path`，实际 Codex 进程在该目录执行；原生配置、历史与凭据仍由平台管理。
- Agent 读取项目 `AGENTS.md`、README、项目说明和现有产品代码。平台不覆盖项目指令，不应用目录模板。
- 平台重启后，接续仅传 `conversation_id` 和新消息。真实 Codex 使用原来的 thread ID 和工作目录完成下一轮；没有新建空项目或替换原生 Session。
- 工作目录中的需求决策文件通过授权文件接口实际下载成功，页面显示工作目录及产物。
- 既有 11 段会话升级前后 ID、原生 thread、状态和消息数量保持一致；旧会话继续使用原目录。
- 路径缺失时拒绝执行，接续换目录返回冲突；同一目录的不同会话串行，包括关闭后进程仍在停止的区间。上述失败路径由针对行为的测试覆盖，未将替身执行器结果当作真实 Codex 证明。
- `scripts/verify.sh` 的格式、语法、Node 测试、Go vet、Go race 测试与构建通过；GitHub 适配脚本语法检查、差异空白检查及共享资产 sanitization 扫描通过。

## 真实业务链路

- [测试仓库接入 PR #65](https://github.com/big91987/reading_list/pull/65)：已合入，手动工作流支持传入准备好的本机目录。
- [产品 Issue #66：为每本书增加读书笔记](https://github.com/big91987/reading_list/issues/66)：使用标准 Git worktree 和任务分支，未使用 Runner 会清理的临时 checkout。
- [正式工作流运行](https://github.com/big91987/reading_list/actions/runs/36808874369)：成功提交平台，等待真实执行结果并回复 Issue。
- [Issue 中的回复与平台入口](https://github.com/big91987/reading_list/issues/66#issuecomment-5923923486)：Agent 基于现有代码询问每本书单篇或多篇笔记，没有再索要仓库路径。
- 平台会话：`ad6319dbc2271e8c28c08690674d369b`。重启后以自然语言追问现有存储结构，Agent 正确引用 `app/app.js` 的字段与存储行为；未替用户确认产品方案，未修改产品代码。
- 已生成并下载：`docs/04-implementation/tasks/reading-notes/requirements-decisions.md`。

本次仅验证工作目录接入、文件访问和原生接续，不代表读书笔记功能已经设计或实现。浏览器检查截图保存在本地忽略的 `output/playwright/`，不将个人路径、凭据或原生历史提交到仓库。

## 保留边界

目录路径不是安全隔离机制。当前调用方需为并发任务准备独立目录，持续保留项目文件；平台不克隆仓库、不创建任务分支、不删除外部项目目录。未来云端接入应由平台准备沙箱工作区，GitLab 提交代码来源及产物位置，具体存储与权限方案另行设计。
