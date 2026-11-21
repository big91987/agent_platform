# 架构检查工具

本工具检查模块依赖、公开入口和循环，并提供模型架构评审及源码引用校验。Go 与 TypeScript/Vue 共用规则引擎；语言适配器负责提取事实。它不使用文件长度给仓库评“屎山分”。本工具治理代码组织与架构规范；格式、类型、构建由项目已有工具负责，产品业务正确性由产品测试负责。

版本 0.7.0。运行环境为 Linux/macOS；要求 Python 3.10+、Go 1.25+；分析 TypeScript/Vue 还需要 Node 22.12+ 和本目录锁定的 npm 依赖。没有平台服务或模型凭据也能运行确定规则检查。

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

报告保留 tool_version、snapshot、逐文件 hashes、规则扫描/排除范围、模型上下文清单、源码事实及两类独立结果：`rules.status/findings` 是确定规则；`model_review.checks/findings` 是模型判断。模型不能清除规则违规。确定规则违规不会被模型无发现覆盖。CLI stdout 是 JSON，--out 同时留存相同报告。

## 架构评分

标准检查入口和 CLI 的最终报告在 `score` 中提供总分、维度分、固定检查点分、扣分证据、规则违规数和评审完整度；Markdown 展示同一份数据。当前评分口径为 `2.0`，不修改规则门禁或退出码。

七个基础维度保持等权；每维固定四个检查点，由模型逐点提供结论、理由、原文证据和关联发现。检查点定义维护在 `archcheck/rubric.py`，不能由被扫仓库增删或调权。额外场景检查独立展示，未完成仍阻止总分，不增加计分分母。

| 维度 | 四个固定检查点 |
| --- | --- |
| 模块职责 | 职责内聚、接口边界、协议与用例分工、装配职责 |
| 规则归属 | 规则归属、规则唯一性、规则与I/O分离、规则变更范围 |
| 分层与依赖 | 依赖方向、模块封装、技术依赖隔离、循环依赖 |
| 状态归属 | 状态负责人、状态生命周期、状态修改边界、状态权威来源 |
| 可测试性 | 规则验证入口、外部依赖替换、失败可定位、验证状态控制 |
| 抽象合理性 | 抽象有实际职责、复杂度适当、复用边界、变化点明确 |
| 规范一致性 | 规范可定位、编码规范一致、架构规范一致、可执行约束一致 |

| 检查点结论 | 检查点得分 | 判定依据 |
| --- | ---: | --- |
| 符合 | 100 | 有证据支持该点符合要求 |
| 轻微偏差 | 80 | 该点存在局部命名、轻量重复或封装偏差 |
| 中度偏差 | 50 | 该点存在具体职责或依赖混合，阻碍独立修改或验证 |
| 严重偏差 | 0 | 该点的关键边界失效；不代表整个维度失效 |

同一检查点按其关联发现的最严重档计分，不按问题条数累扣。维度分是该维度适用点的均值，总分再对适用维度等权平均；最后显示一位小数。例：一个严重偏差点、其余三点符合，维度分为 `(0 + 100 + 100 + 100) / 4 = 75`。只有所有适用检查点均为0时，维度才为0。模型不能直接填写数值，也不能为了目标分数制造或扩散问题。

每个偏差点必须引用同一维度中的发现并提供证据；每个发现必须在其声明影响的每个维度至少对应一个偏差点。同一问题影响多个点时，每点必须说明具体影响，不能自动将整个维度降档。逐字引文、合法引用和父子状态一致由工具校验；影响及严重度仍是待复核模型判断。

确定规则违规只使对应依赖检查点为0：dependency/forbidden_import/external_dependency 对应“依赖方向”，private_entry 对应“模块封装”，cycle 对应“循环依赖”。模型将该点判为符合或不适用也不能覆盖确定规则。规则违规数量、红色结果与退出码独立保留，不能由较高总分抵消。

