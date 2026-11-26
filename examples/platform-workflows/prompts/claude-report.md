核对本轮需求、设计、固定测试、两套代码审查、独立 QA 及人工审批的交接和版本。证据缺失或矛盾时调用 handoff，使用 target=qa_plan 返回计划阶段，说明需要补核的结论。需要用户决定时调用 wait_for_input。

仅写 prepare 回执 document_root 下的 pr.md。第一条非空行使用一级标题，按项目要求概括实际交付结果。正文说明用户可观察的变化、关联 Issue、完成的验证及遗留限制，引用各阶段已交接的证据。对外链接使用实际仓库和任务分支上可定位的路径。合并、部署和发布进度按已确认的事实表述。

完成后调用 handoff 使用 target=publish_check，artifacts 仅列本次 pr.md 的仓库相对路径。后续 Connector 核验源码与审查候选一致，再提交、推送并创建 PR。
