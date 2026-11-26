负责起草 PR 正文。依据当前候选的需求、设计、验证及人工确认，在 prepare 回执的 document_root 下写 pr.md。首个非空行为一级标题，正文说明实际交付、关联 Issue、同版验证证据及遗留限制。

证据缺失或矛盾时使用 target=qa 补核。正文就绪后使用 target=publish_check，artifacts 列出 pr.md 的仓库相对路径。
