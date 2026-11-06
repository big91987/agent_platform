# Provider-independent architecture review implementation plan

**Goal:** Separate executable rule violations from model judgments, make the selected architecture requirements auditable, and exercise the same tool with the user-authorized DeepSeek V4.1 Flash endpoint.

**Architecture:** Keep the current parser and provider-neutral command boundary. Rules remain deterministic; model review must account for every selected check, distinguish scope notes from blockers, and submit exact source quotations whose locations the tool resolves. A model finding remains a candidate after citation validation. Do not alter frozen historical reports or introduce compatibility aliases for the changed review contract.

**Execution:** The user explicitly requested autonomous fixes and multiple real model rounds. Implement in this existing isolated worktree, using focused red/green tests, without another design approval loop.

## Tasks

- [x] Capture an actual DeepSeek baseline using the current tool and the same fixed Model Relay bundle; retain returned model ID, usage, timing and raw response.
- [x] Add failing behavioral tests for normal scope notes versus missing context, per-check coverage, exact unique quote resolution and ambiguous/missing evidence. Implement the minimum new review contract in review.py and configuration validation in engine.py. Add explicit rule results and model review sections, update current consumers and bump the tool version.
- [x] Add transport regressions for truncated/blank responses and configurable output bounds. Keep URL/token/model external, never substitute a model or retry failed API requests automatically.
- [x] Add a reusable scan entry that builds the bundle, executes the configured reviewer, validates its untouched JSON and retains separate raw/model/rule reports. Include relevant HTML/CSS, project verification entry and selected tests as explicitly declared context, not pretend parser support.
- [x] Update current tests, docs and scenario defaults; run the tool/scaffold/workflow suites and lint. Install/update through scaffold.py and verify source provenance.
- [x] Run at least two independent post-change DeepSeek scans on one fixed Model Relay source/standards snapshot, without seeding prior findings. Compare requirement coverage, evidence validity, consistent findings and cost; keep model accuracy claims limited to these runs.
- [x] Publish sanitized standard reports and source changes on the existing draft PR and update the linked Issue. No product refactor, merge, deployment or server-permission changes.

## Review focus

- Required checks cannot disappear or be duplicated while reviewed_files claims full coverage.
- A normal static-review boundary must not become an execution failure; missing required evidence must remain incomplete.
- Exact text found at multiple locations is ambiguous and must not be guessed; source drift still invalidates results.
- Rule violations cannot be cleared by model output, including empty or failed model results.
- No credential, private history or local absolute path enters published artifacts; API truncation never becomes a clean report.


## Actual result

Tool0.4.0: 43 tool tests,12 scaffold/install/workflow tests and Ruff passed. Independent review's artifact-isolation and source-version findings were reproduced and fixed; no remaining Important findings. Standard upgrade applied to Model Relay branch; current rule scan remains separate from model findings.

Seven authorized DeepSeek calls were retained: one0.3.1 baseline and six development/verification rounds. Final SSE round6 accounted for all11 checks and returned7 citation-checked candidate findings; round5 omitted coverage declarations and was rejected. Earlier full-line quote false rejection and HTTP body interruptions were retained and led to fixes. No hidden retry or hand-authored model conclusion. Actual112 Workflow execution/upgrade, product refactor, QA, merge and deployment remain outside this task's completed scope.

## 2026-11-06：核对治理范围与报告可读性

用户再次明确：本任务评估、修复代码与架构规范治理工具，不设计或实施 Model Relay 业务功能。原始七项候选均为架构判断；协调者随后补出的业务验收细节不能算工具输出。检查发现模型的 recommendation/validation 指令过于宽泛，生成的 Markdown 又先列技术状态、直接显示字段名，不便定位整改对象。

采用最小修订：保留 JSON 协议和依赖规则语义；明确业务规则“归属/重复/耦合”属于架构治理，业务计算是否正确不属于此工具判定。提示词要求建议指向已有路径/符号、明确职责与依赖的最小变化，不发明新业务条件。报告通过现有源码事实展示位置与符号，原样呈现模型意见；不以文本过滤充当语义审核。

验证：先补报告保留原始意见、定位源码对象及失败状态不误报的回归，再修改维护源，跑工具与安装路径测试。已有历史报告不改写成新模型结论；模型质量与协议测试分别说明。不修改产品代码或执行产品验收。

### 本轮实际结果

维护源0.4.1保留返回协议，明确架构治理范围、具体对象、中文与允许返回的字段。报告函数最终留在启动脚本：独立复审发现拆为延迟导入模块会在升级锁冲突时丢回执，已先复现、补回归并修复，复审无遗留 Important/Critical。

50项工具回归、12项模板安装/工作流测试及Ruff通过。真实DeepSeek另运行3轮且不自动重试：首轮英文输出被协议及引用校验接受但仍有模型过度概括；次轮中文多出kind字段被拒绝；末轮中文及正确字段，但一段引用重复两次而被拒绝。三轮材料相同、指令逐步收紧，全部原始证据保留；不能按发现数排名或把拒收改成成功。模型输出、唯一引用稳定性和全量材料成本仍有不足。本轮未修改Model Relay业务代码，也未将0.4.1升级到测试仓或112工作流。
