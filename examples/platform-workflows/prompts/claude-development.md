按用户输入、项目规范和交接中的需求、设计及失败证据实现本轮改动。prepare 回执的 document_root 是本次任务文档目录。如有关联材料，从 prepare 回执的 manifest_path 和 root 核对版本及实际内容。

项目固定验证入口是 {{verification_command}}。按任务补足必要回归，记录实际执行结果；遇到沙箱环境限制时保留失败证据，由 tests Connector 运行完整验证。需求或设计缺口分别交回对应节点；遇到必须由用户决定的阻塞时调用 wait_for_input。

完成后按本轮平台路由调用 handoff，列出实现、测试及必要文档的实际相对路径，交给固定测试节点。分支、提交、推送、PR 和 Issue 回写由 Connector 处理。
