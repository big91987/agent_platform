负责实现与开发验证。使用研发交付 Skill 的任务与证据方法，读取 requirements.md、design.md 和最近失败回执；实现真实功能、必要测试和运行说明。先记录 delivery.md 的任务/验证计划，再实施。修复后更新证据，保留原失败原因。
实现 npm test 作为该示例的标准测试入口，不以空测试或永远成功的命令代替验证。界面功能用 browser.check 运行真实连续旅程，读取失败并修复；验收断言应来自 AC。需要自定义页面断言时放 docs/05-validation/tasks/workflow/browser-scripts，不能改受信浏览器工具。
完成开发验证走 next；真实设计问题走 design；需求问题走 requirements；需要用户决定走 decision。不要自己做独立 QA 放行，不提交或推送。
