# Agent Platform：研发任务接入与交付使用手册

本手册面向提交需求、管理项目和验收结果的人。按任务选择起点，保留同一任务的需求、原型、代码、测试和交付关系。产品是否好用与流程是否执行成功分别验收。当前验证事实以[工作流验证记录](../03-delivery/workflows-verification.md)为准；“配置已安装”不等于该路径端到端通过。

## 选择任务路径

| 你手里有什么 | 提交什么 | 工作流应怎样接续 | 看什么结果 |
|---|---|---|---|
| 一句话需求 | 用户、问题、期望结果；不知道的可留给澄清 | 接单判断→必要澄清→需求/AC→交互与架构设计→研发→固定测试→独立QA→PR | 先核对需求与交互，再验实际页面；不能只看PR存在 |
| 已有原型，业务规则还不完整 | 原型包、目标、缺口、哪些决定已确定 | 接单判断→补必要需求/AC→设计→研发；已有原型继续作为体验基线 | 材料下载/摘要、阶段引用、逐页对照和完整用户旅程 |
| 已有原型、PRD和AC | 冻结版本包、批准状态、现有代码基线；可以明确“从设计接续” | 接单判断→核对资料足够→设计或直接研发；缺结构契约不能直接跳研发 | 不重复产品审批，不删除原型动作，不私改规则 |
| 明确Bug/局部配置修复 | 复现步骤、实际/期望、影响范围和必要日志 | 准备任务工作区→关联Issue→接单判断→研发→相关测试→独立QA→PR | 有原故障复现、修复和回归；不机械重做整份PRD |
| 只做Code Review | 仓库、准确PR/head/base、评审目标与规则 | 当前可配置只读Agent会话；完整交付图的QA不是单独Code Review模板 | 同一版本的分级发现、位置、影响和建议；未经授权不修改代码或发评论 |
| 验收后部署 | 已验PR、合并权限、目标环境、精确main版本 | 合并→GitHub Actions自动prepare→授权后手动deploy→页面验收 | merge SHA、Actions/Deployment、health与效果URL一致 |

自然语言指定起点是意图，Agent仍须核对所需资料。不要把平台通用`start_nodes`当作“跳需求”按钮：它可能同时跳过工作区准备和Issue关联。信息不足时，在同一Issue或当前会话回答必要问题；不另建重复任务。

当前标准研发模板没有独立“原型Agent”节点。设计阶段有交互设计责任，但“一句话需求→自动产出可运行原型→浏览器核验→研发”尚未作为完整独立路径验收。当前先验证人提供原型的接续路径，再按质量证据优化上游模板。

## 项目管理员：一次接好项目

每个项目安装一套仓库配置和工作流；维护源、安装器、阶段Skill和共享工具复用。不是把某个测试仓库写死在平台引擎里。新仓库需要自己的repository、prefix、manifest和专用任务工作区根。

