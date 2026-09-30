# Agent Platform 需求澄清记录

> Status: Complete（实验版需求已收口）  
> Target artifacts: `prd.md`（实验实施基线）, `review.md`  
> Updated: 2026-10-01  
> 方法：`strict-resumable-grilling`。用户要求先澄清，再确认 PRD 和原型。未经批准的先行 PRD 草案不是需求事实源。

## 已确认的上下文

| 事实 | 用户依据 |
|---|---|
| 通用 Agent 平台，具体用途由执行器、Skill、工具与配置决定 | 用户纠正固定研发任务、需求／设计／研发 Agent 的平台定位 |
| 通过外部 API／Webhook 调用指定 Agent，平台负责调度和持续会话 | 用户明确描述调用入口与平台责任 |
| Codex、Claude Code 和其他执行器可接入；可配置原生参数、工具与 Hook | 用户明确描述原生执行器代理和定制能力 |
| Session 与 Runtime 分离 | 用户已确认平台建设方向 |
| 会话与协作状态由本平台统一维护，GitLab 等外部系统不承担 Session 的事实源 | 用户明确后续需要返回平台由用户处理和协作 |
| Session／Runtime 分离是平台内部实现，不向用户或外部调用方暴露运行与恢复细节 | 用户明确纠正对外接口和内部原理的边界 |
| 外部入口与平台界面须能接续同一段工作 | 用户明确后续可能返回平台由用户处理 |
| Go 后台；首版自用，认证和部署简单；不强制 K8s | 用户明确约束，并授权后续评估单体或服务拆分 |
| 单 Agent 优先；Workflow 和蜂群后置 | 用户最新范围裁剪 |
| 未来内部 Workflow 由 Agent Graph 构成，对外与普通 Agent 同一接口类别；允许共享记忆与交流 | 用户解释多 Agent 的准确含义，当前不实施 |
| 项目目录为 `<workspace-root>/agent_platform` | 用户对目录和当前仅做需求阶段的计划回复“可以” |

## Checkpoint

| Field | Value |
|---|---|
| active_batch | none（R-001 已收口） |
| active_question_ids | none |
| unanswered_ids | none |
| next_id | D-005 |
| accepted_count | 3（D-001 至 D-003；D-004 已撤回） |
| provisional_count | 0 |
| conflict_count | 0 |
| frozen_count | 0 |
| unscanned_coverage_count | 0 |
| single_question_reason | none（原批次无未答选项） |
| resume_note | 当前无阻塞澄清。用户授权按合理架构连续实现，采用 PRD 的 Codex 首接、同机执行、独立调用凭据、消息排队、停止后明确继续、配置快照和手工清理默认值。Workflow、蜂群、共享记忆、远程执行后置。历史讨论保留如下；当前合同以 PRD 和架构为准。 |

## 历史 Coverage map（当前实验覆盖以 PRD AC 与验收记录为准）

| Dimension | Status | 证据／待澄清范围 |
|---|---|---|
| Goal and success | Open | 通用配置与托管方向已确认；首个实际验证场景和成功结果后续收口 |
| Scope and non-goals | Closed | 单 Agent 首版，Workflow／蜂群后置；Go、简化认证与部署已确认 |
| Stakeholders and users | Open | 内部配置者、会话参与者、外部调用系统；操作权限与分享规则待确认 |
| Roles and authority | Open | D-004 已撤回；PRD 提出简单内部登录和调用凭据边界，待审阅 |
| Domain objects and lifecycle | Open | Agent 配置、Session、一次执行、Runtime；并发输入 D-003，配置变更和数据留存随后细化 |
| Object provenance and bootstrap | Open | 配置入口 D-002；执行器接入与核验、凭据来源随后细化 |
| Main workflows | Open | 调用创建／续接已确认；D-003 确认运行中消息，结果返回与人工交互随后细化 |
| Exceptions and failure | Open | 恢复失败、超时、重试与副作用边界待下一批澄清 |
| Data and integration | Open | API／Webhook 已确认；输入资料、回调和共享信息范围后续收口 |
| Quality attributes | Open | Session／Runtime 分离已确认；执行位置 D-001，访问范围见 PRD 待审建议；D-004 已撤回 |
| Operations | Open | D-001 已确认同机本地执行且无远程执行需求；运行观察与清理仍待规格明确 |
| Migration and rollout | Open | 新项目，不迁移旧产品；原生配置采用 D-002、验证与首版上线边界后续确认 |
| Acceptance and evidence | Open | 配置、真实调用、续接、失败和恢复将对应验收场景；尚未定稿 |

