# 代码与架构规范检查

- 规则扫描：未发现规则违规（仅限已声明的规则）。
- 模型评审：发现 5 项待复核的架构问题。

规则检查依据已声明的依赖约束；模型意见需复核。本报告不判定产品业务功能通过或失败。

## 规则违规

声明范围内未发现规则违规。

## 模型架构发现（待复核）

### 1. internal/relay is one package that mixes HTTP adapters, SaaS rules and SQLite persistence（高）

**涉及位置与源码对象**

- `architecture.json:53-57`
- `internal/relay/billing.go:15` · `tenantBills`
- `internal/relay/accounting.go:127` · `admit`
- `internal/relay/console_api.go:90` · `manageLimits`
- `standards/arch-template/backend/go/saas/README.md:16`

**问题依据**

module_responsibilities, business_rule_ownership, dependency_direction and saas_responsibilities; the Go SaaS standard requires independent application/domain packages with external HTTP and infrastructure, and the principles require package boundaries where strict isolation is needed.

**影响**

Quota, key, billing and tenant rules are implemented inside net/http handlers that receive *sql.Tx; changing a rule or a table forces edits in the same function and prevents narrow reuse or unit-level verification. Transaction boundaries, authorization and response mapping are coupled, so unrelated console capabilities share one package and one change surface.

**调整建议**

Keep cmd/model-relay as assembly. Introduce packages for the rule-rich console capabilities named by the standard (for example identity, access, catalog and accounting) with application and domain packages; move SQL into infrastructure implementations of interfaces declared by those modules; restrict net/http to an httpapi adapter that calls application operations. Move methods such as Store.admit, Server.manageLimits and Server.tenantBills into the owning application operation and a persistence interface. Update architecture.json to declare and enforce those boundaries.

**复查建议**

Re-run make architecture-check after adding rules that forbid httpapi -> domain/SQLite and domain -> HTTP/SQL/application. Preserve behavior with the existing console_contract_test.go, console_http_test.go and metering_regression_test.go suites. This is an import/structure recheck, not a runtime claim.

**原文证据**

`architecture.json:53-57`

    "name": "relay",
          "paths": [
            "internal/relay/*.go"
          ],
          "allow": []

