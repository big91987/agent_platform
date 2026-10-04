# Agent Platform

本机通用 Agent 平台实验版。配置一个原生 Codex，通过 API、Webhook 或网页创建会话，随后在同一会话继续交流、观察执行和查看文件。研发阶段和产出形式由 Agent 配置与用户输入决定。

文档见 [docs 索引](docs/README.md)；各类 Agent 工作流的架构、实践与选型见 [工作流方案目录](docs/02-architecture/workflows/README.md)。

## 启动与使用

需要 Go 1.25+、C 编译器、Python 3，以及已安装并完成 `codex login` 的 Codex CLI。当前实际验证版本是 Codex 0.151.0。Node 用于前端检查，不是服务运行依赖。

```sh
./scripts/start.sh
# 打开 http://127.0.0.1:8788
# 首次启动管理员账号 / 密码：admin / admin；账号密码存于数据库，重启不会重置。
```

1. 在 **智能体** 中选择本机 Codex，配置指令、模型与允许使用的 Skill，执行“检查”，然后试运行。
2. 在 **会话** 中发起请求。进展、最终回复和工具日志来自真实原生执行器；图片直接显示，文本可查看，所有产物均可下载。
3. 在 **用户与角色** 创建账号并生成用户 Token，在 **智能体** 配置中勾选授权用户。调用系统保存返回的会话链接；用户登录后打开原会话接续。

```sh
./scripts/stop.sh
./scripts/start.sh  # 保留会话、原生记录和文件；重启保留登录有效期，过期后重新登录。
```

自定义监听地址、数据位置或并发上限时直接运行：

```sh
go build -o bin/agent-platform ./cmd/agent-platform
./bin/agent-platform -listen 127.0.0.1:8788 -data .data -concurrency 2
```

首次初始化可用 `AGENT_PLATFORM_PASSWORD` 设置自定义管理员密码（至少 12 字符）；已有账号通过用户管理修改，启动配置不覆盖它。

可选参数：`-base-url`、`-codex`、`-auth-home`。默认只监听本机。外部系统此版同样位于本机；无需开放公网端口。

## API / Webhook

Python 接入可使用无运行时依赖的 [SDK](sdk/python/README.md)。Runner 首次执行 `scripts/setup-runner.sh`，之后复用安装环境；[GitHub 示例](examples/github/README.md) 已使用 SDK；迁移其他仓库见[完整接入说明](examples/github/GETTING_STARTED.md)（含随附验收工具、模板和可选本机部署），[GitLab 示例](examples/gitlab/README.md) 演示共享项目目录、任务分支、前序产物与结果回收。

首次输入不传 `conversation_id`。Token 绑定平台用户；`user_id` 可省略，普通用户传入时必须与 Token 对应的 ID 相同。会话标识和页面链接不是访问凭据。

```sh
curl http://127.0.0.1:8788/api/invoke \
  -H "Authorization: Bearer $AGENT_PLATFORM_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"<agent-id>","message":"请记住项目代号青禾","request_id":"event-001","workspace_path":"<prepared-project-workspace>"}'
```

`workspace_path` 可选：首次传本机已有项目目录的绝对路径，平台直接在该目录启动 Agent，保留项目 `AGENTS.md` 和前序产物，不复制、不克隆。省略时沿用平台空白／模板工作区。调用方为并发任务准备独立目录；同一目录的会话执行串行。路径在创建时解析为真实绝对目录并固定，接续可以省略；换目录返回 409。目录消失时明确失败，不自动重建空目录。

返回 `conversation_id`、`message_id`、`status`、`conversation_url` 和 `duplicate`。返回成功表示输入已保存，执行可能尚在排队。

接续：向同一入口发送 `conversation_id`、新 `message` 和新的 `request_id`。也可使用 `POST /api/conversations/{id}/messages`。同一用户的同一请求标识与相同输入返回原接收结果；标识相同但内容变化返回 409。调用方在响应丢失时应重试原请求，不能生成另一个标识；网页亦保留未确认请求的标识。

