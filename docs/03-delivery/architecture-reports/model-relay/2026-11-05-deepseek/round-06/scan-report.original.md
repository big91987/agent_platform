# 架构检查

结果：review_required
Git HEAD：ed423e35fcd1aa4f3779bbe5e5c16601a551b712
源码摘要：21389dcd501435bf5eb7b559d1665425f43d13554d09c32015f00b8f7bc6da1a

仅检查 architecture.json 声明的范围；规则通过不代表领域划分合理或产品验收通过。

## 规则扫描

结果：conformant

声明范围内未发现规则违规。

## 模型评审

结果：findings

- module_responsibilities：deviation — 声明的 relay 模块是一整个 Go 包，同时承载网关请求生命周期、控制台身份/租户/应用密钥、计费与账单、SQLite 模式与迁移、备份恢复和网络安全策略。包内 26 个文件没有可检查的边界，改动一个能力（例如费用口径）必然触及与其他能力共享的包和文件；内聚性由文件名而非模块边界保证。
- business_rule_ownership：deviation — 关键业务规则（租户/密钥模型授权、已确认费用与用量的汇总口径）存在多份独立实现，没有单一所有者：授权检查在准入事务与只读授权中各写一套 SQL；费用/成本汇总分别写在 usageFor、账单查询、概览查询和报表 ledgerSum 中，口径变更需要同步修改多处，容易产生不一致。
- dependency_direction：conformant — 在可检查的模块层面依赖方向是单向的：启动包只依赖 relay，relay 不反向依赖启动包，控制台静态资源由 Go 侧 embed 提供，浏览器脚本不导入后端代码。规则判定为 conformant 且无违规；但需要说明，这一结论成立主要因为 relay 内部只有单一包，包内文件级方向不受约束（该问题在 module_responsibilities 与 gateway_upstream_separation 中另行记录）。
- state_ownership：deviation — 路由运行态（routing、latency、health、configEpoch）与请求生命周期状态挂在同一个 Server 对象上，并由同一个互斥锁保护；控制台管理请求在入口处持锁贯穿整段业务处理，而派发选路也需要同一把锁，因此管理侧读取与真实调用派发互相排队。路由运行态还被控制台的 configurationChanged 直接重建，而不是由路由能力自己负责失效。
- testability：conformant — 策略与规则部分有直接可测的接缝：纯函数式计费/取整、usageParts 汇总、outputDelta 判定都有单元测试；选路与延迟可通过构造只含 latency 映射的 Server 与 latencyClock 注入进行验证；准入、配额与结算则在受控 HTTP 上游与真实 SQLite 上以 InProcess/ActualHTTP 双变体验证。静态评审只确认接缝存在，不主张测试已执行或通过。
- necessary_abstractions：conformant — 现有抽象都承担当前职责且规模很小：adminResponse 只为在释放配置锁前缓冲管理 JSON；materialQuery/identityQuery 等单方法接口仅用于复用 *sql.DB/*sql.Tx 的查询接缝；Policy.Resolver 只用于可控解析。没有为了形式分层而建立空层、通用流程引擎或每表一个聚合。
- standards_alignment：deviation — 可执行声明只把代码分成 cmd、internal/relay/*.go 和 web 资源三块，relay 内部任何职责混合都不会产生违规，因此“检查通过”无法表达项目自己承认要求的领域/适配分离；同时规范文档直接声明单包即可（no package map is needed），与所引用的 Go SaaS 目标标准（业务模块必须独立 application/domain 包、HTTP 不得直接引用 SQLite）不一致。项目文档也明确说明通过检查不等于领域设计合格，说明声明本身未覆盖目标要求。
- saas_responsibilities：deviation — 规则密集的控制台能力（配额、密钥生命周期、定价与账单、幂等操作）没有应用用例层与领域层：HTTP 处理函数直接接收 *sql.Tx、内联 SQL、内联校验与状态规则并直接映射响应，存储实现与业务规则和协议适配混在同一函数中。金额取整、准入判断等确有集中函数，但配额/账单/密钥的操作序列没有独立所有者，事务原子性靠处理函数手工组合。
- gateway_request_lifecycle：conformant — 生命周期职责有明确归属且分层清楚：routedGenerate 拥有尝试循环与“仅在提交前临时错误才换上游”的判定，attemptOutput 拥有输出提交与提交前超时，persist/finish 拥有尝试与请求终态落库，observe 拥有健康/冷却观测，stream 拥有流式结束归类。存在重试上界以字面量分散表达（循环上界 4 与 len(tried)==3 两处）的小瑕疵，但不改变职责归属；本项为静态判断，未验证取消、提交与重试的运行时正确性。
- gateway_upstream_separation：deviation — 路由没有与 HTTP 和 SQL 解耦：选路函数直接接收 *http.Request，并在同一函数内开启 SQLite 事务写入选择决策与尝试登记；路由运行态也通过 SQL 读取当前配置。上游适配同样没有独立实现：上游请求构造内联在网关路由文件中，拨号与地址策略在安全模块，HTTP transport 在 Server 组装处，协议解析在另一文件，导致上游网络细节分散在四个位置而非显式上游适配器。
- frontend_features：deviation — 生产控制台把全部十个页面的业务 UI、业务 API 调用、交互状态和清理都放在同一个脚本的单一 renderPage 巨型分支和单一全局 state 对象中；api/table/edit 等共享部分确实是业务中立的，释放控制器与清除敏感输入的清理逻辑也存在，但“功能拥有自己的业务 API、交互状态与清理”只停留在页面字符串层面，没有功能归属，任何页面改动都落在同一函数与同一全局状态上。

### medium · relay 单包同时承载网关、控制台业务、计费、存储迁移与备份，声明层无法约束任何内部边界

principle：模块承担内聚职责并通过小而明确的接口供调用方使用；模块之间的依赖应明确且可检查。
impact：改动任一能力（例如费用口径、密钥轮换、SQLite 模式迁移）都会落在同一包内的共享文件与共享 Server 结构上，评审与回归范围被迫扩散到全包；包内没有可强制的方向约束，依赖质量只能靠人工评审。
recommendation：在保持现有行为的前提下，按能力拆出至少 gateway（请求生命周期与派发）、routing（策略与运行态）、console/accounting（配额、价格、账单）、storage（模式、迁移、备份）等 Go 包，并把 architecture.json 的模块路径与 allowed 依赖同步收紧；不要为拆分引入空层或通用流程引擎。
validation：拆分后运行 make architecture-check，确认新增违规样本（例如 routing 导入 database/sql）能被规则拒绝；同时运行既有 go test -race ./... 与 make verify 的真实 HTTP/浏览器回归。

architecture.json:55-55
```
"internal/relay/*.go"
```

docs/engineering/architecture-checks.md:5-5
```
relay仍是一个集中业务包，控制台仍是原生JavaScript，本轮没有完成领域分包或Vue迁移。
```

### high · 控制台 HTTP 处理函数直接持有事务句柄、内联 SQL 与业务规则，缺应用用例层

principle：For rule-rich SaaS capabilities, HTTP adapts requests to application use cases; independent domain packages own business rules, and infrastructure implements operation-level atomic storage.
impact：配额更新、账单结账与更正、密钥生命周期等操作序列没有单一所有者，校验、事务写入和响应映射交织在同一函数中，难以复用到其他入口（如运营 CLI 或后台任务），也难以在不启动 HTTP 的情况下针对操作序列做规则测试；事务边界依赖每个处理函数的正确书写。
recommendation：按能力引入 application 用例函数（输入校验、权限复核、操作顺序、事务边界）与领域规则（金额、限额、账单状态），存储实现放在基础设施层并实现用例声明的接口；HTTP 仅做协议适配与响应映射。可以先从计费与配额两个最复杂的能力开始，避免一次性重写全部控制台。
validation：为每个用例补独立的操作级测试（含幂等重放、版本冲突、负总额拒绝），并保留现有 ActualHTTP 契约测试作为行为不变性证据；运行 make verify 全量门禁。

internal/relay/console_api.go:90-90
```
func (server *Server) manageLimits(writer http.ResponseWriter, request *http.Request, transaction *sql.Tx, principal Principal, tenantID, role, ownerType, ownerID string) {
```

internal/relay/billing.go:110-110
```
func (server *Server) manageBill(writer http.ResponseWriter, request *http.Request, transaction *sql.Tx, principal Principal, parts []string) {
```

### medium · 模型授权与费用/成本汇总规则存在多份独立实现

principle：Business rules belong to their owning capability rather than HTTP, SQL or duplicated use cases.
impact：同一授权语义（密钥授予 ∩ 租户授予）与同一费用口径（charge_entries 按租户/密钥/周期汇总）分别写死在准入事务、只读授权、账单列表、概览与报表中；任一处口径调整都可能与其他路径不一致，导致账单、概览与限额显示互相矛盾。
recommendation：把授权判定与费用/成本汇总各收敛为单一实现（如 accounting 能力的导出操作），由准入、账单、概览与报表统一调用；只读展示可以保留查询用例，但查询口径不得复制规则。
validation：用现有治理/计量回归测试（包含 reconciliation 与账单 19/60 断言）证明收敛后数值不变，并新增针对汇总口径的单一来源断言。

internal/relay/accounting.go:141-141
```
if transaction.QueryRow("SELECT count(*) FROM key_model_grants k JOIN key_controls c ON c.key_id=k.key_id JOIN tenant_model_grants g ON g.alias=k.alias AND g.tenant_id=c.tenant_id WHERE k.key_id=? AND k.alias=?", record.KeyID, record.Alias).Scan(&currentlyAuthorized) != nil {
```

internal/relay/console_keys.go:421-421
```
err := store.DB.QueryRow("SELECT t.active,(SELECT count(*) FROM key_model_grants k JOIN tenant_model_grants g ON g.alias=k.alias AND g.tenant_id=c.tenant_id WHERE k.key_id=c.key_id AND k.alias=?) FROM key_controls c JOIN tenants t ON t.id=c.tenant_id WHERE c.key_id=?", alias, keyID).Scan(&active, &allowed)
```

internal/relay/billing.go:26-26
```
(SELECT coalesce(sum(c.fee),0) FROM charge_entries c JOIN admissions a ON a.request_id=c.request_id WHERE a.tenant_id=? AND c.period=p.period) AS usage_fee_micro_usd,
```

### medium · 选路函数内联 SQL 与 http.Request，上游网络/协议细节分散在多个文件

principle：The gateway coordinates through explicit interfaces while upstream adapters own network/protocol implementation. Routing owns policy and runtime state without HTTP or SQL.
impact：选路策略无法在不构造 HTTP 请求和数据库事务的情况下验证；上游调用方式（URL 拼装、鉴权头、拨号策略、transport 配置、响应协议校验）分散在 server.go、security.go、routed_proxy.go、proxy.go，替换或新增上游适配需要同时改动多处网关代码。
recommendation：将选路改为接收纯快照数据并返回选择结果，决策与尝试登记的持久化上移到网关的用例步骤；把上游请求构造、鉴权与协议校验收敛到一个显式上游适配器，使网关只通过接口派发。
validation：拆分后用合成快照验证选路与探索/冷却顺序（不依赖 HTTP/SQL），并保留 routing_http_test.go 的真实 HTTP/SSE 断流与提交后不重放断言。

internal/relay/routing.go:171-171
```
func (server *Server) beginAttempt(ctx context.Context, request *http.Request, record *Record, snapshot routeSnapshot, tried map[int64]bool, round int) (*Candidate, *Attempt, string) {
```

internal/relay/routed_proxy.go:116-116
```
outgoing, err := http.NewRequestWithContext(output.child, http.MethodPost, upstream.URL+"/chat/completions", bytes.NewReader(body))
```

### medium · 可执行架构声明与规范声明都退化为“单包即可”，无法表达项目自己引用的目标标准

principle：Project rules and executable architecture declarations agree with the supplied target standards and do not waive requirements merely by describing the current code.
impact：架构门禁对 relay 内部任何职责混合都保持绿色，团队容易把“检查通过”理解为设计合格；而目标标准要求规则密集业务具备 application/domain 包与 HTTP/基础设施外置，两处文档互相矛盾，后续拆分缺乏门禁保护。
recommendation：让架构声明与规范文本对齐：规范文档写明分阶段的目标边界与当前差距，architecture.json 增加可失败的边界（例如禁止 routing 依赖 database/sql 与 net/http），并在拆分完成后同步收紧。
validation：为每条新增规则准备合规与违规样本，确认退出码与定位正确；在 CI 的 architecture 工作流中观察同版结果。

.trellis/spec/backend/index.md:3-3
```
Single Go module; no package map is needed.
```

standards/arch-template/backend/go/saas/README.md:3-3
```
采用本模板后，业务模块必须使用独立的 application 和 domain Go 包；HTTP 与基础设施实现统一外置。
```

architecture.json:62-62
```
"internal/relay/web/**"
```

### low · 管理控制台与派发选路共享同一把 Server 互斥锁，路由运行态由控制台路径重建

principle：Session, request, routing, health and business state have explicit owners; unrelated capabilities do not depend on one shared mutable object.
impact：控制台请求（含只读报表与跨表事务）在整段处理期间持有派发选路所需的同一把锁，管理侧长查询会延迟真实调用的上游选择；路由/延迟/健康运行态的所有权被控制台改变配置时的重置动作侵占，边界不清晰。
recommendation：把配置纪元与运行态失效做成路由能力自己的操作（由配置变更事件触发），并区分“管理读路径”和“派发路径”的锁，避免只读控制台操作与上游选择互斥等待；不要求引入新的通用并发框架。
validation：以真实并发测试观察管理读取期间的派发延迟与一致性，确认失效后旧样本不参与新配置的选择。

internal/relay/console_api.go:29-30
```
server.mu.Lock()
	defer server.mu.Unlock()
```

internal/relay/console.go:24-24
```
server.routing = make(map[string]*routeRuntime)
```

### low · 原生控制台以单一 renderPage 分支与全局 state 承担全部页面功能

principle：Features own business UI, interaction state, business API and cleanup; shared transport is business-neutral, public entry points constrain dependencies.
impact：页面级业务交互状态（筛选、分页、编辑草稿、调用进行中）共享一个全局对象，任何页面改动都要在同一巨型函数中定位，状态越权与相互覆盖的风险随页面增加；共享的 api/table/modal 已经中立，但功能侧没有对应归属。
recommendation：在不引入框架的前提下，按页面/功能拆成独立模块（各自的 render、动作与业务 API 调用），共享的 api 传输、表格渲染与对话框保留为公共入口；功能模块显式注册到导航，避免共享全局可变状态。
validation：拆分后保持现有 workspace 契约与真实 Go 服务浏览器旅程（多视口、键盘、会话过期与敏感输入清理）不变，用 tests/browser 验证行为等价。

internal/relay/web/workspace.js:15-15
```
const state = {user:null,csrf:'',tenant:'',tenants:[],page:'overview',epoch:0,controllers:new Set(),busy:false,call:null,invitation:'',filters:{},pageNumber:1};
```

internal/relay/web/workspace.js:238-238
```
async function renderPage() {
```
- scope_notes：这是一个静态架构评审：未执行 make verify、go test、浏览器旅程或真实供应商调用，所有运行时正确性（事务并发、锁竞争、SSE 提交、限流边界）均未验证，仅依据源码与既有测试文件的结构判断职责归属。
- scope_notes：architecture.json 的规则判定结果（status=conformant、findings 为空）作为声明层面的事实引用，但未重新运行 .harness/archcheck 工具；判定只覆盖三个粗粒度模块之间的依赖边。
- scope_notes：测试文件（*_test.go）在 architecture.json 中被排除，本次仅作为可测试性与职责归属的证据阅读，不视为产品行为证明。
- scope_notes：standards/ 下的 SaaS、模型网关与 Vue 控制台模板是可选采用参考；本仓未采用 Vue，前端评审按 vanilla JS 场景执行。
- scope_notes：internal/relay/web 的 HTML/CSS 仅作为控制台结构上下文，未逐条检查视觉与可访问性细节。

## 检查范围

```json
{
  "scanned": [
    "cmd/model-relay/main.go",
    "internal/relay/accounting.go",
    "internal/relay/admin_response.go",
    "internal/relay/backup.go",
    "internal/relay/billing.go",
    "internal/relay/configuration.go",
    "internal/relay/console.go",
    "internal/relay/console_api.go",
    "internal/relay/console_clock.go",
    "internal/relay/console_keys.go",
    "internal/relay/console_overview.go",
    "internal/relay/console_reports.go",
    "internal/relay/console_storage.go",
    "internal/relay/console_summaries.go",
    "internal/relay/identity.go",
    "internal/relay/legacy_accounting.go",
    "internal/relay/protocol.go",
    "internal/relay/proxy.go",
    "internal/relay/routed_proxy.go",
    "internal/relay/routing.go",
    "internal/relay/routing_latency.go",
    "internal/relay/routing_latency_storage.go",
    "internal/relay/security.go",
    "internal/relay/server.go",
    "internal/relay/store.go",
    "internal/relay/upgrade.go",
    "internal/relay/web.go",
    "internal/relay/web/app.js",
    "internal/relay/web/workspace.js"
  ],
  "excluded": [
    "cmd/model-relay/deployment_test.go",
    "cmd/model-relay/main_test.go",
    "internal/relay/boundary_test.go",
    "internal/relay/configuration_test.go",
    "internal/relay/connection_lifecycle_test.go",
    "internal/relay/console_clock_queue_test.go",
    "internal/relay/console_clock_recovery_test.go",
    "internal/relay/console_contract_test.go",
    "internal/relay/console_http_test.go",
    "internal/relay/console_migration_test.go",
    "internal/relay/console_process_test.go",
    "internal/relay/console_resource_matrix_test.go",
    "internal/relay/console_review_regression_test.go",
    "internal/relay/console_summaries_test.go",
    "internal/relay/console_test.go",
    "internal/relay/console_v3_test.go",
    "internal/relay/crash_test.go",
    "internal/relay/health_test.go",
    "internal/relay/inprocess_test.go",
    "internal/relay/lifecycle_test.go",
    "internal/relay/metering_regression_test.go",
    "internal/relay/persistence_test.go",
    "internal/relay/protocol_role_test.go",
    "internal/relay/qa_connection_contract_test.go",
    "internal/relay/relay_test.go",
    "internal/relay/routing_boundary_test.go",
    "internal/relay/routing_contract_regression_test.go",
    "internal/relay/routing_http_test.go",
    "internal/relay/routing_latency_recovery_test.go",
    "internal/relay/routing_latency_test.go",
    "internal/relay/routing_test.go",
    "internal/relay/single_fixture_test.go",
    "internal/relay/sse_terminal_regression_test.go",
    "internal/relay/upgrade_test.go",
    "internal/relay/version_test.go"
  ],
  "assets": [
    "internal/relay/web/index.html",
    "internal/relay/web/style.css",
    "internal/relay/web/workspace.css",
    "internal/relay/web/workspace.html"
  ],
  "context": [
    ".github/workflows/architecture.yml",
    ".trellis/spec/backend/index.md",
    ".trellis/spec/backend/multitenant-contracts.md",
    ".trellis/spec/backend/routing-contracts.md",
    ".trellis/spec/backend/runtime-contracts.md",
    ".trellis/spec/frontend/index.md",
    "AGENTS.md",
    "Makefile",
    "architecture.json",
    "docs/engineering/architecture-checks.md",
    "docs/roadmap.md",
    "go.mod",
    "go.sum",
    "internal/relay/console_contract_test.go",
    "internal/relay/console_resource_matrix_test.go",
    "internal/relay/metering_regression_test.go",
    "internal/relay/routing_http_test.go",
    "internal/relay/routing_latency_test.go",
    "internal/relay/web/index.html",
    "internal/relay/web/style.css",
    "internal/relay/web/workspace.css",
    "internal/relay/web/workspace.html",
    "package-lock.json",
    "package.json",
    "standards/arch-template/backend/go/README.md",
    "standards/arch-template/backend/go/model-gateway/README.md",
    "standards/arch-template/backend/go/model-gateway/template/docs/model-gateway.md",
    "standards/arch-template/backend/go/saas/README.md",
    "standards/arch-template/backend/go/saas/template/docs/saas.md",
    "standards/arch-template/frontend/vue/admin-console/template/docs/admin-console.md",
    "standards/arch-template/principles.md",
    "standards/arch-template/verification.md"
  ]
}
```
