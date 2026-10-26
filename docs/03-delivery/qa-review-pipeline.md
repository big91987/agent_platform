# QA feedback loop and independent code review

> 当前（2026-10-26 10:10UTC）：用户授权的长窗口CLI补验cb5547cf已complete/13-of-13/0发现，命令16分51秒、2641841 token；7件原件释放后复取一致。原PR71版本不变，旧四候选此次未复现，两处已确认基线弱点仍未修复；完整执行不等于审查质量已证明。详见文末长窗口结果。

## Agreed behavior

The external CI integration owns the product flow. Agent Platform remains generic.
Requirements, design, development and QA each keep their own conversation while
sharing the task branch/workspace. On a product defect, QA chooses requirements,
design or development and submits evidence. A new Workflow Run resumes that role;
all progress stays on the original Issue. Environment failures do not become
product failures. QA must explicitly select its destination; missing target_stage
is rejected rather than defaulting to PR delivery. Required requirement/design changes go through human review.

After passing QA, the human confirms draft-PR delivery. Code review is a separate
manual workflow entry after PR creation: an independent Agent posts its findings,
and humans decide what to change and whether to merge. Review has no handoff tool
and is not an automatic gate in the QA repair loop. No product PR auto-merge.

## State and recovery

The trusted local registry stores the active stage, round, stage invocation
receipts and last accepted handoff. QA return advances the round, so old callbacks
cannot use previous QA evidence. Immutable UTF-8 document snapshots preserve
handoff evidence; the persistent task worktree holds code, images and current docs.
The same-role native conversation resumes with new input, not a different role's
history. CI concurrency and a per-Issue file lock serialize dispatch consumption.

The sender retains an immutable receipt before dispatch. Accepted requests are
not replayed; rejected/uncertain requests may be explicitly retried unchanged.
The receiver deduplicates using the accepted transition and stable invocation
request ID. Rework Issue notices include the return round, so an identical
unresolved defect remains visible on a later loop while same-round retries deduplicate. It rejects an old callback after the task has advanced. A new Agent
input cannot be inferred merely from a green CI run: Agents may wait for people
or run beyond the CI observation timeout.

Human acceptance is currently enforced through Agent instructions, not a separate
machine-verifiable approval record. The receiver validates stage routing, document
integrity and product digest. Same-machine shared storage is required; no claim
of distributed sandbox migration or remote storage replication is made.

## Verification so far

- Complete local verification script passed: existing web and SDK checks, eight
  focused Python integration tests, MCP HTTP retry/routing tests, Go vet/race tests.
- Simulated integration checks cover QA return to each permitted role, preserved
  conversations, re-execution of subsequent stages, stale/repeated callbacks and
  publication without a code-review gate. These are not real Agent E2E evidence.
- Test-repository workflow maintenance PR #81 merged at
  `889ad317193998f93fe096b72959a2d438d18b21`.
- Earlier real Issue #80 QA passed AC-01–07 with 14 browser runs and 218 actions,
  and remained idle awaiting confirmation under its original configuration.
- The real product run completed the forward path through draft PR delivery.
  QA found no product defect, so actual Agent-selected backward handoff remains
  unobserved. The three return routes and repeated cycles are covered by the
  integration tests above; do not describe them as a real failed-QA E2E run.

## Current real run

Product Issue #82 (undo the most recent book deletion) uses one task workspace
and distinct requirements/design/development native Codex sessions. The operator
answered the name-conflict question, approved PRD, reviewed the running prototype,
and approved the revised design through the platform webpage. Requirements,
design and development dispatches are Actions runs 37002535752, 37003725865 and
37007980863 respectively.

The operator exercised the actual product through the browser: restoration keeps
read state, list order, active filter and another book's unsaved editing draft;
consecutive deletions restore only the latest; duplicate-name refusal preserves
the opportunity and succeeds after renaming; restored state survives refresh.
These are operator observations, not an independent QA result.

Development reported 12 real browser plans / 517 actions and nine unit checks,
including a narrow-screen long-title overflow that it found and fixed. Its shell
verification encountered a sandbox listen EPERM. The same standard verify.py
command subsequently passed in the Runner environment, including core and feature
browser plans, and saved runner-checks evidence. The operator read validation.md
and confirmed handoff to independent QA through the webpage. The registered MCP
dispatched Actions run 37010994014 and created QA conversation
`1b99ef5d9649c895189ae5a47cd9cab3` in the same workspace. Independent QA executed 16 real browser runs / 615 actions and found no product
defect. Its accepted report preserves untested system IME/screen-reader scope.
The operator reviewed the report and approved only draft-PR creation through the
webpage. QA called submit_handoff with target_stage=report; Actions run
37013718462 succeeded and delivered draft PR #85:
https://github.com/big91987/reading_list/pull/85

The final Runner repeated all six standard checks successfully. Delivery head is
`aacc594904bbabbb8d429bd5df1123d59b0e4038`, on `codex/issue-82-platform`; the PR is
open, draft and unmerged. Four distinct native Codex sessions were confirmed for
the four roles, and the product digest remained unchanged throughout QA.
No real failed-QA return occurred; no claim of actual backward-loop execution is
made. Independent code review was validated separately on Issue #74 below, not
claimed as a review of PR #85.

## Browser acceptance prerequisite

