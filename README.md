# Agent Platform

本机通用 Agent 平台实验版。配置一个原生 Codex，通过 API、Webhook 或网页创建会话，随后在同一会话继续交流、观察执行和查看文件。研发阶段和产出形式由 Agent 配置与用户输入决定。

## 启动与使用

需要 Go 1.25+、C 编译器、Python 3，以及已安装并完成 `codex login` 的 Codex CLI。当前实际验证版本是 Codex 0.151.0。Node 用于前端检查，不是服务运行依赖。

```sh
./scripts/start.sh
# 打开 http://127.0.0.1:8788
# 首次启动管理员账号 / 密码：admin / admin；账号密码存于数据库，重启不会重置。
```

1. 在 **智能体** 中选择本机 Codex，配置指令、模型与允许使用的 Skill，执行“检查”，然后试运行。
2. 在 **会话** 中发起请求。进展、最终回复和工具日志来自真实原生执行器；图片直接显示，文本可查看，所有产物均可下载。
3. 在 **调用方接入** 创建调用凭据并选择可调用的 Agent。调用系统保存返回的会话标识；操作者可打开返回页面接续。

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

首次输入不传 `conversation_id`。`user_id` 是调用系统内的用户标识；不同调用凭据形成不同来源。会话标识和页面链接不是访问凭据。

```sh
curl http://127.0.0.1:8788/api/invoke \
  -H "Authorization: Bearer $AGENT_PLATFORM_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"<agent-id>","user_id":"user-001","message":"请记住项目代号青禾","request_id":"event-001"}'
```

返回 `conversation_id`、`message_id`、`status`、`conversation_url` 和 `duplicate`。返回成功表示输入已保存，执行可能尚在排队。

接续：向同一入口发送 `conversation_id`、同一 `user_id`、新 `message` 和新的 `request_id`。也可使用 `POST /api/conversations/{id}/messages`。相同来源的同一请求标识与相同输入返回原接收结果；标识相同但内容变化返回 409。调用方在响应丢失时应重试原请求，不能生成另一个标识；网页亦保留未确认请求的标识。

| 接口 | 用途 |
|---|---|
| `POST /api/conversations/{id}/web-access` | 签发指定会话临时网页入口；body: user_id、expires_in（默认 3600） |
| `POST /api/webhooks/{agent_id}` | 通用 JSON Webhook；同一消息格式与 Bearer 授权 |
| `GET /api/conversations?user_id=…` | 当前调用来源与用户的会话 |
| `GET /api/conversations/{id}?user_id=…` | 当前状态、消息与文件 |
| `GET /api/conversations/{id}/events?user_id=…` | 持久事件 SSE；支持 Last-Event-ID / after 恢复游标 |
| `GET /api/conversations/{id}/events?user_id=…&format=json` | 分批查询原始事件 |
| `POST /api/conversations/{id}/stop?user_id=…` | 停止当前轮并保留队列 |
| `POST /api/conversations/{id}/continue?user_id=…` | 明确继续已保留队列 |
| `POST /api/conversations/{id}/close?user_id=…` | 关闭，保留历史与文件 |
| `GET /api/conversations/{id}/file?user_id=…&path=report.md` | 查看或下载真实文件；download=1 强制下载 |

外部凭据不允许修改 Agent 配置或读取其他来源／用户会话。管理员可处理所有本机会话；调用方账号绑定来源和 user_id，临时网页入口仅授权一个会话。GitHub requirements Runner 示例见 [接入说明](examples/github/README.md)，已验证真实 Issue 链接到平台澄清；钉钉和企微等其他渠道可转换成同一接口。

## 用户与角色

管理导航包含智能体、会话、调用方接入、用户与角色、API 文档。Agent 卡片的“会话”入口只显示该 Agent 的已有 Session；会话回复恢复原生上下文。API 文档有字段表和实际请求调试。

管理员创建调用方账号并绑定来源与 user_id；临时访问用户打开 Runner 签发的链接即可对话，不能进入配置管理或其他会话。详见 [访问设计](docs/02-architecture/access.md) 与 [真实验证](docs/03-delivery/access-verification.md)。

## 原生配置和恢复

- 每段会话固定创建时的 Agent 配置，并持有独立工作目录与 CODEX_HOME。后续轮次使用保存的原生 thread ID 调用 `codex exec resume`。
- Skill 由原生发现与渐进读取，平台只控制范围；不会把全文拼入提示词。路径填写 Skill 文件夹或 SKILL.md。未选择的发现项禁用。
- 原生 TOML 可配置 MCP 和 Hook；Hook 必须由操作者显式信任。平台管理的沙箱、原生存储和 Skill 范围不能从高级配置绕过。
- 环境支持继承、字符串覆盖和 null 清除。平台凭据及原生存储变量不透传；已知原生登录／敏感环境凭据在导出的事件中脱敏。不要通过指令要求 Agent 输出凭据，也不要把敏感文件作为模板输入。
- 模板复制成独立工作目录，拒绝符号链接和项目 `.codex` 配置。需要的原生配置应放入 Agent 配置。模板不复制 `.git`、依赖和本地产物。
- 执行中追加消息按序排队，不会自动打断。停止／失败后队列保留，但需明确继续；失败或已停止的原输入不会自动重放。
- 停止先终止整个进程组，必要时三秒后强制结束。进程身份持久记录，平台重启前核验并清理遗留组；身份无法确认则拒绝启动，避免杀错进程或重叠执行。
- 平台重启不会把未知执行伪装成成功；原生记录缺失时明确失败，不创建新的原生会话冒充恢复。
- 关闭后的会话可在页面明确删除历史与文件；删除不可恢复。默认不自动清理。

SQLite、原生会话、日志与文件均保存在 `.data/`。备份／迁移时先停止平台，再整体复制此目录。运行等待用户时不占执行位置。当前是一个进程、一个数据库，没有外部队列和远程 Worker。

## 验证和边界

```sh
./scripts/verify.sh
# 可选真实调用，会消耗当前 Codex 账户用量：
AGENT_PLATFORM_LIVE=1 go test ./internal/platform -run TestLiveNativeSkillHookAndResume -v -timeout 5m
```

验证证据见 `docs/03-delivery/verification.md`。首版只实现 Codex；Claude Code / DSH 适配、Workflow、多 Agent、共享记忆、组织权限后置。本机执行不是托管不可信代码的多租户安全沙箱。目录隔离解决会话混用，不能宣称隔离任意恶意本机代码。
