# SaaS 业务管理工程规范

先读 [架构及业务约束](docs/saas.md)，再读 architecture.json 中的可执行边界。

- 使用 Go 1.25 或更新版本，运行 gofmt、go vet、go test -race 和构建。
- 领域规则必须放在 internal/accounting/domain，用例及存储接口必须放在 internal/accounting/application；两者是独立 Go 包。HTTP 只调用 Application，禁止直接引用 Domain 或 SQLite。
- SQLite 适配器维护完整准入与结算事务，并在事务内依据实际读取值调用 Domain 规则。不得把原子业务操作拆成多个独立提交。
- 新增业务模块遵循相同的 application/domain 包结构；同步 architecture.json，规则变更必须评审，不能通过放宽规则给业务改动放行。
- 领域模型用 Markdown、必要的图和 Go 代码表达，不引入领域 DSL。
- 同一概念使用统一术语。注释解释业务原因和并发约束。不得用空层或透传接口代替模块职责。
- 变更业务规则时同步规则测试；变更网络、事务和取消时保留真实集成测试。
- 架构模型评审检查职责和抽象，规则通过不代表业务正确或产品验收通过。