The undo feature includes failed localStorage writes and subsequent retry. The
shared browser tool now supports `storage_write_failure` with a boolean `enabled`.
The fault affects only current-page localStorage.setItem and ends when disabled
or reloaded. It never seeds storage or changes product code. The existing real
Chromium tool probe covers failed write, unchanged saved data, recovery and actual
readback, with Escape and Unicode download checks retained. These synthetic-page
checks establish tool behavior, not product QA success.

Source PR: https://github.com/big91987/he_skeleton/pull/39 (open for review).
Test repository maintenance PR: https://github.com/big91987/reading_list/pull/83
(merged). Formal install_light.py synchronization pins source commit
`09d1aeb530a99e25f84db0453a45aae306b612d1`; reinstall reports no changes. The trusted
Runner tool checkout was fast-forwarded to the merged test-repository main, and
standard external MCP discovery refreshed the supported-action description.


Independent review entry was dispatched separately for existing test Issue #74 /
draft PR #75 in Actions run `37006210086`. It created reviewer conversation
`d9ba5ae8b1a7768eb3bc37ab2b2125f1`; the browser shows the Reviewer inspecting actual
code and test differences with no handoff or merge responsibility. Review completed and Actions succeeded. Its findings were posted to PR #75 at
https://github.com/big91987/reading_list/pull/75#issuecomment-5952332426 and the
original Issue. No proven implementation defect was found; remaining acceptance
gaps were explicitly left for human judgment. The product digest is unchanged,
and no handoff/merge was triggered. This does not imply the older QA blockers
were resolved.

Test-repository PR #84 merged the matching Actions form description for explicit
QA destinations. Missing-target sender/receiver checks and two-cycle Issue notice
checks extend existing tests; no extra test family was introduced.

The standard platform verification script passed again after these changes:
web checks, SDK tests, eight pipeline integration tests, Ruff, Go vet/race and
build. This does not replace real QA feedback-loop acceptance.

QA initially treated untested system IME/screen-reader claims as mandatory gates.
The operator requested a clause-level review through the webpage, without
waiving a required criterion or inventing test evidence. QA reread the accepted
PRD/contracts and corrected these to explicitly untested scope, not blockers.
External QA instructions now require every release gate to trace to an approved
clause. This rule belongs to the integration, not generic platform code.

The reusable QA instruction correction was deployed through standard setup.py;
Ruff and the eight integration tests passed after this prose-only change.


## Ready-for-review entry verification

The owner marked product PR #85 ready at 2026-10-02 14:02:11 UTC, but no review
started: the installed workflow only listened to issues and workflow_dispatch.
Maintenance PR #86 added pull_request.ready_for_review and was merged at
0286bd39f254a1d8bc54fbb402a5adafdece1b7b. Nine integration tests pass, including
trusted delivery-record resolution from PR #85 to Issue #82 and rejection of a
wrong branch, commit, fork or uncommitted product files. Review start now posts
its conversation link directly on the PR.

Replaying Ready through the official GitHub CLI did not generate a Run for the
existing PR. Its merge ref 06220e05ff7c565e4a1847ee111db2929c647ca7 still uses base
be597911480d2c836ee694149e81cb5ef5b4c8e6 and does not include the new listener.
The browser connection timed out, so this is CLI event evidence, not a successful
webpage click test. A proposed default-branch pull_request_target listener was
blocked by automatic approval review as a security-boundary change; it has not
been applied and user approval is pending. Native-button end-to-end verification
is therefore incomplete; do not report it as fixed.

The existing manual code_review entry successfully started an independent review
of PR #85 in Run 37019371767. Conversation 933ee7f2080084cb2e97500f2c271bac and
its start notice are visible at:
https://github.com/big91987/reading_list/pull/85#issuecomment-5954551448
Review was running at this checkpoint. Product PR #85 remains open and unmerged.


## General backward handoffs (2026-10-03)

All product stages now use the same submit_handoff tool for the normal next
stage or any earlier stage. Requirements has no earlier stage. Explicit user
requests are sufficient; Agent-proposed returns require human confirmation,
except evidenced QA product defects. Review is still separate. The sender,
receiver and workflow agree on destinations. Every backward transition starts a
new round, retains role conversation IDs, and rejects obsolete callbacks.

The six earlier-stage routes pass integration tests covering continuity, retry
and invalidation. The full platform verification script passed. Test-repository
maintenance PR #88 installed the workflow routes; the local tool was rebuilt and
setup.py refreshed the standard stage configurations and tool discovery. Existing
conversations preserve their instruction snapshots; new configurations apply to
new conversations, while the shared tool binary is used on later invocations.
This is not a claim of a real Agent-selected failure/repair/QA cycle.
# 2026-10-26 · 独立代码审查接入实施计划

目标：以同一代码范围比较完整 Open Code Review CLI 与 Codex＋官方 Delegate Skill；研发图复用执行节点，PR 提交后由独立 GitHub Workflow 触发。人工合并保留，先不更改分支保护。

采用既有执行节点/Agent 节点与 OpenSandbox；不增加平台节点类型或业务调度器。CLI 固定官方 v1.13.3，二进制校验 SHA256；Skill 来自同一标签并保留许可证。审查使用受信模板的规则，显式包含测试文件。输入固定 repository/base/head；研发尚未提交的候选先以文件快照记录，审查前后验证来源不变。报告与原始输出按 Run 保存。完整、发现问题、不完整和执行错误分别处理；exit0、无评论、部分审查均不能直接当业务验收。

