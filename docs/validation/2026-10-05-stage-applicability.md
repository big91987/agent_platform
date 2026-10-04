# 阶段适用性与摘要交接验证

## 行为与边界

代码版本：`ccd7dd9`。Workflow 未修改，仍依次交接各阶段。需求或设计没有新增工作时，Agent 在摘要记录理由、依据和有效约束后直接交下一阶段；不再强制生成 PRD、原型、HLD 或附件。新增实质决定仍遵守确认策略，研发/QA 必需证据、权限和迁移审批保留。

提示词由 `examples/github/approval_policy.py` 统一提供给 Runner 与注册 Agent。工具只放开 requirements/design 的空附件，摘要仍存入不可变回执并参与校验、去重；不新增跨阶段跳转。

## 已验证

- 修改前 Go 摘要交接测试在 requirements/design 均报“handoff documents are required”；修改后通过，实际交接函数向测试 HTTP 接收端发送正确相邻阶段，仅首次派发，重复调用复用回执。
- development/qa/未知阶段的空证据仍被拒绝；跨阶段前跳仍被拒绝。
- `go test ./...` 全部通过。
- 正式 `install-tooling.sh` 安装锁定依赖后，GitHub example 的 96 项 Python 回归通过，含真实浏览器能力测试。首次隔离检出未安装依赖时报 playwright/PyYAML 缺失，正式安装后通过，没有改断言。
- Ruff 检查及 `git diff --check` 通过。
- setup 的新安装与 `--stage-policy-only` 升级测试通过，含重复执行幂等、多个仓库隔离与非指令设置不变。
- 本机维护源应用对应源码变更，经过正式工具安装入口重建交接工具；通过 `setup.py --stage-policy-only` 正式 API 升级四个注册 Agent。升级后核对模型、Skill、权限、工具配置均保留，独立 Review 指令及 Runner 配置未改变。

## 未验证与在途任务

本次没有新建真实产品 Issue 来重复触发整条 GitHub/Agent 链；本地回归接收端不等于真实 GitHub 派发验证，也不证明模型必然每次正确判断适用性。需要后续正常任务观察实际决策与交接。

已存在会话的 Agent 快照及已发送输入保持原样，未改数据库、回执或任务检查点。新会话及后续首次进入的阶段读取升级规则；不能宣称当前在途旧会话已自动切换。

源修复以独立 PR 留给用户合并；运行环境已应用该修复，不代表源 main 已包含它。阶段选择入口与额外 Workflow 本次均未增加。
