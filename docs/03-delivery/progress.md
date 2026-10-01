# Implementation progress

2026-10-01: user approved architecture judgment and continuous implementation. PRD scope/defaults accepted for experiment. Architecture and six-task implementation plan saved. Native Codex 0.151.0 and Go 1.25.6 detected.

Ruling: implement directly in the newly created project, no extra worktree; no existing source or shared branch to isolate. Cost: later concurrent development should create worktrees.
Ruling: SQLite with database/sql and CGo driver; local environment has CGo. Cost: builds need C compiler, documented in README.
Ruling: inline implementation; user requested autonomous work, so plan handoff approval is not repeated.

Pre-flight: Store owns durable input and events consumed by Scheduler and HTTP; Executor emits native events consumed by Store and browser. One public conversation ID; native thread stays internal. No conflicting stage or task abstraction.

Tasks 1–3: complete. Persistence, native configuration/execution, scheduling, API, ownership and confined files implemented. Behavior tests observed RED before implementation and GREEN afterward. Full race suite and vet pass.
Task 4: complete. Formal browser login, Agent save/check/try-run, external conversation reply, real progress, PNG display and text preview implemented and inspected. Browser errors introduced during development were repaired in the supported static asset path.
Task 5: complete. Real Webhook -> browser -> API three-turn continuity, constant native thread, four-conversation isolation, actual downloads, Skill reference/Hook execution, stop/continue and SIGKILL/restart all pass. Evidence index: verification.md. Repeatable startup/stop/check scripts and README saved.
Task 6: complete. Fresh-context independent review covered all stated failure classes. Four Important findings fixed: independent kill watchdog; durable supervised process checkpoint/reconciliation before prompt release; active-map removal atomic with completion; unchanged browser retries retain request ID. Focused reproductions and whole race suite pass. No remaining Critical/Important findings from this review.

Final: Ruling: preserve the local experimental branch; this new repository has no remote/base branch to integrate into, and the authorized outcome is a running local experiment — cost: later sharing needs an explicit remote/integration choice.
Final: minor (deferred): Agent message Markdown is displayed as escaped original text; rich rendering is a later usability improvement.
Final: Ruling: non-image files initially provide downloads and small text previews, rather than a universal viewer — removes unsafe execution and unnecessary format dependencies — cost: office/binary previews need a later supported viewer.
Final: Ruling: strong malicious-code isolation, remote execution and multi-Agent workflows remain explicit non-goals — current acceptance concerns trusted local Codex only — cost: this experiment cannot be offered as a multi-tenant cloud service.

2026-10-01 access simplification: user-approved account-owned API tokens, Agent user grants and fixed conversation links replace source credentials and temporary bearer links. Existing accounts and all ten conversations retained; real account login and GitHub Runner path verified. See access.md and access-verification.md.

2026-10-01: 用户批准本机工作目录输入，云端沙箱与仓库初始化后置。首次调用可传 workspace_path，原生 Session 和该目录分别留存；外部项目 AGENTS.md 不改写，阶段文件可直接读取和下载。GitHub 业务接入脚本要求准备好的目录，接续固定原目录。验证结果见 local-workspace-verification.md。

2026-10-01: Python SDK、一次性 Runner 安装脚本及共享目录 GitLab 示例完成。GitHub PR #67 已合入，正式 Runner 使用 SDK 成功接续 Issue #66 的原会话；本机共享目录经过真实 Codex、平台重启、请求去重、事件查询及文件下载验证。回复关联复用已有 parent_id，不增加状态机。4 个 SDK 边界测试及现有开发检查全部通过。真实 GitLab Runner、云端沙箱仍未部署，未将模拟 CI 当成云端验收。详见 sdk-runner-verification.md。
