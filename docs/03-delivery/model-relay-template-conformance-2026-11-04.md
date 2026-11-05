# Model Relay 与架构 Template 的符合度核查

本文件是2026-11-04的源码对照分析，不是扫描工具生成的报告。2026-11-05实际运行archcheck后的[标准JSON、原始模型结果及执行记录](architecture-reports/model-relay/2026-11-05/README.md)单独保留；工具结果仍为incomplete，含3项引用已校验、待审查的发现。

核查结论：**Model Relay 尚未符合约定的架构 Template。此前的“29 个文件、0 条违规”只证明它满足当前仓库自行声明的粗粒度依赖规则，不能作为 Template 符合度结论。** 当前实现有真实的业务规则、事务和行为测试，但网关、管理业务、存储及前端的职责隔离尚未落实。

## 依据与范围

本次以维护源 `53a2f21f49326ff1cdf24c6e68b562aa8bee9d2b` 的通用原则、Go 模型网关、Go SaaS 和 Vue 管理控制台规范为依据，核对 Model Relay 候选 `d03fe5b6843ea6a274a43411a92dad237b902313`。候选相对主线基线 `521751f1a0afaca26bd50b989f6f0df2d09f10e5` 没有修改 `cmd/`、`internal/` 产品代码，因此本表同样反映该主线基线的产品结构。

检查包含全量生产源码路径、包和导入事实，以及请求派发、路由观测、配额准入、账单、控制台和相关测试的关键代码核查。此次没有执行完整产品行为回归，也没有通过外部模型服务运行工具的 `review` 命令；下表中的职责判断来自本次源码审查。

适用标准：

- [通用代码与架构原则](../../examples/arch-template/principles.md)：模块内聚、规则与技术实现分离、依赖倒置、规范归档。
- [Go 模型网关](../../examples/arch-template/backend/go/model-gateway/README.md)：请求流程、路由状态、HTTP 适配和上游网络实现分工。
- [Go SaaS](../../examples/arch-template/backend/go/saas/template/docs/saas.md)：业务模块内 Application/Domain 独立包、原子存储接口、外部 SQL 实现。
- [Vue 管理控制台](../../examples/arch-template/frontend/vue/admin-console/template/docs/admin-console.md)：按 feature 组织业务、局部状态和业务 API，共享传输不承载业务。

Model Relay 包含网关和租户、权限、配额、账务管理，两个后端场景均适用。路由算法不强制采用四层。前端目前为原生 JavaScript，Vue 样板尚未采用；其 feature、状态及传输分工原则仍可核查，不将“没有 Vue”本身等同于架构缺陷。

## 符合度表

| 检查项 | Template 要求 | Model Relay 实际情况与证据 | 结论 |
| --- | --- | --- | --- |
| 规范归档与导航 | 根 AGENTS.md 引用项目规范；明确所选场景、模块职责和依赖 | 已有 `.trellis/spec/backend`、`frontend` 及业务契约；根 AGENTS.md 未直接导航到这些规范及所选 Template。backend/index.md 仍明确写着“no package map is needed”，将 HTTP、存储、代理统一归到 relay | 部分符合：有知识基础，架构约束尚未对齐 |
| 网关的模块划分 | 请求流程、路由、协议适配和上游实现分别承担职责，以包边界限制依赖 | 26 个后端生产文件均在同一个 `internal/relay` 包，另有一个 cmd 入口；HTTP、SQL、策略、账务可以直接访问同包内部成员。文件名分组尚未形成模块隔离 | 不符合 |
| 路由策略隔离 | 路由拥有选择及运行状态，不处理 HTTP 或 SQL | `routing.go:143` 的 snapshot 直接读取 Store 和 SQL；`:171` 的 beginAttempt 接收 `*http.Request`，处理鉴权、数据库、候选选择和尝试记录。`routing_latency.go:220` 的 acceptLatency 同时查询配置、判断样本和写入 SQL | 不符合 |
| 请求流程与网络实现分工 | 流程通过明确接口使用上游能力；HTTP/网络细节放在适配实现中 | `routed_proxy.go:13` 同时选择目标、解密凭据、组装并发送 HTTP、处理响应、流式提交及持久化结果；`:141` 直接调用 server.client.Do。已有流程代码，但没有隔离网络实现的接口 | 不符合 |
| SaaS 用例与业务规则分层 | 配额、身份、账务按业务职责组织；Application/Domain 为独立包，规则不依赖 HTTP/SQL | `accounting.go:127` 的 admit 同时读取 HTTP Authorization/Idempotency-Key、校验身份和额度、选价格、执行 SQL 并返回 HTTP 状态。`billing.go:15` 的 tenantBills 同时处理 HTTP、账单业务和事务 | 不符合 |
| 事务和存储接口 | 保留完整原子操作；调用方通过业务接口使用存储，不接触数据库句柄或事务 | admit 已在一个事务内完成准入及价格快照，recoverConsole 也保留原子恢复；但 Store.DB 为导出字段，consoleAPI 直接开启事务，tenantBills 等接收 `*sql.Tx`，尚无业务存储接口 | 部分符合：有原子性实现，存储隔离不符合 |
| 运行状态归属 | 路由、身份会话和请求执行的状态各归负责模块，不依赖一个共享对象访问所有状态 | `server.go:34` 的 Server 同时持有 Store、HTTP client、会话、登录尝试、路由、健康、延迟和配置版本。26 文件中有 43 个 Server 方法、16 个 Store 方法，共享内部实现 | 不符合 |
| 前端 feature 和 API 分工 | 页面/功能拥有局部状态和业务 API，通过共享传输访问后端；跨功能通过公开入口协作 | `workspace.js:3` 定义十类页面，`:15` 共享 state，`:66` 内置传输，`:238` 统一渲染业务，`:264` 处理调用测试；两个 JS 文件均无 import/export 模块接口 | 不符合；Vue 样板未采用 |
| 时间、取消和输出提交的明确性 | 请求与尝试有时间预算；取消传递，输出提交后停止重放 | proxy.go、routed_proxy.go 中已有 context、独立尝试及提交状态；相关测试覆盖超时/取消、提交后不重放、迟到样本等场景。实现与测试存在，但仍耦合于 Server；此次未重跑这些行为测试 | 部分符合：机制已有，结构及本次行为验收不完整 |
| 模块可独立测试 | 业务和策略通过模块接口验证，HTTP/SQL 由相应集成测试验证 | 已有 microAmount、overLimit、latencyPool 等局部计算，以及真实 HTTP 测试。chooseLatency 的测试仍构造 Server；关键路由及准入测试依赖整体 fixture/数据库，缺少独立包接口 | 部分符合 |
| 固定工程验证入口 | 锁定依赖，固定入口覆盖格式、vet、race、构建及适用的行为验证 | go.mod、lockfile 和 Makefile 已提供上述入口；架构命令也已接入 verify-gates 与 CI | 入口已具备；此次没有执行完整 make verify |
| 硬规则与 Template 一致 | 架构规则表达所选场景的职责及允许依赖，识别实际偏离 | 当前 architecture.json 只声明 startup、整个 relay、整个 console；没有 routing/gateway/业务分层约束，也未限制 HTTP/SQL 导入。规则分析不评估同包内的职责和调用 | 不符合 Template 检查目标 |