CLI 使用用户指定的 OpenAI 兼容模型服务 glm-5.3-flash；密钥仅私有引用且只进入 CLI 节点。Delegate 使用现有 Codex/gpt-6.1-sol，不继承 Git 或 OCR 凭据。PR 入口仅读可信基线中的适配器，不执行 PR 中的代码、工作流、安装脚本；拒绝跨仓及不受信作者。请求以 PR/base/head/审查方式去重，旧 head 不可证明新 head 通过。

## 任务与验证

- [x] `examples/code-review/runtime.py`、`runtime_test.py`：版本安装、候选准备与范围核对、CLI 报告规范化、覆盖/警告/未知严重度拒绝通过、原始输出和报告保存。先写失败回归，再实现；真实二进制验证其 JSON 契约。
- [x] `examples/code-review/workflows.py`、官方 Skill 与规则：生成独立 CLI/Delegate 两张图；可在研发图 tests→review→QA 复用 CLI 节点。Agent 只写报告；固定后处理核覆盖、版本和源文件未改。
- [x] `examples/code-review/pr_entry.py`、`code-review.yml`：可信 PR 入口、精确 SHA、幂等提交、终态检查与 GitHub 可查看结果。正常、旧 head、fork/非授权、运行失败、部分审查和重复请求回归。
- [ ] 标准导出/项目内复制及112正式 API 安装新图；现有冻结 Run 不改。用真实 PR 触发 CLI 与 Delegate，核原生消息、同版报告、执行时间和释放后产物留存。真实调用失败保留首回执，不造通过、不盲重试。
- [ ] 比较可复核发现、误报/漏报边界、覆盖、耗时与可得 token；未验项目单列。将入口、配置与升级文档纳入模板，不依赖开发者临时脚本。

当前：实施中。已有产品 PR68 实时为 merged，不沿用旧 draft 快照；不得把后续审查称为合并前已执行的检查。


### 2026-10-26 · PR 审查正式入口与沙箱边界

唯一维护源 `fe349085e348dd1b51295385f77b9762be4d9fff` 已提交并推送开发分支。标准导出166件校验后通过正式安装/API应用：独立CLI图 `0ddb0eff52aa31a2494df6b434f7bfc8` revision2、Delegate图 `9efca4daff248c3c6914cd68d54acc64` revision3；研发主图revision12已插入tests→code_review→QA，发现问题回development，10条既有冻结Run未改。112平台二进制仍dfecd925，模型配置只引用私有环境名，Git凭据只进入prepare。112是控制面，实际克隆与审查在每Run独立OpenSandbox `/workspace` 执行，不在宿主执行PR代码。首次91.275秒模型Loop为本机隔离探针，不能冒称沙箱PR审查。