## 历史角色矩阵（需求收口前的讨论快照，不再作为当前合同）

| 维度 | 配置者／管理员 | 会话参与者 | 外部调用系统 |
|---|---|---|---|
| Purpose | 提供可调用的 Agent 配置与环境 | 获得工作结果并参与对话 | 把业务输入交给指定 Agent |
| Responsibilities | 配置、核验、容量和支持；边界待定 | 输入、澄清与结果审查 | 请求来源、会话关联和结果消费 |
| Decision rights | 原生配置入口 D-002；其他待确认 | 运行中回复 D-003，跨用户接续策略未批准，D-004 已撤回 | 调用授权范围待确认 |
| Required approvals | 配置启用／变更是否审批后续确认 | 不内置固定业务审批，业务由配置与 Agent 决定 | 凭据授权后续确认 |
| Permissions | 管理范围后续确认 | 会话可见范围见 PRD 待审建议；D-004 已撤回 | 只访问授权 Agent／会话，具体范围待确认 |
| Inputs | 执行器、Skill、工具、凭据与环境 | 会话、消息、产物 | 用户输入、事件和关联标识 |
| Outputs | 可调用配置 | 回复或明确操作 | 调用与续接输入 |
| Handoffs | 调试后提供调用入口；规则待确认 | 回复交给当前会话 | 平台接收与回调；规则待确认 |
| Failure ownership | 环境／配置异常的具体处置待确认 | 业务判断与重试选择待确认 | 回调故障的处置待确认 |
| Escalation | 首版由内部团队维护，具体入口后续确认 | Agent 无法继续时所需提示后续确认 | 调用／回调错误返回后续确认 |
| Conflict rule | 权限与业务配置的冲突规则后续确认 | 并发输入先确认 D-003 | 重复事件与会话归属后续确认 |

## R-001：首版使用与执行边界

- Ordered decision IDs: D-001, D-002, D-003, D-004
- Dependency check: 四项独立；执行位置不预决配置入口、消息接续或可见规则。已确认的单 Agent、通用 API 与轻量部署不重复询问。
- Comparable context and answer shape: 都是首版用户行为与范围的选择，有推荐与影响说明；允许用户自由修正。
- single_question_reason: none
- response_received: none
- answer_to_id_mapping: none
- resolved_ids: none
- provisional_ids: none
- unanswered_ids: D-001, D-002, D-003, D-004
- write_verification: 已回读核验 R-001、四个唯一问题、Asked 状态、unanswered_ids 和 next_id；记录未包含用户答案。

### D-001

| Field | Value |
|---|---|
| semantic_key | runtime.initial-execution-location |
| context | 用户确认轻量自用和后台调度，但尚未明确是否首版就接入个人电脑。 |
| question | 首版 Agent 实际在哪执行？ |
| options | A．先在平台部署的服务器上执行（推荐）：集中准备环境，先跑通托管闭环。B．首版就接入成员电脑／独立执行机器：保留各自环境，但要同时建设机器接入、在线状态与断线恢复。 |
| recommendation | A；先降低环境与接入复杂度，独立执行机器后续按真实需求增加。 |
| user_answer | “这肯定是本地的，本地执行的，都在同一个机器上的。不会有那种远程调用的需求。” |
| conclusion | 平台与 Agent 执行器部署并运行在同一台本机。远程执行机器、Worker 接入、机器注册与跨机器调度不属于当前需求，不默认加入后续路线图。外部业务系统通过平台 API／Webhook 提交输入仍在范围内。 |
| status | Accepted |
| prerequisites | 已确认首版单 Agent 和轻量自用方向 |
| descendants | 执行器接入、环境准备、资源和故障边界的后续问题 |
| supersedes | none |
| coverage_dimensions | Quality attributes, Operations, Object provenance and bootstrap |
| maturity_checks | Meaning：同机本地执行；Boundary：不含远程执行，保留外部业务 API／Webhook 接入；Authority：平台调度本机执行器；Failure：本机进程故障由平台记录并恢复，会话持久化规则已确认；Counterexample：多个外部系统不代表跨机器 Worker；Consistency：符合轻量部署与内部运行边界；Impact：PRD 部署范围和执行器接入；Evidence：用户明确决定。 |
| probes | none |
| source | 用户对执行基础设施的历史讨论及本轮澄清要求 |
| artifact_mapping | PRD：运营方式、运行环境范围和恢复承诺 |