不适用必须说明具体范围理由，并从分母排除；证据不足、缺点、重复/未知点、非法引用、模型未执行或评审未完成时不发总分。全部不适用也不发总分。完成度同时列维度数和固定检查点数（28）；这是评审回报及引用核验，不等于测试覆盖率或模型实际阅读率。

“规范一致性”只评编码和架构约定；README 的业务承诺、配额/价格等产品口径由产品QA评估。临时观察配置不作为目标仓库的既有规范扣分。不能仅凭相似分支要求合并不同协议、用例或SQL方言。

0.7.0要求完整的逐点回执，缺失不推断、不补齐、不保留旧计分执行分支。旧原始回执和报告保留历史事实；不能将旧模型判断手工分配给检查点来产生新评分。新旧口径的分数变化不表示代码改善；即使同口径，多轮模型判断仍可能波动，不把此指数当作产品质量、研发效率或绝对成熟度测量。

## 结构方案与原生报告

0.6.0 的模型返回必须包含 `architecture_plan`，不接受缺少结构字段的旧回执。历史模型原件保持原样，不手工添加字段来冒充新评审。

- `summary`：当前到推荐方案的职责和依赖变化；无发现时明确保留当前结构。
- `modules`：模块名称、当前职责、推荐职责、当前路径、推荐路径、关联发现及证据。当前路径必须覆盖所有扫描生产源码且互不重复；规则分组不能自动冒充业务模块。
- `changes`：动作、现有源文件、可选解析符号、目标文件、责任变化、理由、依赖/接口及事务影响、关联发现和证据。每个发现有对应调整；不需要搬文件时用原地调整，不能强制重构。迁移项可以关联跨模块问题，不要求该发现只能属于源模块。

动作语义：`move` 是整文件移动，符号必须为 null；`extract` 提取对象或职责，原文件保留；`merge` 可汇入已有文件，符号为 null 时表示整个源文件合并；`adjust` 保持原路径。上下文规范文件仅允许原地调整，并在报告中标注为上下文，不纳入生产结构树。

所有路径都以 `--root` 为根，即使配置位于子目录。源文件、目录祖先与符号依据解析事实核验；符号重名时证据须唯一落在一个符号范围内。无法被解析器定位的对象使用 `source_symbol: null` 并在证据中给出实际代码，报告不伪造符号定位。新目标可尚不存在，但不能越界、制造文件/目录冲突或用整文件移动覆盖现有文件；合并到一个目标属于合法操作。

`report.model_review.architecture_plan` 保存校验后的模块和调整数据。现状树从 `facts.files` 生成；推荐树从现状加结构化方案生成，不接收模型的 ASCII 图作为事实。Markdown 在长证据前展示带问题注释的现状树、推荐树、模块职责对照和文件/函数调整表。JSON 与 Markdown 使用同一份数据，模型原回执不被渲染器改写。只跑规则时仍展示现状，推荐区明确未运行模型。

结构方案缺失或非法时，结构状态为 `incomplete`，保留已验证的诊断、原始回执、事实及规则；总报告返回 incomplete，沿用既有未完成不出总分的规则。结构建议不增加评分项，不预测改造后的分数。路径和引文核验不表示目标方案已经编译、实施或证明最优。

标准入口支持 `--max-bytes`，与 `bundle/review` 使用相同字节计数口径，默认 2,000,000，可显式提高并记录在报告中。预算必须是正整数，超出预算在调用模型前失败，不截断、不少扫源码。字节预算不等于模型 token 容量；调用方还须选用能容纳材料与输出的模型。

## 模型评审

标准入口将材料收集、受信命令调用、JSON校验及报告留存串起来。URL、密钥、模型由调用方显式配置；不用Codex也可使用同一协议。例如采用兼容Chat Completions的服务：

```sh
# CODE_REVIEW_URL、CODE_REVIEW_TOKEN、CODE_REVIEW_MODEL由私有环境提供。
python3 -I inspect_project.py --root PROJECT \
  --max-bytes 4000000
```

