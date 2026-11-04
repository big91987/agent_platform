# 架构检查工具

本工具检查模块依赖、公开入口和循环，并提供模型架构评审及源码引用校验。Go 与 TypeScript/Vue 共用规则引擎；语言适配器负责提取事实。它不使用文件长度给仓库评“屎山分”。

版本 0.3.0。运行环境为 Linux/macOS；要求 Python 3.10+、Go 1.25+；分析 TypeScript/Vue 还需要 Node 22.12+ 和本目录锁定的 npm 依赖。没有平台服务或模型凭据也能运行确定规则检查。

## 安装与快速使用

```sh
# 在本工具目录安装 TypeScript / Vue 解析器；只检查 Go 时不需要这一步
npm ci --ignore-scripts
python3 archcheck.py check --root /path/to/project --out /path/to/report.json
```

项目根目录提供 architecture.json。使用 ../scaffold.py 初始化时会携带工具、规则、说明和验证入口；复制到另一个目录也可独立运行，不能依赖平台开发工作树。

## 架构声明

```json
{
  "source_roots": ["internal", "cmd"],
  "languages": ["go"],
  "exclude": ["**/*_test.go"],
  "documents": ["AGENTS.md", "docs/architecture.md"],
  "modules": [
    {"name": "domain", "paths": ["internal/domain/**"], "allow": [], "allow_external": [], "forbid_imports": ["database/sql*", "net/http*"]},
    {"name": "application", "paths": ["internal/application/**"], "allow": ["domain"], "allow_external": []},
    {"name": "entry", "paths": ["cmd/**"], "allow": ["application"]}
  ]
}
```

- source_roots、paths、public、exclude 和 documents 均相对于 architecture.json 所在目录；--root 是整个仓库边界。组件配置可用 `--root PROJECT --config web/architecture.json`，并用 documents 的 `../docs/engineering-principles.md` 引用仓库内共享规范；越出仓库仍会拒绝。source_roots 是明确的相对目录。每个选中源码必须且只能归一个 module；模块必须匹配实际源码。新增未归类代码、规则拼错、解析失败均返回 incomplete。
- paths / exclude / public / forbid_imports 使用区分大小写的路径通配符，`*` 可跨目录，`**` 也可跨目录；不是正则或 gitignore。根目录中的文件需单列匹配，例：`cmd/**`。
- allow 表示允许依赖的其他模块；本模块内部默认允许。未声明的跨模块依赖禁止。allow_external 限制第三方导入的路径通配符；空列表禁止第三方依赖，省略则不限制。Go 标准库由官方 go/build 元数据识别，不受该第三方列表限制；forbid_imports 仍可禁止其中的 HTTP、SQL 等具体包。它检查导入，不推断运行时通信。
- public 可限制 TypeScript/Vue 模块的公开文件，例如 `["src/features/quotas/index.ts"]`。Go 使用整个包作为边界：同一包的所有文件必须归一个架构模块，配置 public 文件列表会明确报错；不能以文件名伪造分层。
- exclude 明确列在覆盖报告中。Go 测试常有额外的组装依赖，可从架构依赖规则排除，再将关键测试路径列入 documents，供模型审查。documents 是精确文件路径，不是通配符。
- assets 列出未参与源码规则的资源文件；.git、node_modules、vendor、dist、__pycache__、.harness 是工具固定忽略的目录。不要将这些目录用于业务源码。
- 代码、架构配置、所列文档及存在的模块/依赖配置均按实际字节绑定摘要。变更中途修改、增加源码或更换配置，不沿用旧结论。

## 结果与退出码

| 退出码 | 状态 | 含义 |
| --- | --- | --- |
| 0 | conformant | 声明范围内的确定架构规则通过；model_review=not_run |
| 1 | violations | 存在可定位的规则违规 |
| 0 | model_review=no_findings | 模型自报覆盖完整且无发现，引用协议校验通过；不是架构无缺陷证明 |
| 1 | review_required | 模型有待审查发现，或确定规则违规；进入现有代码审查与返工 |
| 2 | incomplete | 缺文件/工具、未知配置、未覆盖源码、解析/解析路径失败、模型错误/超时、证据不符或评审范围不足 |

报告保留 tool_version、snapshot、逐文件 hashes、扫描/排除范围、源码事实、规则发现、分析限制和模型状态。确定规则违规不会被模型无发现覆盖。CLI stdout 是 JSON，--out 同时留存相同报告。

## 模型评审

模型接收规范、业务说明、完整选中源码、相关测试、结构事实及规则结果，评估内聚、逻辑归属、分层有效性、状态所有权和过度抽象。默认材料上限 2 MB，超过就停止，不截断。业务背景缺失必须在 limitations 说明。