### D-002

| Field | Value |
|---|---|
| semantic_key | agent.initial-configuration-entry |
| context | 用户要求平台可配置原生 Agent 的 Skill、Hook 与其他参数；尚未明确配置体验。 |
| question | 你希望怎样配置 Codex／CC 这些 Agent？ |
| options | A．常用项用页面配置，高级项保留原生配置入口（推荐）：方便使用，同时保留执行器自己的能力。B．首版主要编辑或导入原生配置文件：页面只管理配置来源、调用和运行，界面更少，但使用者需要熟悉原生配置。 |
| recommendation | A；避免平台替原生执行器重新发明配置体系，同时提供常用操作。 |
| user_answer | 其他默认就行 → 本次呈现 D-002 推荐 A |
| conclusion | 选择页面配置常用项，Skill、Hook 等高级项保留原生配置入口；具体实验版配置项和错误提示将在 PRD 中明确。 |
| status | Accepted |
| prerequisites | 已确认原生执行器代理与定制方向 |
| descendants | 配置核验、配置更新与原生配置采用的后续问题 |
| supersedes | none |
| coverage_dimensions | Object provenance and bootstrap, Migration and rollout, Roles and authority |
| maturity_checks | Meaning/Boundary/Consistency/Impact 已明确；配置格式、原生核验和错误提示已在获授权实验范围中确定，见架构与验收记录。 |
| probes | none |
| source | 用户关于可配置 Hook、Skill 和执行器参数的明确要求 |
| artifact_mapping | PRD：Agent 配置与调试旅程 |


### D-003

| Field | Value |
|---|---|
| semantic_key | conversation.message-during-running |
| context | 原生 Session 持续对话已确认；运行中追加输入的处理方式尚未确认。 |
| question | Agent 正在工作时，我再发一句话，应该怎么处理？ |
| options | A．保存下来，当前这一轮结束后接着处理；另有明确的停止操作（推荐）。B．默认打断当前执行，立即处理新消息。C．每次发送时由调用方选择“接续”或“打断”：更灵活，但接口与交互多一个选择。 |
| recommendation | A；普通补充不误停工作，主动停止可明确控制。 |
| user_answer | 其他默认就行 → 本次呈现 D-003 推荐 A |
| conclusion | 运行中追加消息保存排队，当前一轮结束后处理；提供单独停止操作。停止与后续排队消息的处置尚未在问题选项中明确。 |
| status | Accepted |
| prerequisites | 已确认持续会话与 Session／Runtime 分离 |
| descendants | 停止后消息处置、并发顺序和事件展示的后续问题 |
| supersedes | none |
| coverage_dimensions | Main workflows, Domain objects and lifecycle, Exceptions and failure |
| maturity_checks | Meaning/Boundary/Consistency/Impact 已明确；停止保留队列、明确继续、不自动重放未知输入已在获授权实验范围中确定，并经实际执行验证。 |
| probes | none |
| source | 用户要求澄清；先行草案的排队规则尚未批准 |
| artifact_mapping | PRD：运行中追加消息、停止和恢复旅程 |


### D-004

| Field | Value |
|---|---|
| semantic_key | conversation.team-visibility-and-participation |
| context | 用户要求简单认证与自用；不同会话隔离不等于团队成员能否查看或接续彼此会话。 |
| question | 首版团队成员之间的会话怎么共享？ |
| options | A．团队内默认可查看并参与彼此会话（推荐）：方便协作，先用简单权限；会话上下文与工作文件仍各自隔离。B．默认仅创建者可见，显式分享后其他成员才能查看和参与：更私密，但增加分享与权限管理。 |
| recommendation | A；适用于可信内部团队。涉及个人或敏感数据时可选 B，不能把共享配置误当作共享会话。 |
| user_answer | 用户否定团队成员假设，明确共享由平台策略或工作流设置决定。 |
| conclusion | 原问题及其默认团队共享推荐撤回；访问授权具体规则后置，不要求用户选择原选项。 |
| status | Superseded |
| prerequisites | 已确认团队自用与认证简化 |
| descendants | 成员角色、系统凭据权限、分享／清理的后续问题 |
| supersedes | none |
| coverage_dimensions | Stakeholders and users, Roles and authority, Data and integration |
| maturity_checks | 八项检查待答案；多人同时回复的处置在 D-003 答案形成后继续细化。 |
| probes | none |
| source | 用户要求先澄清，私有会话仅为先行草案的未批准默认 |
| artifact_mapping | PRD：用户权限、协作与会话访问 |