`internal/relay/billing.go:15` · `tenantBills`

    func (server *Server) tenantBills(writer http.ResponseWriter, request *http.Request, transaction *sql.Tx, principal Principal, tenantID, role string) {

`internal/relay/accounting.go:127` · `admit`

    func (store *Store) admit(record Record, snapshot routeSnapshot, payload map[string]json.RawMessage, request *http.Request) (string, int, string) {

`internal/relay/console_api.go:90` · `manageLimits`

    func (server *Server) manageLimits(writer http.ResponseWriter, request *http.Request, transaction *sql.Tx, principal Principal, tenantID, role, ownerType, ownerID string) {

`standards/arch-template/backend/go/saas/README.md:16`

    目录与依赖是强制约定。Application 调用本模块 Domain，并定义所需存储接口；HTTP 只能调用 Application，不能直接引用 Domain 或 SQLite。Domain 不得引用 Application、其他模块或外部库，也不得引用标准库中的 HTTP/SQL。

### 2. Routing policy and runtime state depend on HTTP requests and SQLite writes（高）

**涉及位置与源码对象**

- `internal/relay/routing.go:171` · `beginAttempt`
- `internal/relay/routing.go:294` · `beginAttempt`
- `internal/relay/routing_latency.go:264` · `acceptLatency`
- `standards/arch-template/backend/go/model-gateway/README.md:18`

**问题依据**

gateway_upstream_separation and dependency_direction; the Go model-gateway standard says routing provides selection and observation operations and owns weight/sample/cooldown state without processing HTTP responses or SQL.

**影响**

beginAttempt both selects a route and opens/persists a SQLite transaction while accepting an *http.Request; acceptLatency writes route_latency_evidence. Routing algorithms and health/latency state cannot be exercised or changed without the HTTP layer and schema, and management/schema changes can touch selection code.

**调整建议**

Extract routing policy and runtime state (weighted counters, cooldown, latency pools and selection observations) into a routing package whose API accepts candidate snapshots and observation events, not *http.Request or *sql.Tx. Keep orchestration in routed_proxy.go/gateway; persist route_selection_evidence and route_latency_evidence through an evidence-store interface implemented outside routing.

**复查建议**

Check that the routing package imports no net/http or database/sql; preserve behavior with routing_latency_test.go, routing_http_test.go and routing_contract_regression_test.go. No runtime correctness is inferred.

**原文证据**

`internal/relay/routing.go:171` · `beginAttempt`

    func (server *Server) beginAttempt(ctx context.Context, request *http.Request, record *Record, snapshot routeSnapshot, tried map[int64]bool, round int) (*Candidate, *Attempt, string) {

`internal/relay/routing.go:294` · `beginAttempt`

    if _, err = transaction.ExecContext(cleanup, "INSERT INTO request_route_decisions VALUES(?,?,?,?,?,?,?,?,?)", record.ID, round, decision.UpstreamID, decision.Enabled, decision.Priority, decision.Weight, decision.Reason, decision.Observed, decision.Cooldown); err != nil {

`internal/relay/routing_latency.go:264` · `acceptLatency`

    _, err = server.Store.DB.Exec("INSERT INTO route_latency_evidence VALUES(?,?,?) ON CONFLICT(request_id,ordinal) DO UPDATE SET evidence=excluded.evidence", id, attempt.Ordinal, string(raw))

`standards/arch-template/backend/go/model-gateway/README.md:18`

    routing 对外提供选择和观测操作，内部拥有权重计数、评分样本和冷却状态；不处理 HTTP 响应或 SQL。

### 3. One Server object and one mutex own unrelated console, routing, health, latency and session state（中）

**涉及位置与源码对象**

- `internal/relay/server.go:37-52` · `Server`
- `internal/relay/console_api.go:29-31` · `consoleAPI`
- `internal/relay/routing.go:172-174` · `beginAttempt`

**问题依据**

state_ownership and module_responsibilities; unrelated capabilities should not depend on one shared mutable object, and session, request, routing, health and business state need explicit owners.

**影响**

Console handlers, routing dispatch, upstream checks, latency evidence and success evidence all mutate fields of Server under the same broad lock. Slow management or storage work can serialize unrelated dispatch and console paths, and state lifetime/cleanup is hidden inside one struct.

**调整建议**

Keep Server as the HTTP/composition entry point. Move routing/health/latency maps into the existing routeRuntime/routeHealth/latencyPool owners behind a routing-state interface, and give the checking registry and session/auth state their own owners and lock scopes. Avoid holding the console-wide mutex while beginning a storage transaction.

**复查建议**

Recheck field ownership and lock use in server.go, routing.go, routing_latency.go and console_api.go; preserve existing routing and console tests as behavior safeguards. This remains a static concurrency-boundary review.

**原文证据**

`internal/relay/server.go:37-52` · `Server`

    	mu                 sync.RWMutex
    	authMu             sync.Mutex
    	sessions           map[string]session
    	attempts           []time.Time
    	invitationAttempts []time.Time
    	passwordSlots      chan struct{}
    	degraded           atomic.Bool
    	client             *http.Client
    	configEpoch        uint64
    	successID          string
    	routing            map[string]*routeRuntime
    	checking           map[int64]bool
    	health             map[string]*routeHealth
    	dispatchSequence   int64
    	latency            map[string]*latencyPool
    	latencyClock       func() time.Time

`internal/relay/console_api.go:29-31` · `consoleAPI`

    server.mu.Lock()
    	defer server.mu.Unlock()
    	if !server.advanceClock(writer) {

`internal/relay/routing.go:172-174` · `beginAttempt`

    server.mu.Lock()
    	defer server.mu.Unlock()
    	if server.Store.advanceClock() != nil {

### 4. workspace.js is one global feature bundle with no feature-owned UI, state or API（中）

**涉及位置与源码对象**

- `internal/relay/web/workspace.js:15`
- `internal/relay/web/workspace.js:238` · `renderPage`
- `internal/relay/web/workspace.js:253` · `renderPage`
- `architecture.json:60-64`
- `standards/arch-template/frontend/vue/admin-console/template/docs/admin-console.md:9`

**问题依据**

frontend_features, module_responsibilities and state_ownership; features own business UI, interaction state, business API and cleanup, shared transport stays business-neutral, and public entry points constrain dependencies.

**影响**

All business pages share one state object, one renderPage switch and shared edit/table/detail helpers. Changes to one page can affect unrelated pages, cleanup is global, and there is no module boundary that exposes or restricts each feature's API.

**调整建议**

Split workspace.js into ES feature modules for models, services, keys, tenants, limits, usage, billing, records and playground; each feature owns its render/action state and cleanup. Keep shared/api business-neutral and keep table, modal and formatting helpers in shared modules. The page router should import feature entry points rather than one global renderPage/monolith.

**复查建议**

Recheck the static import graph and public feature entry points; preserve behavior with the existing Node/browser tests and workspace HTML/CSS asset loading. No new business rules are proposed.

**原文证据**

`internal/relay/web/workspace.js:15`

    const state = {user:null,csrf:'',tenant:'',tenants:[],page:'overview',epoch:0,controllers:new Set(),busy:false,call:null,invitation:'',filters:{},pageNumber:1};

`internal/relay/web/workspace.js:238` · `renderPage`

    async function renderPage() {

`internal/relay/web/workspace.js:253` · `renderPage`

    if(page==='models') {const catalog=await collection(scoped(state.user.platform?'catalog?include_ungranted=true':'catalog')),values=state.user.platform?await collection('models'):catalog;

`architecture.json:60-64`

    "name": "console",
          "paths": [
            "internal/relay/web/**"
          ],
          "allow": []

`standards/arch-template/frontend/vue/admin-console/template/docs/admin-console.md:9`

    src/app 负责组装；features/quotas 拥有页面、局部状态和业务 API；shared/api 不含业务规则。新功能独立建 feature，有跨功能协作需求再定义公开接口，不建立空的全局 store 或所有业务共用的大 service。

### 5. The executable architecture declaration only models one relay package and one console module（中）

**涉及位置与源码对象**

- `architecture.json:53-57`
- `.trellis/spec/backend/index.md:3`
- `docs/engineering/architecture-checks.md:5`
- `standards/arch-template/backend/go/saas/README.md:16`
- `standards/arch-template/principles.md:10`

**问题依据**

standards_alignment; project rules and executable declarations must agree with supplied target standards and must not waive requirements merely by describing the current code.

**影响**

The architecture check can pass because it only enforces module-level path membership; it cannot detect that rule-rich SaaS capabilities lack application/domain boundaries or that routing depends on HTTP/SQL. The declared model therefore gives a false sense of conformance to the supplied standards.

**调整建议**

Update architecture.json and the backend spec to declare startup, httpapi, application/domain modules and infrastructure implementations for the rule-rich console capabilities, plus the routing/gateway split. Add enforceable import rules; keep the current honest limitation text until the packages exist, but do not treat the single relay module as satisfying the SaaS/gateway standards.

**复查建议**

Re-run make architecture-check with compliant and violating import samples for the new module declaration; preserve existing behavior tests as safeguards. This is a declaration/CI gate review, not a runtime validation.

**原文证据**

`architecture.json:53-57`

    "name": "relay",
          "paths": [
            "internal/relay/*.go"
          ],
          "allow": []

`.trellis/spec/backend/index.md:3`

    Single Go module; no package map is needed. Runtime ownership is cmd/model-relay (CLI) and internal/relay (HTTP, persistence, proxy).

`docs/engineering/architecture-checks.md:5`

    relay仍是一个集中业务包，控制台仍是原生JavaScript，本轮没有完成领域分包或Vue迁移。检查通过只证明已声明的依赖规则满足，不能作为领域设计或独立QA通过。

`standards/arch-template/backend/go/saas/README.md:16`

    目录与依赖是强制约定。Application 调用本模块 Domain，并定义所需存储接口；HTTP 只能调用 Application，不能直接引用 Domain 或 SQLite。Domain 不得引用 Application、其他模块或外部库，也不得引用标准库中的 HTTP/SQL。

`standards/arch-template/principles.md:10`

    模块之间的依赖应明确且可检查。文件划分不形成编译器约束，需要严格隔离时使用包或模块边界。


## 逐项评审与限制

- `module_responsibilities`：存在偏差。The Go production code is one internal/relay package containing HTTP adapters, SaaS rules, persistence, routing, security and operational code; file names do not create compiler isolation. The native console is also one global script for all business pages.
- `business_rule_ownership`：存在偏差。Quota, key, billing, tenant and routing rules sit inside HTTP handlers or Store/Server methods that also execute SQL or accept an HTTP request. The rules do not have owning application/domain modules; tests must reach through the same broad methods.
- `dependency_direction`：存在偏差。HTTP adapters directly receive and use *sql.Tx, and routing code accepts *http.Request and performs SQLite writes. Protocol and security helpers are cleanly separated, but the dominant dependency direction still runs from adapters into persistence and from policy into HTTP/SQL.
- `state_ownership`：存在偏差。One Server object owns sessions, login attempts, routing runtime, cooldown/health, latency pools, check-in-progress flags, configuration epoch and success evidence, largely under one mutex. Unrelated console and dispatch paths therefore share a mutable owner and lock scope.
- `testability`：符合。At the organization level, pure protocol/request validation and latency-delta helpers exist, and Store/routedGenerate methods provide direct seams alongside in-process and actual HTTP tests. This does not execute or prove those tests, and the broad handler methods still make the seams coarse.
- `necessary_abstractions`：符合。No empty layer or generic workflow engine is present; the small rowScanner and materialQuery interfaces have present scanning/query responsibilities. The main problem is missing boundaries and over-broad Store/Server objects, not unnecessary abstraction.
- `standards_alignment`：存在偏差。The executable architecture declaration and Trellis backend spec only model one internal/relay package and one web module, while the supplied Go SaaS standard requires independent application/domain packages and external HTTP/infrastructure. The engineering document acknowledges no domain split, so the current declaration cannot enforce the stated target.
- `saas_responsibilities`：存在偏差。Rule-rich console capabilities such as limits, keys, tenants and bills are implemented in net/http handlers that receive *sql.Tx and run SQL directly. There are no independent application/domain packages or operation-level storage interfaces, so HTTP adaptation, rules and transactions are fused.
- `gateway_request_lifecycle`：符合。Request/attempt lifecycle, streaming, cancellation, timeout and output commitment are centralized in generate/routedGenerate/stream and attemptOutput rather than scattered into routing or handlers. This is an organization assessment only; runtime correctness is not inferred.
- `gateway_upstream_separation`：存在偏差。Routing selection, health/latency state and SQLite evidence writes are in the same package and same methods as HTTP request handling. beginAttempt takes *http.Request and writes route decisions; acceptLatency writes latency evidence. This contradicts the model-gateway requirement that routing own policy/runtime state without HTTP or SQL.
- `frontend_features`：存在偏差。workspace.js keeps one global state object, one renderPage switch for every page, and shared edit/table/detail helpers across models, services, keys, tenants, limits, usage, billing, records and playground. There are no feature-owned UI/state/API/cleanup modules and no public feature entry points.
- 范围说明：Static source and document review only; no build, tests, browser or runtime execution was performed. Existing tests are cited only as seam evidence, not as executed proof.
- 范围说明：architecture.json declares only startup, relay and console modules; no application/domain packages exist to inspect, so the review assesses the declared modules and source organization rather than runtime behavior.
- 范围说明：Product rules, price/quota thresholds, permission outcomes and UI behavior are out of scope; findings concern responsibility, dependency, interface and state boundaries.
- 范围说明：Test files excluded from the architecture scan were read as context for test seams; they are not treated as production modules.
- 范围说明：The review does not assert supplier compatibility, deployment status or independent QA acceptance.

## 执行记录与检查范围

工具版本：0.4.1
Git HEAD：无
源码摘要：21389dcd501435bf5eb7b559d1665425f43d13554d09c32015f00b8f7bc6da1a

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
