# 架构模板与工程校验

本目录提供人类和 Agent 共同使用的架构规范、可运行样板和架构检查工具。后端与前端独立选择，按语言和场景组织；当前先覆盖 Go SaaS、Go 模型网关及 Vue 管理控制台。Agent 从 [AGENTS.md](AGENTS.md) 进入。

## 快速开始

在本仓库执行，目标目录必须尚不存在：

```sh
python3 examples/arch-template/scaffold.py \
  --destination /path/to/new-service \
  --backend saas --backend model-gateway --frontend vue
cd /path/to/new-service
python3 setup.py
python3 verify.py
```

Go 样板使用示例包路径 example.com/service；接入现有仓库时按其 go.mod 调整自身导入，不要求现有产品改模块名。可只选一个后端，或只选 Vue。初始化会复制明确版本的源码与工具，不创建指向本工作树的链接。项目按需修改业务代码与架构声明；工具升级通过维护源的 `scaffold.py --update-tool PROJECT` 执行，检查执行中或发现本地工具修改时拒绝覆盖，升级后重新安装依赖并验证。

运行要求：Python 3.10+、Go 1.25+（后端）、Node 22.12+（Vue 与 TypeScript 解析）、SQLite 示例需要 CGO/C 编译器。依赖版本与校验和由 go.mod/go.sum 和 package-lock.json 固定。

## 场景与说明

| 场景 | 参考实现 | 架构重点 |
| --- | --- | --- |
| [Go SaaS](backend/go/saas/README.md) | 配额预留、幂等、完成结算与真实 SQLite 事务 | 业务模块内的规则和用例、外部存储实现；保持事务边界 |
| [Go 模型网关](backend/go/model-gateway/README.md) | 目标选择、实际 HTTP 调用、流式输出与取消 | 请求生命周期、策略及自有状态、网络适配分工 |
| [Vue 管理控制台](frontend/vue/admin-console/README.md) | 真实配额 API 查询、局部状态、取消和迟到响应处理 | feature 公开入口、业务 API 与共享传输边界 |

后端示例可在一个 Go 服务中并用，也能分别使用；前端通过 API 契约连接。示例是可运行的架构切片，不包含完整身份体系、供应商适配、价格、账单或生产部署。每个场景说明明确扩展位置及限制。

```text
arch-template/
  AGENTS.md / principles.md / verification.md
  tool/                           共用架构检查与模型评审工具
  scaffold.py                     初始化及工具升级
  backend/go/
    saas/template/                业务规则、事务和 HTTP 示例
    model-gateway/template/       路由、请求生命周期及网络示例
  frontend/vue/admin-console/
    template/                     独立 Vue 工程
```

## 工程校验

[工具使用说明](tool/README.md)包含配置、安装、完整模型协议和退出码：

- 确定规则：模块覆盖、依赖方向、公开入口、外部导入约束和循环依赖，单独输出rules结果。Go 与 TypeScript/Vue 共用判定逻辑，解析失败或范围不完整不会通过。
- 模型评审：按通用及所选场景的检查项逐项给出结论，区分scope_notes和blockers；生成绑定源码与规范摘要的完整材料；通过受信命令或所附模型 HTTP 适配器评审职责、内聚、分层及过度抽象。工具核对引用与版本，语义结论保留为待审查发现。
- 项目验证：导出项目的 verify.py 执行相应的类型、构建、行为测试和架构规则；模型评审是单独记录的一步。

通用原则见 [principles.md](principles.md)，校验职责和验收要求见 [verification.md](verification.md)。新语言需要接入并验证事实提取，不因通用设计就宣称已支持所有语言。

## 交付范围

当前交付源码、独立初始化与工具升级、可执行规则和模型协议、三个参考切片及回归测试。验证记录见 [validation.md](validation.md)。模型协议测试、模型实际样例、目标产品验收分别记录；外部审查服务和 CI 尚需由采用项目配置并实际运行。

具体项目的模板选择、迁移计划和验收证据归项目文档。Model Relay 用于后续验证，不作为通用模板类别；本轮没有改造、部署或验收该产品。

## 已有仓库接入

从维护源的固定提交安装工具，不覆盖项目代码、AGENTS.md 或架构声明：

```sh
python3 examples/arch-template/scaffold.py --install-tool /path/to/repository
npm ci --ignore-scripts --prefix /path/to/repository/.harness/archcheck
python3 -I /path/to/repository/.harness/archcheck/inspect_project.py --root /path/to/repository
```

项目维护 architecture.json 并将检查命令接到自己的固定验证入口。已有安装用 `--update-tool`，拒绝本地工具漂移；只检查 Go 的项目不必安装 npm 解析器。将 harness-output/ 加入忽略规则。[ci.yml](ci.yml) 可复制为项目的架构 CI，绑定提交、展示 Markdown 结果并保留原始报告；平台自身使用同一维护源直接运行。

`inspect_project.py` 每次生成独立报告，同时更新 latest-report.json/md 两个固定普通文件，供平台按步骤归档；0=规则通过，1=规则违规，2=分析未完成；工作流包装无法保存本次报告时返回3并停止，不接受旧报告。报告保留源码摘要、实际范围和 Git HEAD，未提交修改仍以逐文件摘要为准。它不调用模型，也不代表产品 QA 通过。

## 工作流与 Issue

`workflow.add_check(graph)` 在固定测试的成功出口插入普通执行节点，保留后续独立审查和 QA。规则通过继续；规则违规回研发；分析不完整先归档错误，再停在处理阻塞节点，人工修复后明确重试。运行图绑定所选工具源码摘要，工具被修改、清单无效、额外 Python 模块或旧字节码不能把安装失败伪装成规则违规。

现有研发图有在途任务时，独立初检使用标准安装入口，不改旧图或旧 Run：

```sh
python3 examples/arch-template/install_workflow.py \
  --platform-url <平台地址> --token-file <既有私有API令牌文件> \
  --source-workflow <既有研发图ID> --name '架构检查 · 项目名' \
  --manifest <仓库外的私有安装清单>
```

该入口复用选定研发图的正式仓库准备和已授权运行完成/失败通知 Hook，不复制 Agent、模型或用户凭据。启动时通过现有 Run API/SDK 传 repository、repository_commit 和 issue_number，固定源码版本并关联 Issue；GitHub Issue 应标识为平台任务，避免旧入站重复接单。初检图成功仅表示检查执行结束，结论仍以报告为准。安装重入复用 ID，升级校验完整行为配置并分页拒绝在途 Run；旧冻结定义保持原样。采用新的工具版本后，通过明确的图升级更新其源码摘要，不在旧 Run 内更换工具。


标准扫描入口可同时运行规则与模型：见[工具说明](tool/README.md)。`rules.findings`是确定违规，`model_review.findings`是模型提出的候选问题；不能用数量直接比较模型好坏。0.4.0评审协议采用逐项checks、scope_notes/blockers和精确唯一原文定位，不接受旧limitations/模型行号格式；历史报告不重写。导出、安装、升级均保护默认原始产物目录。
