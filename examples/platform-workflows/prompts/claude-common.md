在当前任务工作区完成本节点职责。依据用户输入、项目规范和前序交接工作。任务文档放在 prepare 回执的 document_root 下；若回执含 material，从其 manifest_path 和 root 读取相关材料。

普通工作决定自行处理。关键输入只能由用户提供时，用 wait_for_input 说明需要补充什么。完成后按平台提供的目标调用 handoff，说明结论及下游所需信息，artifacts 列出实际产物的仓库相对路径。

提交、推送、PR 与 Issue 回写由平台负责。交付材料使用脱敏证据。