文件数、方法数用于说明实际归属，不作为质量评分。认定职责混杂的依据是具体函数同时处理协议、业务判断、存储和共享状态。

## 关键证据

以下链接固定到被检查的候选版本，便于独立复核：

- [当前 architecture.json](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/architecture.json)：将整个 relay 作为一个模块。
- [backend/index.md](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/.trellis/spec/backend/index.md#L3)：现有规范明确保留集中包。
- [Server 的状态集合](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/server.go#L34)。
- [路由读取配置和数据库](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/routing.go#L143)，[路由尝试入口依赖 HTTP](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/routing.go#L171)。
- [延迟样本处理及 SQL 写入](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/routing_latency.go#L220)。
- [准入同时消费 HTTP 请求和数据库](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/accounting.go#L127)，[控制台直接开启 SQL 事务](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/console_api.go#L12)。
- [账单 HTTP/业务/SQL 混合入口](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/billing.go#L15)。
- [派发流程](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/routed_proxy.go#L13)，[直接网络调用](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/routed_proxy.go#L141)。
- [控制台共享状态](https://github.com/big91987/model-relay/blob/d03fe5b6843ea6a274a43411a92dad237b902313/internal/relay/web/workspace.js#L15)。

## 检查实现的缺口

当前引擎按照 architecture.json 判断包和导入关系，能够发现“已分开的模块之间违反依赖规则”，却不会判断调用方把整个产品声明为一个模块是否合理。此次全部 26 个 relay 文件归同一个 Go 包，JS 也没有导入边，当前规则不会报出这些职责问题。把同包文件强行声明成几个模块也无效：工具明确拒绝一个 Go 包归属多个模块，不能伪造隔离。

项目接入还缺少两项工作：

1. **项目与 Template 的适配契约。** 明确本项目采用网关和 SaaS 的哪些职责、前端采用哪些规范；把现有 `.trellis` 约定同步到同一架构要求。业务差异须明确记录，不以当前目录结构反过来降低要求。
2. **实际的职责评审。** 当前集成只执行确定规则，`documents` 也未收录所选 Template 规范和关键测试；工具提供的语义评审能力尚未用于这个仓库。仅将相同宽松规则搬到沙箱执行，仍无法回答符合度问题。

这两项属于 Harness 接入与评审流程缺口；前表中的混合职责属于 Model Relay 代码结构偏离。二者分别处理。

## 本次执行记录与结论边界

本地重新执行已安装工具的标准入口 `python3 -I .harness/archcheck/inspect_project.py`，退出 0。扫描 27 个 Go 文件、2 个 JavaScript 文件；排除 35 个 Go 测试，另列 4 个资源文件。源码摘要为 `8868a1cded1f8c735efb31d646883b565393b78787ce199a52b8fec41f7e3ba4`，规则结果仍为 conformant，模型评审状态为 not_run。这复核了当前规则覆盖的局限，未改变前表的 Template 不符合结论。

112 的工具安装和 Docker 权限故障另属工作流运行验证；本次符合度核查可以依据固定源码完成，不以修复服务器权限作为前置条件。此次未修改 Model Relay 产品代码、未执行架构迁移、未合并或部署。
