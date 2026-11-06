# 代码与架构规范检查

- 规则扫描：未发现规则违规（仅限已声明的规则）。
- 模型评审：模型评审未完成。

规则检查依据已声明的依赖约束；模型意见需复核。本报告不判定产品业务功能通过或失败。

## 规则违规

声明范围内未发现规则违规。

检查未完成：evidence quotation must match one exact unique span: internal/relay/routing_latency_test.go

## 模型架构发现（待复核）

模型评审未完成。

## 逐项评审与限制


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
