# SaaS 业务管理参考

## 分工与 COLA 的对应

这是一个可运行的配额切片，用于演示架构；不包含完整用户、租户认证、订单和账单产品。

| COLA 职责 | 本例位置 | 具体工作 |
| --- | --- | --- |
| Adapter | internal/httpapi | 解析 HTTP 路径，映射状态与 JSON |
| Application | internal/accounting/application | 输入约束、业务操作入口、读结果及原子存储契约 |
| Domain | internal/accounting/domain | Quota、Reservation 及其额度/结算规则；不依赖 SQL 或 HTTP |
| Infrastructure | infrastructure/sqlite | 用事务实现预约、去重、完成及汇总更新 |

本模板必须按上表组织。Application 与 Domain 使用独立 Go 包；Domain 不能反向引用 Application，也不能导入第三方库。HTTP 不能直接引用 Domain 或 SQLite；由 Application 的 Usage 结果返回数据，避免把领域行为暴露给接口层。架构检查对上述导入关系直接报错。

仓储接口以 Reserve/Finish 表达完整原子操作。SQLite 在事务中读取当前状态，调用 Domain 规则后保存结果；不能把检查额度与写入拆成独立提交。Application 不向调用方暴露数据库句柄或事务对象。

领域模型由本说明中的规则、Go 模型及行为测试共同表达，不使用专门的模型语言。

## 已实现的业务约束

- 配额和 request_id 按 tenant 隔离。同一请求重复预留不重复计数；参数变化报冲突。
- used + reserved 不超过 limit；使用差值比较防止整数加法溢出。
- Finish 原子更新请求结算及租户用量。重复同值完成幂等，不同值报冲突；错误回滚。
- 此例消费量必须已知且不超过预留。未知用量、超额结算和价格策略需由目标业务明确，不能沿用本例假设。

SQLite 是真实存储实现，单连接限制在本进程串行访问，busy timeout 不是生产并发容量保证。跨进程、迁移、备份、金额精度和高可用需由采用项目补齐。

## 运行与验证

```sh
go mod download
go test -race ./...
go vet ./...
go build ./...
go run ./cmd/saas
```

API：GET http://127.0.0.1:8080/api/quotas/demo。程序创建 demo 租户及 100 单位额度，仅绑定本机。接口用于展示事务模块的消费方式，接入真实系统前添加身份、权限和项目选定的发布机制。不要将此 API 当作完整租户隔离认证实现。

对应测试覆盖数据库并发、幂等、租户隔离及失败状态；架构配置禁止业务代码依赖 HTTP 和 SQL。模型继续评估业务规则是否散落、接口是否泄露数据库、用例拆分是否必要。
