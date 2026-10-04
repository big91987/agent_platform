# 平台内 Workflow 验证记录

基线：2026-10-05，分支 codex/platform-workflows，起点 3f3b2c8。

当前结果：规划基线、T1 后端和编辑器切片已形成；尚未实现或验证完整平台内工作流。不得引用原 GitHub CI 方案的测试结果证明本方案通过。

| 验证范围 | 状态 | 证据 |
|---|---|---|
| 图编辑、保存、授权、版本快照（WF-01/09/10） | 部分通过 | 定义 API、权限与网页编辑通过；运行快照待 T2 |
| 真实 Agent 接力、人工确认与回退（WF-02～05/12） | 未测 | 待 T2 |
| GitHub Connector 与新仓库闭环（WF-06） | 未测 | 待 T3 |
| 停止、重复交接、重启及升级（WF-07～11） | 未测 | 待 T4 |

隔离原则：独立 worktree、独立数据目录和服务端口；不更改旧平台现场；测试仓库单独创建。敏感运行数据和截图保留本地忽略目录，公共证据只记录脱敏步骤、版本、结果和外部测试资源链接。

## T1 定义持久化切片

- 真实组件：Go HTTP handler、现有登录与用户 Token、SQLite；没有模拟存储或授权。
- 先运行新增 `TestWorkflow*`：全部因缺失入口返回 405 而失败。
- 实现后 `go test ./internal/platform -run '^TestWorkflow' -count=1` 通过。覆盖带退出路径的反馈循环、歧义路由／无出口／失效节点拒绝、版本冲突及跨用户读取限制。
- `go test ./... -timeout 120s` 通过（平台、pipeline-tool）；`go vet ./...` 通过；`git diff --check` 通过。
- 范围限制：本轮测试仅证明图定义和 HTTP 访问行为。Connector 执行、运行状态机、Agent 交接、UI、安装与重启恢复均未通过此结果证明。
- `go test -race ./internal/platform -timeout 120s` 通过；macOS 链接器输出 `LC_DYSYMTAB` 警告，未导致链接或测试失败。新增文档本地链接核验通过。

## T1 可视化编辑器与真实网页验证

日期：2026-10-05。隔离实例启动命令：`./bin/agent-platform -listen 127.0.0.1:8792 -base-url http://127.0.0.1:8792 -data .data/workflow-demo`。未变更原 8788 实例。

真实网页操作创建 `草稿审核 · 编排验收`：

1. 管理员登录，在工作流页新建空白图。
2. 从节点面板拖入 Agent，配置现有 Codex；加入人工确认和结束节点。
3. 通过端口建立 next／approved／changes 路由，changes 回到 Agent；包含真实鼠标端口拖拽，不只修改 JSON。
4. 保存后取得 `456bd401841d315a4f3d6f75fae12b77`；刷新后 3 个节点、3 条连线保留。
5. 拖动 Agent 从 (77,47) 到 (141,78)，再次保存和刷新，坐标保持；之后重建回退路径并保存到版本 3。
6. 导出面板展示完整 JSON（无源 ID、版本和用户授权）；从正式导入入口粘贴，独立保存副本 `782c122df45559f911d656d4b9d7540e`，版本 1，原定义保留版本 3。
7. 隔离服务重启后原定义仍可打开。截图在本地忽略目录 `.data/workflow-evidence/editor.jpg`。

修复：页面资源回归测试发现新 JS 未被 HTTP 静态白名单提供。移除重复名单，按明确的 embed 资源集提供公共静态文件；全套 Go 回归恢复通过。点击连续添加重叠节点问题先补失败行为测试，再实现避让并通过。

验证：15 项 Node 行为测试通过；`go test ./... -timeout 120s`、`go vet ./...`、`git diff --check` 通过。浏览器未记录页面控制台错误。

限制：上述流程只配置图，未启动真实 Agent Run。Connector 在页面标明尚未开放。文件下载事件等待超时，尚未取得下载文件，不能宣称文件导入／导出已验收；已验证的是正式 JSON 文本往返。运行版本快照、审批／回退执行、停止和未知外部效果恢复留待 T2～T4。

## 独立测试仓库

已通过 GitHub 正式入口创建并查询核实 [agent-platform-workflow-demo](https://github.com/big91987/agent-platform-workflow-demo)，可见性 PRIVATE，默认分支 main。当前仅初始化 README；尚未应用真实平台流程。后续 T3 使用该仓库，禁止把 reading_list 的旧成功记录算作新方案验证。
