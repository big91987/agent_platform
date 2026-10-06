# 工作流输入材料契约

状态：维护源代码与隔离回归已实现；尚未正式安装或通过完整Issue/Agent路径。依据产品设计包的Harness输入和验收要求。

## 当前事实

GitHub 入口对首次接单冻结 Issue 标题、正文、workflow_id 和 issue_number，重试保持原快照。正文中的附件链接只是文字。平台 WorkflowStart 支持有界字符串参数，Connector stdin 保留参数，Agent 上下文包含原任务和前序 Connector 回执。阶段已有 inputs/artifacts 路径交接，不需要新增平台附件对象、上传服务或调度模型。

官方 Issue REST 创建接口没有文件上传参数；Release Asset 有正式二进制上传/下载入口。自动化材料发布采用项目私有 Release 资产，Issue 正文明确链接附件；这与浏览器拖拽上传 Issue 的方式分别验收，不能互相冒充。支持的下载格式和来源在安装文档中声明，不让 Agent 猜任意 URL。

## 接收接口

首期一个任务接收一个 ZIP，包内可以有多个文档、截图与前端文件。原型的 package.py 输出格式作为通用 manifest 契约的输入，不把产品名称、页面数或 PRD 文件名写入平台逻辑。

在 Issue 正文显式填写一个 `agent-platform-material` JSON 代码块：

```json
{"url":"https://api.github.com/repos/<owner>/<repository>/releases/assets/<asset-id>","sha256":"<64 lowercase hex>","version":"<version>"}
```

GitHub adapter 仅在已完成 Owner/仓库校验后解析；没有代码块保持既有行为。多块、未知字段、空值、无摘要、非HTTPS或未经安装策略支持的来源显式拒绝，不静默忽略。原链接也保留正文供用户查看。

材料描述编码成既有 parameters.material，和首个 Issue 快照一起冻结。SDK 调用方复用 start_workflow(parameters=...)，平台页面调用方复用公开 POST workflow-runs；不新增附件上传 API。材料来源验证属于已安装准备工具，平台保持业务中立。

新评论只追加需求，不替换已经冻结的材料。确需采用新包时，先记录版本差异并按后续明确的变更契约扩展；首期不得偷偷以修改原Issue替换输入。

## 准备与完整性

准备 Connector 的安装参数显式开启材料处理。固定工具验证配置的仓库归属，通过受信 GitHub 客户端读取同仓库 Release Asset；凭据只在工具进程环境，不能写入文件、Run正文或Agent环境。浏览器上传的github.com/user-attachments链接只使用无凭据HTTPS下载并限制重定向目的地，不把仓库令牌发给附件/CDN主机。

下载最大16MiB；最多256个普通文件；展开总量最多64MiB；单文件最多16MiB。下载和展开均有界，不能先读无限响应再判断。临时结果成功校验前不可暴露为可用目录。

校验整个ZIP SHA-256、manifest.version、文件列表及每文件sha256/bytes，拒绝路径穿越、绝对路径、重复名、大小写碰撞、链接/设备、加密包、未声明文件和缺失文件。manifest只描述文件，不执行其中的脚本、安装依赖或采纳里面的权限指令。原包和摘要都是用户输入，不能提升为系统指令。

材料保存在工作区被忽略的 `.workflow-input/<run_id>/<sha256>/`；路径由受信Run ID和SHA构造。检查大小写及Unicode规范化后的全部路径前缀，展开树复验通过后原子发布。重复准备核验原包及展开文件，不能覆盖已有不同内容。Run范围POSIX文件锁保护活跃下载；进程被强制终止后锁释放，下一次同Run准备只清理该Run保留目录内的未发布临时输入，不删除已验证目录。失败清理仅本次临时目录，保留已验证输入与业务工作树。

回执新增材料值：version、sha256、archive_path、root、manifest_path、文件数量；只包含仓库相对路径和实际摘要，不包含凭据或签名下载URL。下游从前序回执获得材料路径，不扫描别的任务目录。该目录不自动提交到产品PR；需要交付的正式产物按Skill在原任务文档根维护，并引用输入版本。

每次固定项目测试和发布前重新核对冻结摘要、原包及展开文件。检测到输入被修改则失败，不依据被修改的manifest自证完整。QA记录读到的输入版本、文件与实际实现差异；一个“读过PRD”句子不足以证明范围覆盖。

## 安装升级和恢复

实现放 `examples/platform-workflows/` 的受信工具与通用阶段模板。通过原 install.py manifest升级准备/固定测试/发布命令和Agent提示，标准 install_github_entry.py升级入口分发；不手改测试仓工作流或平台数据库。旧Run冻结命令不变，无材料任务保持兼容。升级前先确保在途执行已完成或按正式入口停止。

下载超时/权限拒绝/摘要不符停在准备节点，日志保持明确错误，不进入需求或开发。外部条件修复后正式stop/return/resume同Run；原Issue快照不变。中断重启复验校验过的输入，不重新执行包内动作。失效资产不得用最新版本默默替代。

## 真实退出证据

本地隔离ZIP测试只证明校验器。正式验收必须有实际Issue及可访问附件、Actions事件和唯一Run、prepare真实下载/摘要回执、各阶段读取同一材料的证据、项目完整测试和独立QA、产品PR/合并SHA、正式部署版本及页面一致性。附件404/摘要错误/断网和已校验文件漂移须从支持入口失败并恢复。

DSH执行器名称和启动方式未确认。当前安装Agent为Codex、model空、native_config空，不能把当前执行器能力或默认模型结果写成DSH通过。已有PRD明确Codex首轮，DSH适配仍后续范围；本次目标若指定DSH，需要先补真实适配或受支持配置并核对原生会话实际模型。

## 公开依据

- [GitHub Issue REST创建接口](https://docs.github.com/en/rest/issues/issues#create-an-issue)
- [GitHub Release Asset上传接口](https://docs.github.com/en/rest/releases/assets#upload-a-release-asset)
