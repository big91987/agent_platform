# Architecture Structure Output Implementation Plan

**Goal:** 正式评审直接输出可核验的现状结构、推荐结构、职责对照及对象调整表。

**Architecture:** 复用事实解析器；`structure.py` 负责结构协议校验与树/表渲染，`review.py` 负责统一评审和证据核验。结构输出不改变七维评分。用户已确认方案并要求实施；本会话直接执行，无提交或部署。

**Spec:** `/private/tmp/archcheck-structure-output-handoff-2026-11-19.md` 及已确认的六步提升计划。

## Constraints and review focus

- 维护源为 examples/arch-template；不改 maas_proj 源码、不做业务 QA。
- 新协议必须包含 architecture_plan；旧回执明确 incomplete，原始历史不改写。
- 当前树只来自生产事实；规范文件的原地调整标明上下文，不伪装成生产文件。
- 提取保留原文件，整文件移动才移除；允许合理合并，拒绝覆盖冲突。
- 依赖和职责建议待复核，不冒称编译或事务行为已验证；不强制四层。
- 结构校验失败保留诊断与确定规则，总报告 incomplete，沿用既有未完成不出总分。
- 标准入口显式 max-bytes 与底层一致，不截断；MaaS 配置 4,000,000 字节。

## Tasks

- [x] 写结构校验与渲染正反例，观察旧实现失败。
- [x] 增加 architecture_plan 协议及结构模块；接入 review/validate-review/标准报告。
- [x] 更新受支持的受控测试回执和版本，历史真实模型原件保持原样；接通标准入口输入预算。
- [x] 运行工具及安装升级回归：70 项工具、12 项模板通过；评分算法不变，结构失败保留有效诊断。
- [x] 标准评测默认调用已配置模型；仅显式 --rules-only 跳过，缺配置或模型失败返回 incomplete，不静默降级。
- [x] 固定 maas_proj f1e6d65，标准安装升级后的默认入口真实评审前后端，核对原件、引用及原生结构报告。
- [x] 更新说明和验证记录，输出 MaaS 152/40 个生产文件的原生现状树、推荐结构、职责与对象调整表和七维评分。

首次外发曾被自动审批拒绝；列明 MaaS 源码范围和 DeepSeek 目的地后，用户明确重申默认用模型评测，随后正常审批通过。已完成真实 DeepSeek 评审，原始请求、回执、失败轮次和最终报告分别保留；没有改写历史回执。

真实运行暴露了跨模块提取的错误限制：源模块不应被要求重复列出所有目标模块的问题引用。增加失败回归后修复通用校验，通过标准升级应用，重新调用模型。最终后端 37.1、前端 64.3；后端 8 项发现/14 项调整，前端 6 项发现/9 项调整。源码未整改；前一轮同源码后端为 64.3，差异来自两轮模型判断及结构协议变更，不能作为整改效果或稳定评分证明。

完整原件与最终 verification.json 位于忽略的 harness-output/maas-structure-validation-2026-11-19/；可阅读报告是原生 Markdown/JSON 的逐字节复制，位于 outputs/maas-structure-validation-2026-11-19/。未执行 MaaS 产品 QA、代码迁移、提交或部署。
