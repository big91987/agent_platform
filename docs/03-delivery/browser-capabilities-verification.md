# 浏览器验证能力补齐

## 问题与修复

固定动作不支持原生缩放和可访问性树，Agent 又没有补充页面断言的入口，使必需验收停在“工具不支持”。本次在 GitHub example 的浏览器 MCP/runtime 增加 `zoom`、`accessibility`、`page_script`，补充阶段验收来源、责任及工具缺口处理规则。平台核心没有新增研发业务逻辑。

任务脚本只在当前临时页面执行，保持本地来源网络限制；路径限定任务目录，原质量门禁和受保护文件检查继续生效。原始脚本随执行证据保存。上游快照来源保留，局部适配的哈希记录在 tooling/SOURCE.json。

## 验证

- 修改前，真实 stdio MCP 回归在 zoom、accessibility、page_script 路径报 Unsupported action。
- 修改后，5 项真实浏览器测试通过：原生缩放重排及 AX 证据、失败断言不放行、隐藏文字不算可访问文本、路径/符号链接限制及错误输入恢复、页面脚本的外网请求被拦截。
- 原生缩放返回 2；1440px 窗口布局宽度变为 720 CSS px，DPR 为 2，visualViewport.scale 为 1，未使用 CSS zoom 或 pinch 替代。
- 完整 scripts/verify.sh 通过：前端测试、2 项导航浏览器回归、6 项 SDK 测试、66 项 GitHub 适配测试、8 项部署测试、Go vet/race/build。Go 链接器仍有已有的 macOS LC_DYSYMTAB 警告，退出码为 0。
- smoke-portability.py 通过：独立临时产品，经 stdio MCP check/verify 执行 3 次真实浏览器检查，产生 6 张截图，无外部 Harness checkout 依赖。
- 发布前敏感信息扫描通过。

## 真实任务边界

在版本页脚任务上实测：桌面 200% 和 AX 读取通过；320px 窗口原生 200% 时，任务布局断言报告版本文字横向溢出。该结果没有被改为通过。已更新本机 MCP 注册、切换到验证后的 bundled runtime，并将证据交给原研发会话继续修复；这不等于产品 QA 已通过。

AX 树验证不代表实际读屏器语音、全部浏览器或所有辅助技术兼容性。阶段规则属于 Agent 指令，不能据此保证所有未来任务都能自主消除环境限制。
