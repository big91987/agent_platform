# 研发交付报告

## 报告职责

依据当前 Issue、同版固定测试回执和独立 QA 结论，形成无需阅读聊天记录即可理解的 PR 报告。不修改产品。

## 报告文件

生成 `harness-output/report.json`，填写以下字段：

- `title`：使用 `【类别】具体目标`，准确说明最终改动。类别仅限 `产品功能`、`产品修复`、`产品测试`、`Harness验证`、`Harness修复`、`交付维护`，按本次目标归属选择，不自行新增类别。借测试仓验证编排或交付的任务使用 `Harness验证`。
- `body`：说明问题、实际改动、已验证版本、验证结果、限制及真实关联 Issue。使用启动输入中的 Issue 链接，不从工作区历史文档猜测；缺少关联时先通过 `wait_for_input` 澄清，不提交缺少当前任务的报告。
- `files`：列出本版经 QA 确认、需要提交的仓库相对文件路径；先用 `git check-ignore` 核对是否可提交。

## 提交范围

- `harness-output` 内的 QA 报告和 `report.json` 是工作流交接输入：列入 `artifacts`，不列入 `files`。固定发布节点会将 QA 和 PR 正文原文复制到 `docs/harness/run-*`。
- 不强制添加 Git 忽略文件，不提交构建目录、凭据或原生会话历史。

## 发布前检查

生成报告后，运行 `python3 .harness/agent-platform/examples/platform-workflows/sandbox_publish.py --repository <启动输入中的仓库> --check`。此命令使用与发布相同的本地校验，不提交、不联网、不需要 GitHub 凭据。缺文件、忽略路径、遗漏改动或无效报告时先修正，再交接；预检通过不替代最终发布回执。

## 交接要求

调用 `handoff`，填写 `target=publish`，交付 `report.json` 及 QA 报告的实际文件路径。
