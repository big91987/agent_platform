# 补充浏览器验证

`check(root, plan)` 仍是同一个 MCP 工具。优先使用已有动作；固定动作无法表达的页面检查，由 Agent 在任务目录补充断言，不必修改公共 Harness。已有 `verify` 质量门禁继续执行。

## 原生缩放与可访问性证据

```json
[
  {"action": "viewport", "width": 1440},
  {"action": "zoom", "factor": 2},
  {"action": "accessibility", "contains": "版本"},
  {"action": "page_script", "script": "docs/05-validation/tasks/123/browser-scripts/layout.js"}
]
```

`zoom` 使用隔离 Chromium 的扩展 API `chrome.tabs.setZoom/getZoom`，保存实际倍率、布局视口、DPR 和当前截图。不是 CSS zoom、deviceScaleFactor 或触摸捏合模拟。320px 浏览器视口放大 200% 后，布局宽度通常是 160 CSS px；应按验收规定的条件检查，不能偷偷换成 320 CSS px。仅含 zoom 的检查使用支持扩展的 Chromium，其余检查沿用原有浏览器启动方式。临时配置不复用用户浏览器。

`accessibility` 通过 Chromium Accessibility.getFullAXTree 保存真实 AX 树，`contains` 检查未被忽略节点的可访问名称。跨多个文本节点的内容分别断言；它不证明读屏器语音或特定读屏软件兼容性。

## Agent 自己写断言

文件须位于当前任务 `docs/05-validation/tasks/<issue>/browser-scripts/*.js`，内容是无参数函数表达式，例如：

```javascript
() => {
  const title = document.querySelector("h1");
  if (!title) throw new Error("标题不存在");
  const rect = title.getBoundingClientRect();
  if (rect.left < 0 || rect.right > innerWidth)
    throw new Error("标题超出视口");
  return { width: innerWidth, title: title.textContent };
}
```

脚本在隔离页面里执行，可使用 DOM、浏览器 API、异步函数；抛异常或返回 false 表示失败，返回的数据记入 observations。5 秒异步超时，整次检查还受原有进程超时控制。工具保存 executed-plan.json（含本次实际脚本内容）、browser.json、截图及 AX 文件，供独立 QA 检查和复现。

这是补充页面断言的入口，不是宿主任意命令执行接口。没有 Node、宿主文件系统或凭据；网络仍限制在当前临时本地应用，WebSocket 禁用。禁止越出当前任务脚本目录、符号链接及直接提交内联 source。Agent 可在其既有执行权限内编写其他测试，但不能改公共门禁或扩大宿主权限来跳过限制。

## 阶段责任

新增验收标明来源（用户要求、项目约束、推荐默认）、必要性、验证办法及负责阶段。需求和设计阶段先判断方法是否可行；研发就绪与 QA 最终放行分别判断。已属于 QA 的检查可以在 QA 执行，不能把研发必需检查直接略过。

遇到缺口先使用现有动作或补充任务断言，真实失败就修复。仍需额外设备或权限时记录尝试、证据和最小补齐方案，按既有交接策略处理负责该决定的阶段。不得删掉已接受条款、伪造通过，或把同样的能力缺口原封不动交给下游。

## 安装与验证

新仓库默认使用 examples 内工具。已有 `browser_source` 覆盖不会被静默替换；选择已验证的 bundled runtime 后刷新工具注册：

```sh
bash examples/github/install-tooling.sh
python3 examples/github/setup.py --config <private-runner-config> --tools-only \
  --browser-source examples/github/tooling
PYTHONPATH=sdk/python:examples/github python3 -m unittest browser_capabilities_test -v
```

旧会话保持原有 Agent 配置快照；相同 MCP 连接的新进程会读取更新工具代码及配置。已经运行的旧进程需本轮结束后再使用新能力。仅更新 Agent 模板不会改变旧会话的历史指令，应向旧会话提供本次工具升级说明。

实现依据：[Playwright 扩展支持](https://playwright.dev/docs/chrome-extensions)、[Chromium tabs zoom API](https://developer.chrome.com/docs/extensions/reference/api/tabs)。
