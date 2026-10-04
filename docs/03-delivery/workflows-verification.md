# 平台内 Workflow 验证记录

基线：2026-10-05，分支 codex/platform-workflows，起点 3f3b2c8。

当前结果：规划基线与 T1 后端定义切片已形成；尚未实现或验证完整平台内工作流。不得引用原 GitHub CI 方案的测试结果证明本方案通过。

| 验证范围 | 状态 | 证据 |
|---|---|---|
| 图编辑、保存、授权、版本快照（WF-01/09/10） | 部分通过 | 定义 API 和权限测试通过，网页／运行快照待实现 |
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
