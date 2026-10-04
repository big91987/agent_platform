# Agent Platform 文档索引

## 产品与架构

- [产品需求](01-product/prd.md)
- [决策记录](01-product/decision-ledger.md)
- [本机平台架构](02-architecture/architecture.md)
- [账号与访问](02-architecture/access.md)
- [外部工具](02-architecture/external-tools.md)

## 工作流方案

- [方案目录与维护方式](02-architecture/workflows/README.md)
- [CI/CD Runner 直接调用 Agent](02-architecture/workflows/runner-direct.md)
- [GitHub CI + Agent Platform](02-architecture/workflows/github-agent-platform.md)
- [方案比较与选型](02-architecture/workflows/comparison.md)

## 接入与验证

- [GitHub 完整接入](../examples/github/GETTING_STARTED.md)
- [Pipeline 与交接工具](../examples/github/README.md)
- [可选本机预览部署](../examples/github/local-preview/README.md)
- [交付与独立 QA/Review 验证](03-delivery/qa-review-pipeline.md)
- [全阶段人工回退验证](03-delivery/all-stage-return-e2e.md)
- [交接管理验证](03-delivery/handoff-management-verification.md)
- [近期维护与恢复证据](validation/2026-10-04-maintenance-and-resume.md)

验证文档中的 Issue、PR 和 run 状态是记录时的快照；判断当前可否推进需查询实时状态。历史实验的未测范围不得被后续文档省略或转述为已通过。