## 发给用户的完整本轮问题

已经明确的通用平台、Go、单 Agent 优先、Workflow／蜂群后置，我不再重复问。第一轮确认四个独立的使用规则：

**D-001：首版 Agent 实际在哪执行？**

- **A．先在平台部署的服务器上执行（推荐）**：集中准备环境，先跑通托管闭环。
- B．首版就接入成员电脑／独立执行机器：保留各自环境，但要同时建设机器接入、在线状态与断线恢复。

**D-002：你希望怎样配置 Codex／CC 这些 Agent？**

- **A．常用项用页面配置，高级项保留原生配置入口（推荐）**：方便使用，同时保留执行器自己的能力。
- B．首版主要编辑或导入原生配置文件：页面只管理配置来源、调用和运行，界面更少，但使用者需要熟悉原生配置。

**D-003：Agent 正在工作时，我再发一句话，应该怎么处理？**

- **A．保存下来，当前这一轮结束后接着处理；另有明确的停止操作（推荐）。**
- B．默认打断当前执行，立即处理新消息。
- C．每次发送时由调用方选择“接续”或“打断”：更灵活，但接口与交互多一个选择。

**D-004：首版团队成员之间的会话怎么共享？**

- **A．团队内默认可查看并参与彼此会话（推荐）**：方便协作，先用简单权限；会话上下文与工作文件仍各自隔离。
- B．默认仅创建者可见，显式分享后其他成员才能查看和参与：更私密，但增加分享与权限管理。

你可以回复“全部 A”，或“D-003 选 C，其余 A”，也可以直接修改选项。暂时拿不准的项可以说“待定”，我会保留并说明对首版的影响。


## 2026-09-30：调用身份与 Session 接口研究（非已接受决定）

### 对 R-001 的纠正

- 用户指出 D-004 中“团队成员”不符合通用 Agent 平台定位。原问题及选项属于历史记录，其团队默认共享推荐已撤回，不能作为待用户接受的有效推荐或既有需求。
- 用户未选择 D-001 至 D-004 的任何选项。会话归属和共享边界仍待明确，不把研究建议登记为 Accepted。
- 用户明确：外部系统调用需要考虑会话标识、系统身份和共享；具体接口尚未想好。用户将研究对象纠正为 Anthropic 的平台。

### 经官方资料核对的事实

- Anthropic Managed Agents 将持久化 Session 事件日志、Harness 执行循环和 Sandbox 执行环境分开；Harness 可通过 Session 日志恢复。
- 创建 Session 绑定 Agent 和 Environment；后续通过同一 Session 的 events 接口发送输入，通过事件流和历史读取输出。
- API 身份及访问范围由凭据和 Workspace 控制；Session ID 用于定位会话，不等于调用权限。
- 会话历史与 Sandbox 状态保留规则不同，不能仅凭 Session ID 承诺任意 Runtime 上原样恢复工作文件与执行器原生状态。

官方来源：

- https://www.anthropic.com/engineering/managed-agents
- https://platform.claude.com/docs/en/managed-agents/sessions
- https://platform.claude.com/docs/en/managed-agents/events-and-streaming
- https://platform.claude.com/docs/en/manage-claude/workspaces

### 待讨论建议

- 用调用凭据识别接入系统，用 Agent 标识选择配置，用平台 Session ID 关联持续会话；不要求外部调用者指定 Runtime 或原生执行器 Session ID。
- 先保留创建会话、发送事件、查询会话、读取事件四类接口；调用接收与实际执行完成分开。
- 外部事件按来源去重；同一 Session 的执行串行化，不同 Session 可并行。运行中消息排队还是打断仍为 D-003 待定项。
- 复用 Agent 配置、共享同一 Session、共享知识／记忆属于三个不同边界；不默认互相包含。
- 跨系统访问同一 Session 必须明确授权；首版授权形式待用户确认，不提前引入团队模型。


## 2026-09-30：平台会话归属与对外边界（用户明确决定）