[Issue69](https://github.com/big91987/model-relay/issues/69)和[Harness PR70](https://github.com/big91987/model-relay/pull/70)已建立标准入口，PR70精确合main a1cbbb86。首次自动40257071468/40258960641在读取PR失败前未建Run；根因之一是入口漏pull-requests:read。固定分支手动40259093279又暴露GitHub checkout真实超时/HTTP2中断，以及Delegate模板要求externalSandbox不支持的单节点禁网而HTTP400；两者均无Run，不归为模型审查失败。维护源补PR读权限、安全错误分类、CI只稀疏取项目工具包、模板遵守现有沙箱网络契约，并新增Go真实图验证回归；不扩大冻结Run权限。旧失败保留，不盲rerun旧基线。

[PR71](https://github.com/big91987/model-relay/pull/71)当前draft/open、head25ba454995db22cb2a4c41df1a5f626ff60d0be6，包含同版标准副本及入口修复，尚未合并。pull_request_target仍使用main旧YAML，40261623285两项失败不能当修复版结果。修复后仅一次正式分支dispatch [40262132950](https://github.com/big91987/model-relay/actions/runs/40262132950)，2026-10-26T03:03:41Z受理唯一Delegate Run `1c3c7d6c2f0d111a7bfdec6f8b241f81`，沙箱ready后prepare运行；CLI仍待Runner队列。此时尚无真实PR审查结论、同版报告或释放复验；不得称两种集成完成或产品QA通过。

管理传输曾因通用旧回执被误认造成后续命令落入Shell，未完成升级；已确认归档1658880bytes/SHA fd1ef25e…完整，改为精确摘要回执及私有文件执行，正式安装完成。该管理错误和入口/队列等待计入工程交付耗时，不计Agent净审查。定时任务继续暂停，未启动旧产品任务或部署。


2026-10-26 03:24UTC追加实测：正式CLI Run bf2aea6f0d536f404e4acba2a69906fb于03:18:21Z completed，但审查为 **incomplete/No-Go**。准备92.690秒；实际OCR命令626.512秒、7件完成3件/失败4件，原生partial，537137总token（input516832/output20305/cache426240为input子集），超过500000聚合预算且grace round超时。零发现不能当通过，不上调预算掩盖；7件1115521bytes正式原件已全部重新下载并核size/SHA，CI失败正确。Delegate原Run准备120秒fetch超时，没有Agent；实际CLI同版准备成功，网络波动/历史体积/固定120秒共同候选，未证明单一网络根因。正式review准备在600秒节点预算内允许fetch480秒且不自动重试；OCR依赖merge-base，保留历史，不浅拉破坏比较范围。

独立复审另发现可信Python以-c在PR目录启动，可执行同名hashlib.py，且sitecustomize在代码前启动；此为Harness安全缺口，不是Model Relay产品缺陷。最终修复在标准可信模板使用普通command执行正确shell引用的python3 -I -c，覆盖准备、CLI、Delegate校验和研发审查；平台自由代码节点不强制隔离。真实恶意同名模块/启动钩子及$()/反引号/引号回归红→绿，19审查测试、28sandbox定向经独立复审通过，无剩余Important。可信摘要另按实际ConnectorReceipt.output混合日志读取唯一64位摘要、exit0且日志未截断；原stdout字段不存在，已用真实捕获的无换行拼接形式复现再修，不保留旧兼容分支。

最终scripts/verify.sh真实exit0，136271bytes/SHA9bd524f128e4e612f34aa63d1c52cb19b76753ae3d5c2f0ad38ef95f618c74f4，Go部分缓存如实保留；中途ruff导入排序失败日志保留。新增手动engine选择用于单独验证一种方式，自动PR仍双路，不为单路诊断重复消耗另一模型；YAML实际校验另列。上述新修复尚待精确版本标准安装/新head入口，不能称最终整链已绿。[机器人Issue反馈6407484385](https://github.com/big91987/model-relay/issues/69#issuecomment-6407484385)明确报告原始阻塞与结论。旧冻结Run不改，安全风险未因当前PR无同名模块而泛称已消除。


2026-10-26 03:35UTC：安全修复328dee78通过标准导出166件应用，正式API核CLI3/Delegate4；主图仅未来Run升级至13，prepare/code_review均python3 -I。脚本曾停在历史waiting检查，自动审批拒绝忽略在途任务；只读查明该Run始终冻结revision4而主图已12，针对冻结设置回归真实0.735秒Pass，补管理员全量精确摘要后获准正式升级，十条旧定义全部保持。中间非管理员脱敏摘要与管理员全量摘要不一致造成检查停止，未PUT/未改旧Run；风险核验不能记为Agent净审查。两条旧审查环境均released，CLI七件1115521bytes释放后新目录下载全size/SHA一致；释放回复一次reset后先查实已released，没有盲重发。

项目PR71新headd398a4908f63f8e4ac4677acf11f93293d9bcd16/tree79370b59dac31048018e4c5aaa9a6787b090500c，168远端Git blob与源328dee78标准副本逐字节相符，GitGuardian success。旧base自动40264270036仍失败，保留。仅一次选delegate正式dispatch [40264483828](https://github.com/big91987/model-relay/actions/runs/40264483828)→唯一Run [2db0e54e3cf8084c39e2d5bd10257df0](http://10.1.20.112:8795/workflow-runs/2db0e54e3cf8084c39e2d5bd10257df0)，03:31:33Z创建，prepare65.928秒exit0；03:32:39Z启动原生Codex/gpt-6.1-sol独立审查，未见待审批。原生提示模型metadata fallback/bundled bubblewrap警告保留，不擅换模型/权限。当前未完结，不把会话运行或Skill加载当审查通过。新增手动engine选择已由Actionlint1.7.12真实exit0校验，source push HTTP2失败后HTTP1.1推送成功。定时任务仍暂停；PR未合并、无产品部署。


2026-10-26 03:39UTC：Delegate Run2db0e54e…原生事件71409于03:35:59Z报告Connection failed / responseStreamDisconnected，willRetry=true；此前仅启动及UserInput事件，没有代码读取或报告。模板遗漏原项目显式proxy、只给PATH；CLI及既有研发角色均保留proxy，二者差异已用失败回归复现。维护源Delegate只复用模板显式HTTP(S)/NO_PROXY变量，inherit_env仍false，不带Git/模型密钥；20项审查测试和独立6项workflow复审绿，无新增Important。该配置遗漏与网络失败相符，因果仍待新正式任务实测，不称已恢复。OCR预算阈值按轮检查可超出，README已按真实537137记录，不能宣称500000硬上限。

当前Run仍运行。正式stop请求被自动审批拒绝（未明确授权中止该在途Run），未执行；已通过用户确认选项申请停止/释放再正式安装复验。用户尚未回复前不改冻结图、权限或环境，不换模型，不连续resume，不在宿主代跑审查。其余源码修复/评审/交付准备继续，原网络事件及首轮不完整报告保留。是否允许停止与复验需以用户实际答复为准。


2026-10-26 05:51UTC：用户确认“允许停止并复验”后，先核Run2db0e54e…仍running/seq2，再正式stop→stopping，核stopped后release→released，原网络错误和冻结revision4不变。源b2a4059011c708683028265e8aaf7c36c7f730de经代理Git推送成功；166件标准导出校验/安装器升级Delegate图至5，主研发图仍13，独立CLI仍3。非管理员Agent.env响应是脱敏null，监控脚本误对其get报错已纠正，不能当运行配置缺失。项目PR71更新head1cbd3c9240421c04beffa763420eb41c0009064d/treea1803e7879dac15506fdfe7380c203a4d9ae6009；仅代理/测试/README及来源清单，产品不改。只一次正式branch dispatch选择delegate，Actions40276343113/05:49:47Z pending，等旧base自动检查40276276554，不重复触发。真实网络恢复和新报告尚待。

新增平台限制实证：旧Agent review从03:32:39Z驻留到用户授权停止05:46:38Z，约134分钟，配置execution_timeout_seconds1800没有中断该原生轮次。CI观察40分钟后于04:11:45Z失败也没有停止后端。源码workflow_control.go仅在会话idle后的自动接续检查该值，且ContinuationLimit0先分支；它不是正在运行Agent的硬超时。此为平台执行治理缺口，不能把模板1800称硬时限或把CI停止当Agent停止。当前先修既有代理并取正式审查结果，保留此缺口后续通用整改；没有通过改DB/原回执或延长限制掩盖。明确执行节点的真实命令timeout、Agent接续窗口、CI观察和沙箱TTL是不同控制。


### 2026-10-26 · 用户授权后的Delegate正式复验结论

唯一Run `72d28bd9700af3bb5265ff2acfdc3510`，冻结Delegate图revision5/head `1cbd3c9240421c04beffa763420eb41c0009064d`、base `a1cbbb862714346a77a24c96b2bf5d5672c6be1a`。prepare05:53:26.938–05:54:52.985＝86.047秒；原生review至06:00:13.739＝320.754秒（含工具、推理与归档）；validate至06:00:23.867＝10.127秒；Run合计416.929秒。正式[Actions40276343113](https://github.com/big91987/model-relay/actions/runs/40276343113)于06:00:43Z SUCCESS，dispatch05:49:47Z→完成约10分56秒，其中Runner/并发队列到job05:53:08Z约201秒；不能称全部为Agent计算时间。原生634事件全部分页导出，有24次command_execution完成、一次正式MCP handoff(target=validate) accepted，原生会话closed；不是只看progress。环境代理修复后已有真实模型响应及源码读取，旧网络错误不改写。

Agent原始报告13件全部reviewed、skipped0/comments0；实际Git diff分别读取全部13个选择文件，另有OCR默认排除的README 1件，不算已审查。固定后处理独立核清单/精确range/全部源码摘要未变，生成clean/exit0；这是静态审查及机器完整性校验，不是动态产品QA、无缺陷证明或批准合并。原生单thread单turn累计input2054581/cache1928832/output5120(reasoning1312)/total2059701，cache与reasoning分别为子集；货币费用不可得，不估造。

8件1104788bytes正式原件已逐件size/SHA核验；正式release回复reset后只读查实released，没有重放释放；在全新目录重新下载8件全部一致。GitHub实际附件12630817340/code-review-delegate-40276343113已下载，其中result.json231bytes/SHA f4ebf775252899708ca9655721170b407541dfe963cc482bcfc7008aee470ae5、report.md386bytes/SHA373a1705ece77726a11ae41b1f06f26f15f8781e3f7ab2873726acc6e63f0e62、preview.json2757bytes/SHAe6770691b245b753daf8ffd6745d05a9d463c772bb6a91d314264dc3ce7e9d52，与平台释放后原件逐字节相同。GitHub精确head的Code Review/delegate SUCCESS，旧main配置自动40276276554红仍保留；PR71仍draft，不将手动修复分支通过称为main自动入口已修复。独立CLI图3及研发图13已配置，但本轮没有新的完整研发Run或新head完整CLI结论。

结论与局限：Codex＋官方Skill这一独立PR审查路径完成真实闭环；完整OCR＋GLM此前真实PR为预算partial，7件/较早head与本次13件不是受控同版对比，不能直接算提效、准确率或认定模型优劣。基础服务连通不代表大PR能完整审查。保留CLI500000阈值和未完成失败，不无限加预算。未做已知缺陷集的误报/漏报基准，未做跨仓fresh install及长期并发可靠性验证。产品代码与部署未改。

下一步建议按证据排序：①给Agent正在运行轮次增加真正硬超时，并区分CI观察/接续窗口/沙箱TTL；②模型持续网络重试应由平台生命周期及时反馈到关联Issue，本轮仍依赖Builder具名反馈；③默认读取source.json的base/head/expected而非整份约1MiB摘要库存，减少上下文重复，2.06M总token大部份cached仍不能称免费；④按完全相同代码范围/规则比较两种模型Loop，改善CLI分组及预算内完成率后再决定默认CI策略。当前保留可审草稿，不虚构用户对这些限制的风险接受。


2026-10-26 06:20UTC执行节点补验：用户指出不能只闭环Delegate。通过项目workflow_dispatch engine=cli唯一Actions40279068795，当前PR71精确basea1cbbb86/head1cbd3c92生成Run23f8a9c1705f686933889c09c16cdd41，CLI图revision3；prepare1实际70.572秒，review2普通执行节点已启动。保留同预算500000、相同规则与覆盖约束，对照此前同版Delegate13/13；不拿旧head7件与新head13件直接算优劣，不改main旧自动失败事实，不把独立CLI试验当主研发新阶段整链完成。Issue69评论6409623841明确边界，等待终态后核报告、原件与释放后读取。


补验更新（2026-10-26 06:36UTC）：同版CLI Run23f8a9c1真实failed；OCR命令976.015秒、474570总token（input443375/output31195/cache333888为input子集），13件选中但0件完成，失败包括请求/分组超时与上下文压缩阈值，不是本轮token预算耗尽。comments:null触发维护源报告器TypeError，report.md缺失导致归档失败；原Run不改写。六件1123685bytes仅作为只读诊断私有保存且逐SHA核对，不冒称正式归档。用户要求先放宽时间再同版比较：源码请求300秒/分组20分/进程30分/执行35分/CI50分/job60分，500000阈值和严格覆盖保持；null/畸形finding红→绿、22审查回归/141模板回归/Go契约/vet及独立复审通过。标准升级与新同版双路尚待实际入口回执。

原始raw SHA f17aa24b9ccfaf3f1f029edb363a359030635ef5a1dee35c6e8f672a49c82569，summary.files_reviewed=13不是completed=13；必须读manifest覆盖。实际Run23f8在06:36:50.463Z以report缺失failed，冻结revision3及原始receipt保持；Builder只读docker取文件是诊断恢复，不是平台正式留存。新维护源assess先正规化null/畸形comments，原错误仍error，不因报告生成修复改变模型失败。独立review首轮发现partial/failed早返回绕过坏成员验证，补跨状态失败回归后修正，最终无C/I；141模板19.519秒、Go契约0.796秒、22审查0.572秒、vet exit0。

正式升级（2026-10-26）：维护源639bbe8已提交/推送，标准166件导出核SHA安装；CLI revision4、Delegate6、研发14，十条旧冻结Run逐一不变。PR71新精确heada6e77d582039b4c91e979a528e0b78c88c6dcb80的168远端Git对象与导出匹配；API确认该范围两路均无Run后，原GitHub入口一次engine=both补验延长时限后的两路结论。模型/规则/500000阈值保持，未合并/部署，等待实际Actions与Run回执；Issue69评论6410386755已记录旧失败与新对照边界。


对照执行中（2026-10-26）：当前同版heada6e77d58的Delegate Run275829a3已completed/released，13/13、0发现、固定校验clean；审查419.637秒，八件1104789bytes释放后全新下载摘要全部一致，GitHub作业SUCCESS。首次双路Actions40284625015的CLI因checkout三次直连443/75秒失败而未建Run，不能归模型；仅在项目标准git_proxy字段设置用户已有代理后核无Run，再正式一次engine=cli，Actions40288396722 / Run89dc1f4e6bb8b742ee604b56f2d7467d，CLI4冻结新时限，prepare80.622秒已完成，review07:53:20Z真实运行。旧失败/取消保留，非无限重试；完整结论仍待CLI回执。

补充：delegate原生完整902事件读到EOF，总input2076291/cache1953408/output4958/reasoning1157（output子集）、total2081249，费用不可得。初步曾把旧null报告器缺陷称作Codex漏检，随后核base a1cbbb86的Git blob ccd0f8b0f332285ddd30e31e91ea2c79cd15e9f4（13325bytes/SHA7562ac055756c98eca163e646d9e68ccc88f4525601249740b0feb83484ca4be）确认同一缺陷基线已存在。本轮规则只审新增差异，不能据此判本PR漏检；应表述为差异审查无法替代全库检查或实际执行验证。两边预算/模型不同，因此本轮对照集成方式实际结果，不作等算力模型排名。项目代理配置0600，仅git_proxy值变化，其他字段/凭据不变；公开Git探针直连/代理均一度成功，不把瞬时探针当真实checkout三次失败的反证。


## 放宽时间后的同版对照（2026-10-26）

本轮先放宽执行时间，再比较两个真实集成方式的覆盖、问题及消耗。源代码在比较期间冻结，未为某一侧改断言或删审查范围。测试对象为 Harness 自身的真实 PR71；这不是 Model Relay 产品 QA，也不是等算力的模型准确率测评。

- PR71 base `a1cbbb862714346a77a24c96b2bf5d5672c6be1a` → head `a6e77d582039b4c91e979a528e0b78c88c6dcb80`，tree `1d0fb66d1c831101d9d35314046a134bb0d1df65`，168远端对象与维护源639bbe8标准导出一致。
- Git实际变更14件；OCR默认排除README，明确选择13件。两路source.json完全相同（5667文件库存，SHA `98fb4021169fe6268f840026b11e80897785de1e6fcfc39302a85ecc70360d33`），解析后rules逐字节相同（SHA `329b379966e1b1397dc167251bf9c63e5742a73504821fa4011ca9664d17c1ad`）。两侧候选源码未改。
- OCR固定v1.13.3。CLI请求180→300秒、分组10→20分钟、进程1200→1800秒、执行节点1500→2100秒、CI观察2400→3000秒、GitHub job50→60分钟；模型及500000聚合token阈值保持。Codex使用gpt-6.1-sol与官方Delegate Skill，角色和审批配置未变。

| 项目 | Codex＋官方 Delegate Skill | GLM＋完整 OCR CLI |
|---|---|---|
| 实际 Run | [275829a3](http://10.1.20.112:8795/workflow-runs/275829a3cd7d892c636a0fb2b02aff1c)，图revision6 | [89dc1f4e](http://10.1.20.112:8795/workflow-runs/89dc1f4e6bb8b742ee604b56f2d7467d)，图revision4 |
| 结果 | clean，13/13，0跳过，0确认发现 | incomplete，5/13完成、8失败，4候选；原生partial、固定exit20 |
| 准备阶段 | 69.273秒 | 80.622秒 |
| 审查阶段驻留（含工具/启动/归档） | 419.637秒 | 1118.861秒；OCR命令本身1108.908秒 |
| 固定结果校验 | 独立执行节点9.863秒 | 在同一CLI执行程序中完成，未把partial exit0当通过 |
| Run墙钟 | 498.774秒 | 1199.484秒 |
| 累计输入/输出token | 2076291 / 4958，总2081249 | 1627422 / 50926，总1678348 |
| 缓存输入（输入子集） | 1953408 | 115904 |
| GitHub回执与报告 | [成功作业](https://github.com/big91987/model-relay/actions/runs/40284625015/job/121446116876)；附件`code-review-delegate-40284625015` | [失败作业](https://github.com/big91987/model-relay/actions/runs/40288396722/job/121457440221)；附件`code-review-cli-40288396722` |
| 沙箱与原件 | released，8件1104789bytes | released，7件1428783bytes |

GitHub附件均为可下载文件，不是已部署在线页面。两侧释放后均在新目录完整下载原件、逐bytes/SHA核对；GitHub每侧result.json/report.md/preview.json亦实际下载，与平台原件逐字节相同。CLI report.md为3192bytes/SHA `f842098c02e2a3732d3f72ebfdcdd50408529192bdb53f3eb8dc2b4d0ab6055b`；Delegate report.md为386bytes/SHA `73120277b1df047c4a8c491aafe82b62790c6575d22f51634b59040c401b50be`。结果可用不依赖继续保留沙箱。

CLI在时间放宽后确实得到更多输出，但仍未完成。500000只是上游软阈值，实际1678348（约3.36倍），不能描述成500000硬限额。日志首个预算停止已1187807，其后仍有其他分组和grace回合，最后一个grace请求还发生300秒超时；精确的每请求/压缩/分组记账顺序仍待上游进一步诊断，不能把全部超额归为并发或单一模型原因。summary.files_reviewed=13表示尝试数，完成数必须读取manifest（5）。费用不可得；token口径、缓存与预算不同，不能直接推算费用或宣布模型胜负。

### 四项候选的独立静态复核

下表依据实际GitHub文件清单、基线与当前源码、Go类型契约及隔离探针核验，未运行产品测试、未让协调者代产品QA。只读取候选content，不以模型思考文本作为证据。

| 候选 | 结论 | 可复核依据 |
|---|---|---|
| README清单摘要未随文件更新（low） | 误报 | GitHub files明确README modified/+2/-2，blob `d219bd51f134df55ba7bae698ce30376e7c2f666` 与导出一致；5605bytes/SHA `3da66c244ab2c9d57509560716991abd61163fd0d470b8cc047880de537eb914` 匹配SOURCE。被OCR排除不等于Git没有修改。 |
| Delegate的畸形comments/files导致异常（medium） | 基线已有缺陷/加固项 | base与639bbe8的assess_delegate逐字相同；两版对comments:[null]或files:[null]均可复现AttributeError。不能算当前PR新引入。 |
| 私有配置null/[]漏捕TypeError（low） | 诊断分类加固项 | 两版同样TypeError，当前新增run包装未覆盖此类型，但未新增失败行为，不会提交审查或误判通过。 |
| 平台Receipt output/log显式null（low） | 受支持接口契约下误报 | Go的output是string,omitempty，log为*ConnectorLogInfo,omitempty；previous_results直接marshal类型化回执，空值缺省，不产生该候选的显式null。仅人工构造不符合契约输入可触发。 |

因此是 **4候选、0确认当前PR新增缺陷**，不是发现4个新bug。Codex零发现与CLI候选未证实，均不能证明代码无缺陷或计算漏报率；本次缺少含已知新增缺陷的受控评测集，且CLI只完成5/13。此前将旧空comments报告器崩溃称为Codex漏检不准确：同一缺陷在base已存在，原GitHub评论已更正。

### 观察到的瓶颈与后续改进

1. **先控制审查输入。** Codex首条命令读入约1MB source库存，5667项中5216为历史Run材料；还两次因jq/python不存在退出127。完整库存继续留给固定校验，给Agent的读取说明应只取base/head/expected，并明确使用已有python3；不为模型随意尝试就安装额外工具。需要下一轮实测后才能量化节省。
2. **CLI先解决完成性与预算语义。** 单纯延时没有获得完整审查。应独立排查分组/上下文压缩与聚合预算记账，显式区分Git实际变更和筛选清单，避免README类误报；本轮不再加时间、加token或弱化覆盖求绿。实际费用保护不能只依赖该软阈值。
3. **减少CI等待占Runner。** 一台Runner让matrix串行，并在远程审查期间持续占用。首次CLI checkout三次直连超时，没有平台Run；应用项目既有git_proxy字段后，只补跑CLI，checkout2秒成功。公开小探针曾直连/代理均通过，不能反证正式job失败，也不能把总耗时全部归仓库大小。后续模板可把提交与异步观察拆开，保留版本绑定和去重。
4. **失败交付仍有缺口。** 旧23f8因报告器空comments异常缺report，原回执failed保持，六件只读诊断副本不能冒充正式归档，也未强删原环境绕过门禁。此次源修复后，新CLI partial能够正式产出七件并释放，这是新路径实证，不代表旧失败回执被修复。部分失败产物保留与正式释放机制仍需梳理。
5. **边界仍保留。** Agent原生回合硬截止缺口未修；本轮无待审批阻塞。研发主图revision14已升级，但tests→code_review→QA新增阶段没有新整链Run，本对照不能代替。跨仓fresh install、长期并发可靠性、误报/漏报评测未做。

当前建议优先用已完成本样本的Codex＋Skill作审查参考，CLI＋GLM继续作为对照与调优路径；不据本轮静态结果替代固定测试、独立产品QA或人工合并，也不自动切换现有研发图策略。PR71仍draft，Issue69保持Open以追踪CLI完成性和研发整链未验部分；平台仅开发分支提交推送，没有合平台main或产品部署。


### 用户授权的延长窗口补验（2026-10-26，进行中）

用户要求 CLI 至少获得30分钟或更长时间。实时核对PR71仍为open/draft、base a1cbbb862714346a77a24c96b2bf5d5672c6be1a / head a6e77d582039b4c91e979a528e0b78c88c6dcb80；旧CLI89dc1f4e及Delegate275829a3均completed，最近Actions均已结束。上一轮整命令30分钟未耗尽，约18分29秒因预算和请求限制提前结束，因此只改外层超时不足。

唯一模板源码将单请求放宽600秒、分组30分钟、OCR进程45分钟、执行节点50分钟；软token阈值500000→3000000（不是硬费用上限，可能超出），CI观察60分钟/job75分钟。模型、low effort、并发2、规则及被审PR版本保持。本轮仅重跑CLI，以新用户授权事件的稳定request_id通过正式API新建隔离Run；不恢复完成Run、不重跑Codex、不改变PR71 head或既有结论。不把这轮Harness代码审查描述成产品功能质量评测。先标准导出/安装、冻结旧Run核对，再单次提交和完整留存；尚未获得新审查结果。


### 长窗口 CLI 补验结果（2026-10-26 10:10UTC完成）

用户授权的新Run [cb5547cf](http://10.1.20.112:8795/workflow-runs/cb5547cfa116d5e163eb994faea92fcc)已正式completed，CLI图revision5。标准导出源35a4cbda2a8241a451a8752c50da82aa25c21763/166件，安装同时把独立Delegate图更新到7；只新跑CLI，研发主图没有由本次安装修改。PR71 base a1cbbb862714346a77a24c96b2bf5d5672c6be1a / head a6e77d582039b4c91e979a528e0b78c88c6dcb80保持draft/open，未改候选代码、未合并或部署。该Run由本轮用户授权事件通过正式API启动，不是新增GitHub Actions执行；原Actions失败及旧Run历史保留。

实际结果：原生complete、manifest selected13/completed13/reused0/failed0/waived0，raw.comments及result.comments都为空，固定exit0/clean。准备113.386秒，审查阶段1021.003秒，OCR命令1010.783秒（16分51秒），全Run1134.390秒（18分54秒）。无需为凑30分钟强行保持运行；配置是单请求600秒/分组30分钟/OCR45分钟/执行50分钟。原生63次工具调用（34search、23read、5diff、1find）零工具失败；本轮无预算耗尽或请求超时记录。stderr仍有Git2.39.5低于OCR建议最低2.41.0的环境告警，不能称环境版本已完全符合其建议。

实际input2572183/output69658/total2641841，cache_read500544为输入子集；软阈值3000000，实际费用不可得。上一CLI为5/13、4候选、18分29秒、1678348 token；Codex为13/13、0发现、审查阶段419.637秒、2081249 token。输入源码清单SHA98fb4021169fe6268f840026b11e80897785de1e6fcfc39302a85ecc70360d33与rules SHA329b379966e1b1397dc167251bf9c63e5742a73504821fa4011ca9664d17c1ad均逐字节同前两路，候选源码不变。这次同时改时间和预算，单样本不能把完成率变化单独归因于时间，不能据不同缓存/模型token宣布费用优劣。

本轮四条旧候选均未再报告，OCR原始comments就是0，并非平台删除。三组日志均Round1/1 added no new findings，仍low effort；没有逐条解释为什么排除旧候选。之前复核的两处基线健壮性/诊断缺口仍未修，不因新clean关闭。结论是完整执行已获实证，问题发现质量未获证明；结果随运行变化，不能从0发现推导无缺陷或模型优劣。后续应以真实产品功能PR、已知缺陷样本及明确新增/历史问题范围评价，不能继续仅加时间求发现数。

正式留存7件1120242bytes，释放后从新目录重新下载全部bytes/SHA一致，environment=released。report.md379bytes/SHAfa20e637da701050ecec6437285f779e3f476ad649ee8f974af03a49493ac85e；result SHA7c560823d282ea9ea2bf59ff20e700178b21b741018a7c062feb2c7d5522c9d7。完整正文与版本信息已发布到[既有Issue69](https://github.com/big91987/model-relay/issues/69#issuecomment-6413542887)，平台原件需登录，不冒称GitHub Actions附件或部署看板。

工程记录：22审查相关回归、141模板回归20.374s与ruff通过；首次额外模板调用缺sdk/python导致导入错误，正确PYTHONPATH后完整通过，属于测试调用问题。直连push超时先核远端未更新，再用既有代理推送35a4cbd成功。CLI适配器subprocess.run使用PIPE，结束后才写原始报告/日志，长任务在途缺少文件级进度，是后续可观测性改进点。本轮没有更改在途配置、旧冻结回执或定时任务。
