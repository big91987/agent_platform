# Vue 控制台工程规范

先读 [架构与接口](docs/admin-console.md)，依赖规则由 architecture.json 执行。

- 按业务功能组织。页面和组件负责展示；composable 拥有局部异步状态；api.ts 负责业务接口；shared/api 负责传输。
- 外部使用 feature 的 index.ts，不直接访问其内部状态；共享模块不依赖业务模块。
- 不将租户数据放入全局单例。取消和迟到响应不能污染当前页面。
- 使用严格 TypeScript、明确错误/加载状态和可访问的表单。规范、类型、构建和测试均运行，不能用 Vite 转译代替类型检查。