- 用户原意：平台后续承载协作；Session 不能在 GitLab 维护，因为工作可能回到平台，由平台用户处理。Session 与 Runtime 分离是平台内部原理，不应暴露给用户侧或外部接口调用方。
- 已确认边界：平台统一持久化会话、工作状态及平台协作需要的关联；GitLab、其他消息渠道及外部 API 是入口，不能替代平台维护会话。
- 已确认边界：用户和调用方不负责选择 Runtime、恢复执行器会话、定位执行机器或管理执行进程。
- 已确认场景：外部消息发起工作后，用户能在平台中查看和接续处理；平台接续不建立一段割裂的新工作上下文。
- 对上一轮提案的纠正：GitLab Issue 与平台会话的关联应由平台接入侧维护，不能要求 GitLab 成为 Session 事实源或依赖调用方保存关联才可恢复。
- 尚未确认：外部调用具体采用透明的平台会话标识还是外部业务关联键、共享权限模型、运行中输入策略和接口字段。内部执行器会话 ID 与对外关联标识不得混用；不能以本轮决定推导取消所有对外会话关联能力。

这些决定将约束后续 PRD 和接口设计；未开始实现或原型。


## 2026-09-30：逻辑会话实例与执行容量（用户明确模型）

- 用户示例：两个外部系统各有五个用户，各自建立一个会话，平台承载十个独立的逻辑会话实例；内部可以使用五个执行单元与十个持久化 Session 异步衔接。
- 已确认：逻辑会话实例数量与执行单元数量不要求一一对应。内部调度与持久化细节不暴露为用户必须管理的运行对象。
- 已确认：用户之间、Agent 之间的 Session 共享由平台功能策略或工作流配置确定，不因使用同一 Agent 配置或同一执行机器而自动共享。
- 语义边界：示例中十名用户各开一个会话，不代表平台强制一名用户只能有一个 Session；具体会话创建与接续操作仍待澄清。
- 语义边界：等待用户输入的会话不要求持续占用执行单元；在执行单元一次只承载一个执行的简化假设下，五个执行单元最多同时执行五个会话，其他可执行会话等待调度。此容量说明不是新增的用户选择或已批准调度算法。
- 首版继续只做基本单 Agent 能力；本轮未将 Workflow 或蜂群提前纳入实现范围。


## 2026-09-30：首版部署简化与对外实例标识

### 用户明确方向

- 首版不要求把 Session、Runtime 拆成独立服务；后续有性能瓶颈等实际证据再评估拆分。
- 对外调用参数需要明确；新建的逻辑会话实例应有一个 ID，用于后续接续。
- 与此前决定的关系：对外会话连续性和独立上下文仍成立；Session／Runtime 生命周期边界不等于首版要拆微服务。

### 本轮建议（待确认，不是已定接口合同）

- 首版采用同一 Go 服务承载接口、会话持久化和执行调度，仍将持久化数据放在可恢复存储中，避免因进程结束丢失会话。
- 对外字段建议为 instance_id，表达用户所说的持续会话实例；它与此前讨论的平台 Session ID 是同一标识，不新增另一层实例对象或第二套 ID。
- 创建时指定 Agent、输入和调用身份，平台生成并返回实例 ID；后续输入携带该实例 ID。调用身份来自凭据或登录。
- 外部系统代理其用户调用时，若需区分用户归属，则携带该系统的用户标识，并与已认证的系统身份组合；不要求先创建平台团队成员。具体共享授权仍待确认。
- Runtime ID、执行器原生 Session ID、进程及机器位置属于内部信息，调用方不负责传入。

未开始平台代码或原型。


## 2026-09-30：参考 Dify 与 Coze 的对外调用方式

### 用户明确要求

- 参考 Dify 和 Coze 官方 API，先核对实际调用需要传哪些信息，再确定本平台接口。

### 官方资料核对

- Dify POST /chat-messages：应用 API key 选择并授权应用；正文 query、inputs、user；conversation_id 可选，不传或为空创建新会话，后续传响应中的会话 ID；response_mode 选择流式或阻塞（Agent 类仅支持流式）；files 可选。inputs 按应用变量定义，没有变量可传空对象。
- Coze POST /v3/chat：Bearer token 鉴权；正文 bot_id、user_id；conversation_id 位于查询参数，可选，不传可自动创建；新会话须通过 additional_messages 提供用户输入；stream 可选，auto_save_history 默认 true。非流式立即返回执行元数据，需另查对话状态及消息。
- Dify user 是接入方提供的终端用户标识，平台不验证该用户身份；它不是系统 API key 或会话 ID。
- Dify 官方网页与 API 会话身份隔离。本平台已确认要支持外部发起后回到平台继续协作，因此仅借鉴请求模式，不能照搬这项隔离行为。

