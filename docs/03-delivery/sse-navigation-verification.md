# 会话多标签页导航与流式连接验证

日期：2026-10-04。

## 原因与修复

HTTP/1.1 下，会话页对每个标签永久保留 SSE，即使会话已结束或页面进入后台。六条长期连接占满同源连接额度后，后续页面和 API 请求排队。

修复前真实浏览器复现：前六页 69–163 ms 打开；第七页 6000 ms 超时；关闭一个旧标签后第七页 183 ms 打开。同期直接 HTTP 测量会话接口约 1–36 ms，排除本次故障由后端慢响应引起。

前端现在仅在前台且会话 running/queued/stopping 时保持 SSE。idle/failed/stopped/closed 释放连接；隐藏、pagehide 时同时停止定期刷新。重新显示时先按持久事件游标补齐历史，再按执行状态恢复订阅。可见的空闲页保留短请求检查，以发现外部发起的新一轮执行。重叠刷新合并为一个请求，避免请求堆积。暂停网页订阅不调用停止 Agent 的 API。

## 可复现回归

```sh
bash examples/github/install-tooling.sh
NODE_PATH=examples/github/tooling/full_harness/browser/node_modules node --test web/live_browser_test.cjs
bash scripts/verify.sh
```

两个浏览器测试运行真实页面、HTTP/1.1 服务和 EventSource。仅业务 API 数据为固定夹具，页面可见性测试以浏览器 visibilitychange 边界提供信号；不宣称覆盖所有宿主 WebView 的窗口管理行为。

修复前两项均失败（第七页导航超时、隐藏后连接不释放），修复后通过。覆盖：

- 连续八个空闲会话保持可导航，不保留 SSE。
- 执行中订阅；隐藏后无连接、无轮询；返回前台补齐一次工具事件并从新游标重连。
- 重叠刷新只发一次详情请求；运行结束释放订阅；外部再次排队后重新订阅；pagehide 释放。

完整检查通过：前端 10 项、浏览器 2 项、SDK 6 项、GitHub 适配器 61 项、部署 8 项，以及 Ruff、Go vet/race/build。Go 链接器出现已有的 macOS LC_DYSYMTAB 警告，命令退出成功。

## 本机部署与实际会话复测

部署前确认无 running/queued/stopping 会话；通过 stop/start 脚本重启本机服务，健康接口正常，服务提供的 app.js 与修复文件逐字一致。

用 Issue 94 的实际会话链接在独立浏览器上下文连续打开八个标签，耗时依次为 152、119、157、119、85、75、112、126 ms。历史事件和工具卡片存在，无浏览器 JS 异常，所有空闲页无 SSE。没有向业务 Agent 发送消息或重跑业务流水线。

部署前已打开的旧页面仍运行旧 JavaScript，需刷新旧会话页加载修复。前端通用的“页面暂时无法打开”提示未在此复测中出现；不能据此断言该提示所有可能原因均已消除。