1. 准备运行中的平台、原生执行器、平台管理账号、可访问目标私有仓的GitHub认证、Git提交身份、Python/Node/Git/Trellis及项目验证入口。
2. 在“智能体”中核对基础Agent的执行器、模型和权限。安装器继承executor/model，按阶段重新生成指令、Skill和工具绑定。仅“挂上Skill”不证明执行过它；需求、设计、研发、QA各阶段须留下实际读取和产物证据。
3. 用[标准安装器](../../examples/platform-workflows/README.md#安装)注册六阶段Agent、Connector、共享浏览器和研发图。明确项目验证argv，例如`["make","verify"]`；完整测试预算也由管理员配置。
4. 打开“智能体编排”，找到项目工作流，核对节点、回退线、通知Hook和调用授权。共享浏览器工具是同一注册能力，不必每个项目各复制一份。
5. 完成GitHub入站安装；部署另按下述发布路径配置。研发图的终点默认是草稿PR，不能以Run结束表示部署完成。

安装参数示意，全部占位符先替换。不要在命令或JSON里写真实密码/Token：

```sh
python3 examples/platform-workflows/install.py \
  --platform-url <platform-url> \
  --base-agent <existing-agent-id> \
  --authorized-user <runner-platform-user-id> \
  --repository <owner>/<repository> \
  --prefix <project-prefix> \
  --workspace-root <dedicated-task-workspace-root> \
  --skill-root <shared-harness-skill-directory> \
  --qa-skill <qa-skill-directory> \
  --test-command-json '["make", "verify"]' \
  --evidence <private-evidence-directory> \
  --manifest <private-installation-manifest.json> \
  --github-config <private-ci-config.json> \
  --github-token-file <existing-platform-api-token-file> \
  --prepare-browser
```

平台登录密码使用安装器的环境变量机制。平台服务端GitHub令牌和入站平台API Token职责不同：前者供受信Connector访问GitHub，后者供Runner调用平台。GitHub令牌不交给研发Agent；API Token归属被授权平台用户。为Runner选择专用平台调用账号/既有授权账号，核对它可使用本工作流、Agent和Connector，再为该账号创建API Token；安装时通过authorized-user登记同一账号。私有配置和安装manifest不提交产品仓。依赖版本、token权限和预安装命令以安装说明为准。

项目接入检查还应确认实际基础Agent模型、全部Skill路径、Runner依赖、仓库权限、Git网络路径和固定测试预算；初次接入证据不外推另一个仓库。

阶段指令来自维护源`examples/platform-workflows/prompts/`，阶段Skill由安装器指定目录，工具在平台注册并逐Agent绑定。项目已有公共规范继续原位维护，阶段产物遵循Skill，放在`docs/workflow/runs/<run_id>/`；下一阶段消费真实artifacts路径，不猜固定文件名或每阶段都改同一README。

调整通用行为应先改维护源，再沿原manifest升级；在页面改已安装对象可能触发安装漂移保护。模型、Skill和工具变更前核对在途任务；脚本/Skill磁盘内容不是数据库冻结快照，应按不可变版本目录部署并保留旧版本。具体恢复见安装说明。

## GitHub怎么接

在平台主机用上述安装导出私有CI配置。按当前SDK的安装说明准备独立Runner环境；正式入口依赖此环境，不能只放一个YAML：

```sh
python3 -m venv <platform-source>/.data/runner-venv
<platform-source>/.data/runner-venv/bin/python -m pip install -e <platform-source>/sdk/python
```

在目标仓库checkout运行：

```sh
python3 <platform-source>/examples/platform-workflows/install_github_entry.py \
  --project <project-checkout> --repository <owner>/<repository>
```

将生成的`.github/workflows/agent-platform-entry.yml`和来源manifest经正常PR合入默认分支。设置仓库Actions variables：

| Variable | 值 |
|---|---|
| `AGENT_PLATFORM_WORKFLOW_ROOT` | 已验证的平台维护源/版本目录 |
| `AGENT_PLATFORM_WORKFLOW_CONFIG` | 该项目私有CI配置文件路径 |

先核对既有自托管Runner的注册范围和在线状态，再复用可访问该主机且可为目标仓执行任务的Runner；相同标签不会给未授权仓库增加访问。当前分发模板标签为`self-hosted/macOS/ARM64/he-full`，运行入口使用维护源的`.data/runner-venv/bin/python`。不同Runner环境需在模板维护源适配并经原安装器分发；不因接另一个项目就默认新增Runner。

现在由仓库Owner创建的Issue和评论触发；原触发者与重跑者均核验Owner身份。普通协作者、企业微信、钉钉入口不属于当前已实现信任契约。CI只转发事件，平台负责阶段接力；Actions绿灯表示已转发，不表示研发通过。平台回写的通知带标记，入口忽略它们，防止回环。

Issue成功接单后会出现原Run链接。Runner/网络失败时先看该Actions的错误与平台事实；支持的重试入口是同一Workflow的手动运行，输入原Issue号及必要的原comment_id。运行中的任务先保留现场，不靠重复点击获得“更快执行”。响应未知时先用原CI调用账号按原事件键查询；request键查询按平台调用者隔离，换成管理员账号得到404不能据此认定未接单。管理员可沿原Issue接单评论的Run链接核验。不删除入口状态文件或重建Issue绕过去重。

平台网页也可发起任务并创建Issue。后续渠道可调用现有SDK/API，不需要重做研发图；每个入口仍须有自己的身份核验和稳定事件键。

## 路径一：自己有原型，交给Agent继续

### 提交前

把可运行前端、PRD、交互、AC和已接受决定放进同一版本包。PRD拥有业务规则；原型约束页面、动作和状态；AC定义正式产品的可观察结果。明确哪些是演示、哪些必须由后端实现。仅截图或“做成差不多”不足以约束权限、额度、费用及异常。

包内manifest声明版本、入口与每文件相对路径/字节数/SHA。当前材料接收支持ZIP，最多16MiB、256声明文件、单文件16MiB、展开64MiB；不接受穿越、链接或未声明文件。包内脚本不会自动执行。

可上传到同私有仓的有版本Release Asset；正文必须引用精确资产REST ID，不能用可变的latest链接。也支持符合文档格式的GitHub user-attachments ZIP URL，浏览器拖拽上传路径仍需实测；截图可随包提供。localhost仅用于你预览，不能成为Agent唯一输入。

### Issue写法

```text
按材料版本实现目标产品。原型/PRD已确定，普通选择自主推进。
现有基线是<commit>。从设计接续；先核对必要架构契约，再实现。
权威文档为docs/prd.md、interactions.md、acceptance.md。
主要布局、动作及异常不能删；真实联调缺条件如实未测。
```

另加一个且仅一个完整材料块，替换全部占位符：

````text
```agent-platform-material
{"url":"https://api.github.com/repos/<owner>/<repository>/releases/assets/<asset-id>","sha256":"<64位小写十六进制ZIP摘要>","version":"<material-version>"}
```
````

首次接单冻结该描述。修改原Issue不会偷偷换输入版本；新材料要经过明确需求修正，不靠改展开manifest自证摘要。对于临时网络/认证失败，修复环境后通过同Run正式恢复；错误输入必须明确处理，不假装重试能修好错误SHA。

### 验收顺序

1. 原Issue出现唯一Run链接，prepare回执的版本/SHA、实际材料root和manifest一致。
2. 接单、需求/设计和研发交接说明具体承接哪些规则/AC；不能只有“已阅读”。
3. 实现从空状态完成真实用户旅程，页面逐页对照原型，后端权限/持久化/异常均验证。
4. 固定项目测试和独立QA对应同一代码指纹；偏差退回正确阶段并复验。
5. 同一个PR交付；合并、正式部署后版本一致，再从用户页面验效果。

当前Model Relay design-v0.2.0：Release资产→Issue #21→Actions→唯一Run→正式prepare核验19文件→真实需求交接→设计已发生；实现、QA与最终发布在执行。此例尚不是完整通过案例。可点击证据集中在[验证记录](../03-delivery/workflows-verification.md)。

## 路径二：从一句话需求开始

例如：“为团队提供模型服务管理后台，可以管理租户、Key、配额和费用，管理员能完成首次接入，开发者能排查调用。”

Agent先核对用户、结果、范围和已知项目事实。普通实现选择记录推荐并继续；真正影响范围或收费/权限且无法从授权推导的问题才澄清。需求阶段形成可观察AC和首次可用旅程；设计须把这些结果落到界面、状态和架构，而非只给文字菜单。

对上游质量的下一轮验证将要求：从同一需求得出规则与可运行原型；PRD/原型/AC相互追踪；浏览器检查首次使用、主要动作与错误恢复；进入研发时冻结同一版本；QA验成品与原型一致。当前人提供原型的结果用于区分上游需求/设计质量与下游实现/QA能力，不能据这一次试验直接断言全部瓶颈已定位。

## 路径三：修Bug

Issue例：“列表包含已删除项。步骤：新增A/B、删除A、刷新；实际仍2项，期望仅B。请保留既有分页和导出行为，按局部修复处理。”

接单Agent依据现象、期望、影响范围和实际代码判断；明确局部修复可直接研发。结构、存储或公共协议变化仍需要设计；无法复现或期望不明时先补信息。直接开发仍经过分支/Issue准备、相关测试、独立QA与原PR，不跳门禁。

原Run未结束的返工在同Run处理。交付后发现问题可从Run回退到研发/QA，保留旧记录和原打开PR；产品PR已经关闭/合并后的新增工作，明确关联原Issue/Run/PR并形成新任务范围。

## 路径四：Code Review与QA怎样配

Code Review核对实现、规范、契约、安全和可维护性；QA核对用户结果、交互及实际测试证据。编译通过不是Review结论，Review无发现也不是产品验收。

仅评审代码时，可在“智能体”配置专用Codex Agent，选定模型、`read-only`沙箱、只读评审指令和项目适用的评审Skill，关闭不必要网络/提升权限/继承环境，在“会话”选择独立干净checkout及准确PR/head/base。输入明确只评审和输出分级发现；报告可以保存在会话，readonly Agent不为写报告扩大源码权限。该通用配置方式尚未在本轮做完整专用Review验收，不能当现成一键模板承诺。

现有`qa-rework`模板适用于已有产物独立验收与返工，不是只读Code Review：入口为QA，可退需求/设计/研发，修复后重跑tests/QA，最终人工确认。它不创建Issue、分支或PR，要求隔离且可用的工作区，不能与另一个未结束Run共写。单独安装用不同prefix/manifest，不能覆盖正在执行的研发图。

正常研发模板已有独立QA：读挂载验收Skill/SOP、同版测试回执与权威材料；实现缺陷回研发、设计缺陷回设计、业务缺口回需求。是否再增加合并前独立Code Review节点按项目责任配置；目前标准安装器没有专用review-only模板，后续必须以可复用模板/安装和真实评审入口补齐后再更新本节状态。

## 路径五：合并后部署并看效果

先明确谁可合并、哪些环境可部署、是否需人工审批。普通产品仓合并授权不能用于平台维护源。研发Run默认交付草稿PR；部署是另外的真实动作与回执。

复用[本机预览部署安装](../../examples/github/local-preview/README.md)。产品仓由标准安装器生成部署Workflow及来源manifest，正常PR集成，配置目标安装根/固定URL等仓库variables及现有Runner。模型测试仓的正式模式为：main push只prepare；Owner在Actions选择main、目标preview或隔离validation、勾选deploy才发布。

从原Run交付面板核对PR状态、merge SHA、Actions和Deployment；打开效果地址并核对healthz/页面版本。迁移有非空旧数据验证；失败恢复通过可信控制器及正式入口，保留失败和旧版恢复事实。手启动一个前端或临时服务不算部署验收。与配置样例相同的HTTP地址/示例域名也不表示真实供应商联调成功。

## 进度、补充、停止和恢复在哪里

“智能体编排”→项目工作流→该工作流的“运行记录”→单Run→当前Agent会话。每个工作流有独立Runs列表；GitHub原Issue也有同一Run链接。展开命令回执看结果，查看命令日志看真实失败；完整日志按分页/eof核对，truncated明确证据不足。

- 普通补充：在原Issue新评论或当前可接收的Agent会话回复；修改旧评论不产生新指令。
- 交接中的补充：先核对当前节点与接收状态，不能续聊已经交接的旧会话。
- 暂停/人工回退：停止→等待停止完成→选择目标节点并说明原因→正式继续；停止不撤销已发生GitHub或文件动作。
- 通知失败：在通知记录单独重试/复核，不重跑Agent；结果未知先查询实际外部回执。
- 命令失败/超时：诊断副作用和原版本，再从正式回退入口重试；不手改状态或删工作区绕过去重。
- 后续新需求：已完成Run的旧评论不能把它重新打开；使用明确的回退入口或关联的新Issue，依据是否仍是原交付返工判断。

## 留存一次用户试验

每条案例在[验证记录](../03-delivery/workflows-verification.md)保留：用户起始材料与目标、平台/仓库版本、实际操作入口、角色/模型/Skill、材料及代码摘要、正常与失败恢复、当前PR/部署事实、页面证据、发现的流程缺陷和标准修复版本、仍未测的限制。手册只将已经验证有效的步骤写为实测操作；计划和未完成能力明确标注。

本轮退出不以Model Relay有多少页面判断。需要同时证明用户能选路径、按手册接入自己的仓库、获得可复核交付，以及典型副产品具备与PRD/原型一致的功能、UI和成熟度。

## 查看节点输入及判断为什么等待

进入“智能体编排 → 工作流 → 节点设置”。“角色使用的 Agent”保存共享角色职责和权限；编辑共享 Agent 会影响引用它的新会话，当前冻结会话不被替换。“Session Prompt”是当前节点工作说明，任务及用户修正由平台追加。自主模式将 `{{handoff}}` 展开为合法目标、条件及应携带信息，固定模式采用明确完成信号。预览只读，修改应回到 Session Prompt 和目标策略。连线与列表是一份数据。

运行记录 → Run → 对应节点“查看实际会话输入”，可核对冻结角色、项目规则发现方式、工具及首轮 User Input。后续澄清在原会话追加，不再重灌全部上下文；中途修正按消息来源传给后续节点。

新的标准安装区分四种情况：

- 当前 turn 结束且未交接、无明确阻塞：沿原会话自动继续，受次数与期限限制。
- 明确等待澄清/外部阻塞：显示具体问题，用户回复后继续。
- 已接受完成/交接：本轮结束后按冻结拓扑推进一次。
- 用户停止或执行失败：保留现场，通过 Run 的继续/回退入口显式处理，不自动重放未知副作用。

默认最多自动继续 3 次，期限从本节点开始计 14400 秒；达到限制时 Run 仍未完成。可在节点设置调整，0 关闭对应限制；失败不进入自动继续循环。旧 Run 不自动转换输入协议，在实际输入窗口显示旧版兼容提示。

此路径已用真实 Codex 验证自然收尾、澄清、用户修正、需求→设计→研发和固定完成；详细版本与其余失败范围见 [验证记录](../03-delivery/workflows-verification.md)。这不表示 Model Relay 产品完整验收已通过。

节点新增“允许请求用户输入”。打开时 Agent 可用 wait_for_input 提出具体问题或说明外部阻塞；关闭后工具不暴露且服务端拒绝调用。可把研发/批处理节点关闭，配置合法 handoff 指向允许澄清的节点。旧新版定义未保存该字段时保持此前开放行为；改为关闭后保存，再启动的新 Run 才采用关闭配置。

等待原因要分开处理：澄清进入原会话回答；外部阻塞先解除条件再说明恢复依据；自动继续上限/关闭表示节点未完成且推进暂停，没有新增问题可答。检查现场后在原会话提供具体继续说明，或停止后回到合适节点。执行失败走正式恢复/回退，不把失败当普通回答重试。

用户回复接受后旧等待清除，旧轮次凭据不能向新一轮提交等待或交接。主动丢弃的排队消息不再传给下游；投递失败/不确定消息仍保留记录，恢复时请明确有效范围。预算属于本次节点，不因一条新消息重置。