| 接口 | 用途 |
|---|---|
| `POST /api/webhooks/{agent_id}` | 与 invoke 相同的消息格式和用户 Token |
| `GET /api/me` | Token / 登录账号的 User ID |
| `GET /api/conversations` | 当前用户有权限访问的会话 |
| `GET /api/conversations/{id}` | 状态、消息、实际文件 |
| `GET /api/conversations/{id}/events` | 持久事件 SSE，支持游标；format=json 查询数组 |
| `POST /api/conversations/{id}/stop` | 停止当前轮，保留队列 |
| `POST /api/conversations/{id}/continue` | 明确继续保留的队列 |
| `POST /api/conversations/{id}/close` | 关闭，保留记录 |
| `GET /api/conversations/{id}/file?path=report.md` | 查看文件；download=1 下载 |
| `POST /api/users/{user_id}/token` | 管理员生成 / 重置该用户 Token |
| `DELETE /api/users/{user_id}/token` | 管理员撤销 Token |

## 外部工具与 Skill

管理员可在 **外部工具** 页面注册 HTTP 或本机 stdio MCP 服务，发现工具名称、说明和参数 Schema，再在 Agent 上选择具体工具。注册与选择保存在 SQLite；连接配置进入新会话快照，已有会话不被静默改写。Python SDK 同样提供注册和发现方法。

Skill 继续通过 Agent 配置中的目录选择，并由 Codex 原生渐进加载。工具实现留在外部 MCP 服务中；平台不内置 GitHub 交接业务。详见 [注册设计与当前验证边界](docs/02-architecture/external-tools.md)、[独立交接工具示例](examples/pipeline-tool/README.md)。

Agent 每个选中工具可设置“自动审批”（默认）或“每次确认”。确认请求在会话页面显示，批准或拒绝后继续同一原生 Session；配置调整用于新会话。

## 用户与网页入口

每个用户有固定 User ID 和一个可选 API Token。Token 不自动轮换，重置或撤销由管理员在用户页面操作。Agent 配置授权用户，普通用户只能查看和接续自己的会话。管理员可以管理全部记录。

固定 conversation_url 没有临时 Token 或到期时间。未登录时输入用户名密码，成功后自动回到原会话；已登录直接检查权限。链接无需刷新，不泄露 API Token。撤销 API Token 不影响正常网页登录，停用用户则阻止两种访问。

旧账号、会话、文件与原生 Session 保留；升级将旧来源范围转换为用户的 Agent 授权，来源级凭据停止使用。为调用账号生成新 Token 后更新调用系统配置。详见 [访问设计](docs/02-architecture/access.md)、[GitHub 示例](examples/github/README.md) 与 [真实验证](docs/03-delivery/access-verification.md)。

## 联网与运行中提权

管理员在 Agent 配置中分别设置 **允许联网**（`network_access`）和 **权限不足时允许申请管理员审批**（`allow_elevation`），两项默认关闭。联网开关只改变网络权限，不扩大文件写入范围；自动推进业务流程不等于自动批准提权。

启用提权申请后，Codex 原生的命令执行、文件操作和权限申请会显示在对话中。管理员核对原因、完整命令或请求权限后批准或拒绝，同一原生 Session 继续执行。普通调用用户可以看到请求，但不能批准。命令授权仅限本次调用；权限授权仅限本轮，不写入永久放行规则。停止执行、连接中断或平台重启会取消待处理审批。已交接的只读会话不能通过提权取得写权限。

新会话采用创建时的权限。已有会话在空闲时，由管理员打开 **侧栏 → 会话信息 → 应用 Agent 当前联网与提权设置**。该操作仅同步这两项权限，保留原生 Session、历史、模型、指令、工具和工作目录；下一轮生效。对应接口为 `POST /api/conversations/{id}/apply-agent-permissions`，仅管理员可用，执行中返回 409。