可先生成材料，交现有 Agent 或人工主持的模型评审：

```sh
python3 archcheck.py bundle --root PROJECT --out /private/review-input.json
python3 archcheck.py validate-review --root PROJECT \
  --bundle /private/review-input.json --result /private/model-result.json \
  --out /private/review-report.json
```

也可直接调用受信命令。命令 stdin 接收完整 bundle，stdout 返回一个 JSON 对象；不经过 shell。超时会终止该命令的进程组，不自动重试。不继承任意环境变量，所需私有配置通过 --pass-env 显式传入。

```sh
python3 archcheck.py review --root PROJECT \
  --command '["python3", "/absolute/path/to/model_reviewer.py"]' \
  --pass-env CODE_REVIEW_URL --pass-env CODE_REVIEW_TOKEN --pass-env CODE_REVIEW_MODEL \
  --timeout 660 --out /private/review-report.json
```

model_reviewer.py 使用与现有代码审查一致的配置名：CODE_REVIEW_URL 是完整 Chat Completions 地址，TOKEN 和 MODEL 由私有执行环境提供。它拒绝 HTTP 跳转，不将凭据转发到其他地址，不自动重试。它发送 bundle 中所有 sources；需先确认该源码范围可发送到所选服务。适配器请求 JSON 输出，不做 Agent 工具循环、不默认选供应商、不读取旧任务密钥。其他原生 Agent 可实现相同命令协议，沿用其自己的只读/沙箱权限。

命令具有调用者授予的执行权限，本工具不提供恶意代码隔离。不要让仓库内容指定 reviewer 命令；CI 使用受信基线中的命令与配置。

返回格式（snapshot 和 reviewed_files 必须来自本次材料）：

```json
{
  "snapshot": "材料中的摘要",
  "reviewed_files": ["internal/application/order.go", "internal/domain/order.go"],
  "limitations": [],
  "findings": [{
    "title": "同一订单资格规则散落在两个用例",
    "severity": "medium",
    "principle": "业务规则归所属模块统一维护",
    "impact": "修改资格规则时两个入口可能行为不一致",
    "recommendation": "提取由订单模块维护的资格判断，两个用例共用",
    "validation": "验证两个入口在资格边界上的相同行为",
    "evidence": [{"path": "internal/application/order.go", "line": 10, "end": 10, "quote": "该行完整原文"}]
  }]
}
```

引用行号从 1 开始，quote 必须与指定完整行逐字一致。reviewed_files 必须与材料 coverage.scanned 完全一致；这是模型自报覆盖，不伪装成独立阅读证明。可选 reviewer 元数据由适配器记录实际模型标识、服务返回的 usage 和耗时，不估造 token 或费用。

引用核验只证明“引用真实且版本一致”，**不能证明架构判断正确**。每个发现保持 verification=needs_review，经现有审查补读相关代码、业务约束和反例；重要发现可以要求返工。若要扩大范围，将新文件加入架构声明或 documents，再生成新材料，不把模型在范围外的猜测当证据。

## 支持边界

- Go：官方 go/parser 扫描语法、包导入和函数/类型位置，go/build 识别标准库；包含选中的不同 build-tag 变体，不执行仓库代码。当前拒绝 Go replace、嵌套 module 及组件根 go.work；本地依赖未纳入范围会失败。运行实际构建、vet 和 race 测试验证编译及行为。
- TypeScript / JavaScript：官方 TypeScript AST，解析普通/type-only/import-type/re-export/字面量动态导入及 .mts/.cts/.mjs/.cjs 源码；Vue 使用官方 SFC 解析器。读取独立 tsconfig 的 paths/baseUrl，extends/references 明确拒绝；不把类型检查当成已执行。
- 计算出来的 import/require 不可解析时返回 incomplete。Vue 自动导入、动态组件和运行时反射的关系不在静态保证中，需项目明确配置或评审。
- 多语言共用规则不代表已经支持任意语言。新语言接入需提供同样的源码位置、unit 和 imports 事实，新增解析/解析路径正反例；规则引擎不重写。当前没有跨 HTTP/RPC 的调用关系推断。
- 模型协议/HTTP 测试使用受控响应，只验证接入；实际模型评审样例单独记录，不能把这些测试当成模型质量评测。

## 验证工具本身

```sh
python3 -m unittest discover -s tests
```

其中进程和模型传输测试需要本地端口；Go 解析需要可写的标准编译缓存。使用与业务应用相隔离的测试环境运行。
