# 工作流输入材料实施计划

> For agentic workers: use superpowers:executing-plans to implement this plan task-by-task. Execute in the existing platform maintenance branch; product implementation remains delegated through the formal workflow.

**Goal:** GitHub Issue附件作为冻结、可校验的输入进入同一个Run，所有研发阶段使用同一份材料并保留恢复行为。

**Architecture:** 复用WorkflowStart.parameters及prepare回执。下载、安全校验、持久化属于受信准备工具；阶段模板消费回执，SDK保持通用参数接口，不新增附件服务。

**Tech Stack:** Python标准库、既有GitHub CLI与SDK、现有Go平台和安装器。

**Spec:** docs/02-architecture/workflow-materials.md

## 全局约束

- 下载16MiB，256文件，展开64MiB，单文件16MiB。
- 用户输入不成为权限指令；不执行包内代码。
- 无材料任务与旧冻结Run保持既有行为。
- 先维护源，再原manifest升级；不改运行数据库，不自行合并源main。
- 原型/产品需求草案等待评审，不在这项工程工作中擅自批准。

## 评审重点

1. 私有Release下载身份与仓库一致；附件重定向不得收到GitHub令牌。
2. 中断发生在下载、解压、原子发布任一阶段，重试不接受半包。
3. 恶意路径、大小写碰撞及压缩炸弹被实际限额阻断。
4. 修改Issue或展开manifest不能改变原材料版本或自证完整。
5. 没附件的任务、旧Run及已有安装漂移保护保持兼容。

## 任务1：材料安全校验与持久化

Files: create examples/platform-workflows/materials.py, materials_test.py.

Interface: validate_description(value, repository)->dict; install_material(workspace, run, repository)->receipt|None; verify_material(workspace, run)->None. 仅使用run.parameters.material和run.run_id；prepare无材料时返回None。

- [x] 写失败回归：成功多文件包；SHA/版本/文件摘要错；未声明/缺失/重复/穿越/绝对路径/链接；256与257文件；各尺寸边界；中断临时目录与幂等重试；已有目录漂移；同摘要原包与修改后manifest自证攻击。
- [x] 运行 `python3 -m unittest discover -s examples/platform-workflows -p materials_test.py`，确认目标功能缺失失败。
- [x] 实现标准库校验和有界下载。传输作为内部依赖隔离测试；对公开附件不带认证，对同仓私有Release用既有受信GitHubCLI。路径与已接受摘要从参数确定。
- [x] 同命令通过；只把实际支持来源列入文档，不回退到无界任意URL下载。

## 任务2：入口、准备和固定门禁

Files: modify github_entry.py, github_entry_test.py, repository.py, repository_test.py, install.py, install_test.py, prompts/common.md, README.md, sdk/python/README.md.

Consumes: 任务1三个函数。Produces: 冻结parameters.material及prepare材料回执。

- [x] 写失败回归：合法Issue代码块成为首次快照；重试或修改正文不换包；重复块/不支持来源拒绝；无附件兼容；prepare失败没有Agent输入；test/publish发现漂移失败；首次安装/重复安装/原manifest升级/旧命令不变。
- [x] 分别运行对应unittest模块，确认目标断言失败。
- [x] 入口只解析已授权正文，准备在分支准备完成后安装材料；固定测试/发布前验证材料；目录默认忽略。新增开关仅由原安装器生成，不给产品仓临时脚本。
- [x] 阶段模板要求从准备回执引用实际文件和版本；不覆盖Skill文档命名规则、不自动批准草案。SDK文档示例使用既有parameters接口。
- [x] 以上回归及ruff检查通过；检查日志和回执无凭据。

## 任务3：真实Issue材料入口和恢复

Files: update docs/03-delivery/workflows-verification.md; reuse existing deployment/install documentation.

- [ ] 核对当前无在途执行，备份正式服务及manifest；通过原manifest升级，不改变旧暂停Run输入。
- [ ] 原型评审完成后发布确定版本ZIP为私有测试仓Release Asset，建立一个真实Issue引用版本/SHA。记录实际资产URL，不能把localhost当Agent输入。
- [ ] 核对Actions交给唯一Run、准备回执的实际SHA、原生阶段工具读取材料、输出覆盖原PRD/AC及视觉交互。实际执行器须与用户指定一致。
- [ ] 用支持入口验证附件404/权限拒绝、SHA错误、下载中断与输入漂移；权限或网络失败在同Run正式恢复，错误摘要维持明确拒绝，不偷偷换包，无手改检查点、无补造回执。
- [ ] 固定完整项目测试、独立QA通过后核对原PR、精确合并、正式部署及真实UI；供应商或支付条件缺失明确Not Run。

计划自查：规格每项落任务1/2/3；无需修改Go表或新增领域对象。任务3全部未执行，隔离工具通过不能代替真实Pipeline证据。产品运行模型及需求基线未定时，不启动产品实施Run。