每次独立保留review-input.json、reviewer.stdout.json、reviewer.stderr.log、model-result.json、report.json和report.md；失败也保留已有规则结果及原始回执。导出、首次安装及升级都会在.gitignore末尾保护harness-output/和.harness-output/。这两个目录是工具保留的产物目录，不放业务源码；解析器也排除它们。自定义输出若位于扫描源码内而未处于保留目录，在调用模型前明确拒绝。发布前检查敏感信息。标准评测入口默认调用配置的模型并生成评分、诊断和结构建议。未配置 CODE_REVIEW_URL、CODE_REVIEW_TOKEN、CODE_REVIEW_MODEL 时明确 incomplete，不退化成仅规则报告；这些配置仍由调用方显式提供，工具不内置供应商或凭据。仅规则检查须显式指定 --rules-only，或使用专门的 archcheck.py check。

独立的bundle/review/validate-review命令仍可用于其他受信模型或原生Agent。`review --command`的stdin是完整bundle，stdout必须是一个JSON对象，不经shell、不自动重试、不默认选供应商。执行超时终止整个命令组。`--pass-env`只传所列私有配置；不能让被扫仓库决定受信命令。

### 要求与覆盖

内置检查覆盖模块职责、业务规则归属、依赖分工、状态归属、可测试性、必要抽象和规范一致性。场景可在architecture.json增加：

```json
"review_checks": [
  {"name": "gateway_transport", "requirement": "请求流程通过明确接口使用上游网络实现，路由不处理HTTP或SQL。"}
]
```

name必须唯一，不覆盖内置检查。导出场景自带对应要求，组合时保留；冲突明确报错。模型必须逐项输出conformant、deviation、not_applicable或insufficient_evidence及理由，已判断符合或偏离时附源码或规范证据。每项偏离须由finding引用其name，多个相关检查可合并成一项finding。

`coverage.scanned`是确定规则分析的生产源码；`coverage.context`是提供给模型的规范、配置、选定测试及其他上下文。测试可以从依赖规则排除而同时进入上下文；HTML/CSS可通过documents纳入，不宣称已有HTML/CSS依赖解析器。reviewed_files必须完整列出bundle.sources中的所有路径，这是模型自报阅读覆盖，不是独立阅读证明。

正常静态评审范围写入scope_notes；缺少必需资料写入blockers或对应检查的insufficient_evidence。前者不强制incomplete，后者不能报通过。不能通过填写“未执行产品测试”把正常架构评审变成执行失败。

### 发现与整改建议的范围

模型可以检查业务规则放置在哪个模块、是否重复、是否与 HTTP/SQL 耦合，不能据此判定价格、配额、权限结果等业务行为正确或错误。业务背景用于理解职责，不作为扩展产品需求的依据。

每项发现需要定位已有包、文件、类型或函数，指出违反的命名检查项及适用规范，并说明这些对象的职责或依赖应作的最小调整。不能只有“拆模块”“增加领域层”等原则口号；建议的新边界必须注明为建议，不冒充已有对象。没有测量的性能影响只能表述为风险。

validation 应说明架构复查方法，如依赖方向、公开入口、重复规则归属或模块测试接口；已有回归仅用于保护原行为，不凭空添加产品业务验收条件。当前可读报告及模型说明统一使用简体中文，代码标识和路径保持原样。模型仍可能偏离这些要求，引用校验不能代替语义复核；工具不以业务关键词过滤或自动改写模型意见。

inspect_project.py 自动生成的人类可读 Markdown 先分别展示规则结果与模型结果，再列问题、涉及位置、可定位的源码符号、依据、影响、调整和复查建议。符号来自语言解析事实，无法对应到符号的规范文件引用只显示文件位置。原始模型文字与 JSON 保持原样，执行元数据和范围放在报告末尾。当前返回协议包含逐点评审和结构方案。

### 统一返回协议