来源：

- https://docs.dify.ai/en/api-reference/chat-messages/send-chat-message
- https://docs.dify.ai/en/api-reference/guides/get-started
- https://docs.dify.ai/en/api-reference/guides/end-user-identity
- https://docs.coze.cn/developer_guides_chat_v3

### 接口简化建议（待确认）

- 新建和续接可共用一个消息调用接口：不传会话 ID 则创建并返回，传 ID 则续接；不强制客户端先调用独立创建接口。
- 借鉴通用名称 conversation_id，表达此前的逻辑会话实例 ID；如采用该名称，应替换此前 instance_id 建议，不能两套并存。具体字段名尚未批准。
- 首版只保留选择 Agent、终端用户标识、当前输入、可选会话关联和必要返回方式；配置变量、文件与其他参数按实际使用场景增加。
- 调用系统身份由本平台凭据识别，用户标识在该系统范围内解释；同一会话在 API 与平台页面使用统一记录和授权，不用外部传入 Runtime 或原生执行器 Session ID。

本轮未实施接口，也未将建议登记为已接受合同。


## 2026-09-30：会话与运行状态查询研究

- 用户确认上一轮需求基本明确，并询问 Dify／Coze 是否提供查询实例及 ID 是否被占用的接口。占用可能指 ID 存在或会话正在执行，未将两者混为同一状态。
- Coze GET /v1/conversation/retrieve 使用 conversation_id 查会话元数据；官方说明仅支持本人创建的会话。
- Coze GET /v3/chat/retrieve 使用 conversation_id 与 chat_id 查单轮执行状态，包含 created、in_progress、completed、failed、requires_action、canceled。其发起对话接口明确同一会话只能有一个进行中的对话，否则报 4016。
- Dify GET /conversations 按 user 列出会话；GET /messages 按 conversation_id 与 user 读取历史。会话列表示例 status=normal 不能当作正在执行或空闲的依据；本次核对的 Chat API 未发现与 Coze 单轮执行详情等价的查询接口，不以此断言 Dify 其他 API 均无状态查询。
- 建议本平台提供按 conversation_id 查询会话详情及当前执行状态的能力，不暴露内部 Runtime；查询只读，不等于预留或锁定执行权，接收消息时仍需原子处理并发。具体运行中消息策略保持 D-003 待确认。

来源：

- https://docs.coze.cn/developer_guides_retrieve_conversation
- https://docs.coze.cn/developer_guides_retrieve_chat
- https://docs.coze.cn/developer_guides_chat_v3
- https://docs.dify.ai/en/api-reference/conversations/list-conversations
- https://docs.dify.ai/en/api-reference/conversations/list-conversation-messages

接口能力为研究建议，未实施，未定稿状态枚举。


## 2026-09-30：实验前恢复 R-001 的三个未回答问题

- 用户原话：可以先实验一版，询问是否还有需要澄清的事项；明确调用端通过会话 ID 锚定实例，并能跳到对应平台对话页继续处理。
- 已确认用户旅程：平台返回会话关联标识与可打开的对话页入口，外部调用和平台网页接续同一会话。具体链接字段名未预决。
- 恢复已有批次 R-001，仅重新呈现 D-001 至 D-003；不生成新批次或冒认选项已被接受。D-004 错误角色框架撤回，正式标记 Superseded。
- 本轮对 D-001、D-002 的措辞作实验范围解释，未变更问题语义；选项推荐仍为建议。
- 实验成功验证建议：API 发起、获取 ID 和页面入口、网页读取同一会话并回复、接口继续、服务重启后仍可读取和接续；具体执行器续接能力须实测，不仅检查数据库记录。
- Coverage 本轮收口：调用入口、会话关联、页面接续基本明确；执行位置、配置入口和运行中输入未关闭；访问策略和其他生产能力不凭实验授权默认为已确定。

### 本次重新呈现的问题（按顺序）

**D-001｜执行位置**
- A．先在平台所在机器执行，实验先用本机（推荐）。
- B．首版就支持接入独立执行机器。

**D-002｜配置方式**
- A．常用项在页面配置，Skill、Hook 等高级项保留原生配置入口（推荐）。
- B．首版主要导入或编辑原生配置文件，页面只管调用和会话。

