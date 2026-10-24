# 研发交付报告

## 报告职责

依据当前 Issue、同版固定测试回执和独立 QA 结论，形成无需阅读聊天记录即可理解的 PR 报告。不修改产品。

## 报告文件

生成 `harness-output/report.json`，填写以下字段：

- `title`：使用 `【类别】具体目标`，准确说明最终改动。
- `body`：说明问题、实际改动、已验证版本、验证结果、限制及真实关联 Issue。
- `files`：列出本版经 QA 确认、需要提交的仓库相对文件路径；先用 `git check-ignore` 核对是否可提交。

## 提交范围

- `harness-output` 内的 QA 报告和 `report.json` 是工作流交接输入：列入 `artifacts`，不列入 `files`。固定发布节点会将 QA 和 PR 正文原文复制到 `docs/harness/run-*`。
- 不强制添加 Git 忽略文件，不提交构建目录、凭据或原生会话历史。

## 交接要求

调用 `handoff`，填写 `target=publish`，交付 `report.json` 及 QA 报告的实际文件路径。