```json
{
  "snapshot": "材料中的摘要",
  "reviewed_files": ["材料中的所有源码和上下文路径"],
  "scope_notes": ["静态架构评审未执行业务测试"],
  "blockers": [],
  "checks": [{
    "name": "gateway_transport",
    "status": "deviation",
    "reason": "流程直接处理网络实现细节",
    "evidence": [{"path": "internal/gateway/relay.go", "quote": "一段完整且唯一的原文行"}],
    "criteria": []
  }],
  "findings": [{
    "checks": ["gateway_transport"],
    "title": "请求协调和上游网络实现混合",
    "severity": "medium",
    "principle": "请求流程与网络实现分工",
    "impact": "协议改动会触及请求协调",
    "recommendation": "由上游适配实现网络细节",
    "validation": "通过接口验证流程并保留真实HTTP集成测试",
    "evidence": [{"path": "internal/gateway/relay.go", "quote": "一段完整且唯一的原文行"}]
  }],
  "architecture_plan": {
    "summary": "保留网关流程，提取上游网络实现，通过接口调用。",
    "modules": [{
      "name": "gateway",
      "current_responsibility": "请求协调与网络调用混合",
      "responsibility": "请求协调与上游适配分工",
      "current_paths": ["internal/gateway"],
      "target_paths": ["internal/gateway", "internal/upstream"],
      "finding_refs": [1],
      "evidence": [{"path": "internal/gateway/relay.go", "quote": "该文件中完整且唯一的原文"}]
    }],
    "changes": [{
      "action": "extract",
      "source_path": "internal/gateway/relay.go",
      "source_symbol": null,
      "target_path": "internal/upstream/request.go",
      "responsibility": "上游网络调用；原文件保留请求协调",
      "reason": "隔离网络实现与协调职责",
      "dependency_change": "通过已有或必要的窄接口注入，保留取消及流式生命周期",
      "finding_refs": [1],
      "evidence": [{"path": "internal/gateway/relay.go", "quote": "该文件中完整且唯一的原文"}]
    }]
  }
}
```

实际checks必须覆盖材料中的全部要求；上面的gateway_transport是额外场景检查，criteria为空。七个内置维度每项必须带四个criteria结果，每个结果包含name、status、reason、evidence及finding_refs；模型请求response_contract列出完整固定名称与字段。status与父项使用相同枚举；偏差点的finding_refs为对应findings的1起始索引，其他状态为空。父项状态按证据不足、偏差、符合、不适用的优先序汇总。缺项、错引、偏差无依据或父子状态不一致明确incomplete。模型引用提交path和逐字原文，不再计数行号；可引用长行中的连续片段或完整多行，不得改写、省略拼接。工具在同版源码中精确、唯一匹配后生成line/end；零处或多处匹配（含重叠匹配）均拒绝，不猜位置、不修改模型原始发现。规范、源码、检查要求和策略变化均使旧材料失效。旧版limitations/行号协议明确拒绝；历史报告保持原样。

引用核验只证明原文真实和版本一致，不证明架构推断正确。标准化后的发现仍为verification=needs_review，交项目既有审查复核。规则违规与模型发现分别记录，不使用发现数量评估模型质量。

### 提供商适配与预算

model_reviewer.py只使用CODE_REVIEW_URL（完整Chat Completions地址）、CODE_REVIEW_TOKEN、CODE_REVIEW_MODEL。默认max_tokens=65536，可用CODE_REVIEW_MAX_TOKENS设置正整数并显式pass-env；更换模型时核对其上下文和输出上限。默认使用标准Chat Completions SSE并请求usage，避免长评审期间仅等待最终响应；只拼接content，丢弃reasoning_content。显式CODE_REVIEW_STREAM=false可选择非流式，须一并pass-env，不自动回退。缺少[DONE]、HTTP响应中断或超出接收预算均报incomplete，不自动重试。拒绝重定向、非stop结束、空白/非JSON响应，不把截断当成无发现。保存实际返回model、usage和耗时，不估造费用。不会保存reasoning_content。

例如DeepSeek V4.1 Flash当前使用deepseek-flash，完整地址https://api.deepseek.com/chat/completions；以实际/models响应及[官方模型文档](https://api-docs.deepseek.com/quick_start/pricing/)为准。工具本身不内置该模型或地址。向任何服务发送源码前，明确材料范围和凭据使用授权。

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