**D-003｜运行中追加消息**
- A．保存排队，当前一轮结束后处理；另有停止按钮（推荐）。
- B．默认打断当前执行，立即处理新消息。
- C．调用方每次选择排队或打断。

可以回复“全部 A”，或逐项调整。拿不准的项可以标“待定”，它会保留为未决，相关实验行为不冒认已获确认。

- Dependency check：执行位置、配置入口、运行中输入互相独立，均为原批次未答问题。
- response_received：本次重新呈现前无选项答案。
- resolved_ids：none。
- unanswered_ids：D-001, D-002, D-003。
- write_verification：已核验 Checkpoint 活跃问题、D-004 Superseded、三项原问题仍未答，以及本次呈现原文。


## 2026-09-30：R-001 回答映射与执行位置解释

- 用户原话：“那个D001没明白啥意思啊？你说的是那个环境调试吗？其他默认就行”。
- 映射：D-002 A、D-003 A；D-001 无选项答案，用户请求解释。D-004 已撤回，不纳入“其他默认”范围。
- 本次只解释并恢复唯一未答问题 D-001。
- 解释：A 是 Go 平台与 Codex／CC 在同一台机器，平台启动本机执行器；B 是平台在机器甲、执行器在机器乙，需要远程执行接入。该问题不涉及调试环境与生产环境划分，也不意味着每个用户需要自己的机器。
- 唯一待回答的问题：第一版先把平台和 Codex 都跑在当前这台 Mac 上，可以吗？推荐 A，只建设本机执行闭环；远程机器接入后置。可以回答先本机、需要远程或待定。
- D-002 与 D-003 的方向选择已确认，配置核验细节及停止后队列处置作为后续规格缺口保留；不宣称整体澄清已完成。
- 回读核验：active_question_ids 和 unanswered_ids 均为 D-001；D-002、D-003 记录用户原话、A 映射及 Provisional 缺口。


## 2026-09-30：确认同机本地执行，不建设远程执行能力

- 用户原话：“这肯定是本地的，本地执行的，都在同一个机器上的。不会有那种远程调用的需求。”
- 回答映射：D-001 A，且范围进一步明确为不需要远程执行能力；不写成已计划后续支持远程 Worker。
- 平台与 Codex／CC 等执行器在同一台机器，平台启动本机执行进程；多个会话共用本机执行容量并由平台隔离各自上下文。
- 去掉当前设计中远程 Worker、机器注册、执行机器心跳与跨机器调度；也不以所谓扩展点把这些能力提前写进对外合同。
- 语义区分：用户否定的是平台到远程执行机器的调用；既有外部系统 API／Webhook 接入和通过对话页继续工作仍成立。
- 验证：D-001 Accepted；D-002/D-003 已映射 A 并保留规格缺口；D-004 Superseded；原批次没有未回答选项，整体需求记录保持 Active。


## 2026-10-01：实验 PRD 草案交付

- 用户在确认同机本地执行、其他选项采用默认方向后要求继续。
- 已整理 `prd.md`（Draft）和 `review.md`（Pending User Approval），供范围审查；没有将草案建议登记为 Accepted。
- 草案覆盖配置、调用创建、API 与网页接续、运行中排队、停止、日志与产物、失败与重启恢复。
- 待审建议：首轮接 Codex；API 与一种通用 Webhook 格式；内部简单登录和独立调用凭据；停止保留队列且明确继续、会话配置稳定、手工清理。
- D-002、D-003 的已选择方向保留；细节在草案中以建议默认列出，仍未批准。
- 未实施产品代码或原型，未将需求标记为 Ready for Architecture。


## 2026-10-01：实验需求收口与实施授权

- 用户授权进入架构设计与连续实施，除特殊阻塞不再停顿；后续确认当前没有其他需要澄清的问题。
- 本版采用 PRD 的实验默认值：Codex 首接、通用 API/Webhook、本机执行、内部简单登录与独立调用凭据、运行中消息排队、停止后明确继续、会话配置快照和手工清理。
- D-002/D-003 的规格缺口已收口，不再维持 Provisional；D-004 的错误团队角色框架维持撤回。
- 原生恢复、Skill/Hook、停止、重启和跨来源隔离必须用真实证据验证；当前验证结果见 `../03-delivery/verification.md`。
- Workflow、蜂群、共享记忆、渠道专用适配、其他原生执行器不属于本次完成承诺。