仅调整一段会话时，管理员也可调用 `PATCH /api/conversations/{id}/execution-permissions`，明确传入 `network_access` 和 `allow_elevation`。同样仅允许轮次之间更新；不会修改 Agent 默认值，也不会影响其他会话。

## 原生配置和恢复

- 每段会话固定创建时的 Agent 配置、工作目录路径，并持有独立 CODEX_HOME。可直接使用调用方准备的外部工作目录。外部目录不应用模板，Agent 指令使用原生 developer_instructions，仓库 AGENTS.md 保持原样。后续轮次使用保存的原生 thread ID 恢复；所有会话统一通过 app-server 双向协议恢复，逐段推送真实文字增量；也能继续此前由 `codex exec` 创建的原生 Session。
- Skill 由原生发现与渐进读取，平台只控制范围；不会把全文拼入提示词。路径填写 Skill 文件夹或 SKILL.md。未选择的发现项禁用。
- 原生 TOML 可配置 MCP 和 Hook；Hook 必须由操作者显式信任。平台管理的沙箱、原生存储和 Skill 范围不能从高级配置绕过。
- 环境支持继承、字符串覆盖和 null 清除。平台凭据及原生存储变量不透传；已知原生登录／敏感环境凭据在导出的事件中脱敏。不要通过指令要求 Agent 输出凭据，也不要把敏感文件作为模板输入。
- 模板复制成独立工作目录，拒绝符号链接和项目 `.codex` 配置。需要的原生配置应放入 Agent 配置。模板不复制 `.git`、依赖和本地产物。
- 执行中追加消息按序排队，不会自动打断。停止／失败后队列保留，但需明确继续；失败或已停止的原输入不会自动重放。
- 停止先终止整个进程组，必要时三秒后强制结束。进程身份持久记录，平台重启前核验并清理遗留组；身份无法确认则拒绝启动，避免杀错进程或重叠执行。
- 平台重启不会把未知执行伪装成成功；原生记录缺失时明确失败，不创建新的原生会话冒充恢复。
- 关闭后的会话可在页面明确删除历史与文件；删除不可恢复；仅删除平台拥有的记录与文件，不删除外部项目工作目录。默认不自动清理。

SQLite、原生会话、日志与平台工作区保存在 `.data/`；外部工作目录仍在传入的位置。备份／迁移时先停止平台，再复制 `.data/` 和关联的外部工作目录。回合结束等待用户回复时不占执行位置；回合内等待工具确认仍保留原生进程并占执行位置。当前是一个进程、一个数据库，没有外部队列和远程 Worker。

## 验证和边界

开发检查执行 `scripts/verify.sh`，包含 Python SDK 行为测试、Ruff 格式／lint、前端检查、真实浏览器连接生命周期回归及 Go race/vet/build。开发机需要 Python 3.10+、Ruff、Node、Go 和 C 编译器；SDK 使用方不需要这些开发检查工具。

首次运行真实浏览器回归前，执行 `bash examples/github/install-tooling.sh` 安装锁定版本的 Playwright/Chromium。

```sh
./scripts/verify.sh
# 可选真实调用，会消耗当前 Codex 账户用量：
AGENT_PLATFORM_LIVE=1 go test ./internal/platform -run TestLiveNativeSkillHookAndResume -v -timeout 5m
```

验证证据见 `docs/03-delivery/verification.md`；[SDK 与 Runner 的实际验证](docs/03-delivery/sdk-runner-verification.md) 包含正式 GitHub 运行及本机共享目录接续。首版只实现 Codex；Claude Code / DSH 适配、Workflow、多 Agent、共享记忆、组织权限后置。本机执行不是托管不可信代码的多租户安全沙箱。目录隔离解决会话混用，不能宣称隔离任意恶意本机代码。
