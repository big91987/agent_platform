# Go SaaS 业务管理

本场景面向身份、权限、业务配置、配额及账务等管理业务。采用本模板后，业务模块必须使用独立的 application 和 domain Go 包；HTTP 与基础设施实现统一外置。前端技术栈独立选择。

可运行样板、规则和行为测试位于 [template](template/AGENTS.md)。先读 [Go 共用要求](../README.md)，以及[通用原则](../../../principles.md)和[工程校验与验收](../../../verification.md)。

## 职责分层

| 职责 | 代码归属 | 作用 |
| --- | --- | --- |
| Adapter | httpapi 和 CLI | 输入解析、协议校验及响应映射 |
| Application | internal/<业务模块>/application | 协调一次完整操作，明确事务、执行顺序和失败处理 |
| Domain | internal/<业务模块>/domain | 业务对象、权限、金额和状态规则 |
| Infrastructure | 具体技术实现 | 存储、凭据和其他外部依赖 |

目录与依赖是强制约定。Application 调用本模块 Domain，并定义所需存储接口；HTTP 只能调用 Application，不能直接引用 Domain 或 SQLite。Domain 不得引用 Application、其他模块或外部库，也不得引用标准库中的 HTTP/SQL。SQLite 实现业务模块声明的接口，入口负责组装。简单规则保持精简，不为每张表创建聚合、接口或透传类。

```text
internal/
  identity/{application,domain}/     用户、租户、成员和邀请
  access/{application,domain}/       Key 生命周期及授权
  catalog/{application,domain}/      业务资源和配置
  accounting/{application,domain}/   配额、价格、用量和账单
  httpapi/                      HTTP 适配
  infrastructure/sqlite/        按业务分别实现存储接口
```

上面列出 MaaS 的业务模块归属；可运行样板只实现 accounting 配额切片。新增业务模块必须使用同样的 application/domain 结构并声明依赖，不允许通过放宽现有规则消除违规。技术实现按业务分别组织，不能把所有 SQL 集中回一个通用 Store。

领域模型使用 Markdown 说明、必要的模型/状态图及 Go 代码；本模板不引入专门的领域建模语言或 JSON 驱动业务执行引擎。

## 典型操作与一致性

“邀请成员”由 HTTP 适配调用邀请用例，用例协调操作者权限、邀请规则和保存；存储实现在事务内完成所需写入。调用方不直接拼接该模块的表或权限检查。

配额准入要把授权复核、幂等登记、额度判断及预留作为适用的原子操作；请求完成时用量、终态与结算也要保持一致。具体事务由用例要求决定，不能因为拆模块变成多个独立提交。价格计算、账单生命周期和只读报表可以分别采用纯函数、业务模型及查询用例。

## 校验与工具候选

| 要求 | 校验方式 |
| --- | --- |
| 业务模块不引用 HTTP 处理器、SQLite 实现或其他模块内部实现 | Go 分析提供包和依赖事实，由统一架构规则判定；验证未覆盖包及违规样本 |
| 格式、静态错误、模块完整性和实际构建 | gofmt、go vet、模块校验和 Go 编译 |
| 权限、金额、状态、幂等、额度及结算正确 | 独立规则测试、存储集成测试、race 和实际 API 测试 |
| 对接现有前端或网关后结果一致 | 组合项目的契约测试及真实用户旅程 |

工具版本与入口见 [Go 共用要求](../README.md)及[工具手册](../../../tool/README.md)；样板配置直接由同一引擎执行。架构参考：[COLA](https://github.com/alibaba/COLA)、[Kratos layout](https://github.com/go-kratos/kratos-layout)。
