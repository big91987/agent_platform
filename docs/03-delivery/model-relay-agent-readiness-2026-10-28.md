# Model Relay 工程初始化体检

日期：2026-10-28（北京时间）。工具用于工程初始化，不加入正在进行的研发流程。

固定工具：`@kodus/agent-readiness@0.1.3`；固定仓库：`big91987/model-relay` main `a1cbbb862714346a77a24c96b2bf5d5672c6be1a`。

**工具原始评级为Level 1，35项基础检查中12项通过、23项未通过，4项AI检查跳过。评级是其规则结果，不是产品成熟度或工程质量验收结论。**

## 执行范围与回执

- 基础检查实际耗时0.238秒，exit 0；未设最低等级门禁，exit0仅说明扫描命令正常结束。
- 不调用模型、不启动Web服务、不执行仓库测试或安装脚本。安装在忽略目录并关闭npm生命周期脚本；扫描进程未继承Git或模型凭据。
- 独立快照5667文件，扫描前后逐文件SHA256一致。远端main的Git树与现有Runner缓存对象相同，从固定Git对象只读导出；未修改缓存工作区。
- 两次网络压缩包获取分别180秒/240秒超时，均未用于扫描；另起Git clone未完成且在找到缓存后仅停止该下载及子进程。失败记录与部分下载私有保留，未停止任何Runner或Pipeline。
- 实际扫描于北京时间2026-10-28 14:27:57开始。源码获取和准备时间与0.24秒扫描执行时间分开。
- 原始JSON：`27143` bytes，SHA256 `6f4e3efa5b058535de71a7c2d536ea7ea34bf3304b695b5e70423c05471c48ef`；stderr为空。原JSON、安装锁和完整性、快照清单及执行回执保存在忽略目录`.data/agent-readiness-scan/`。

## 七个维度的原始分数

| 维度 | 通过/计分项 | 工具百分比 |
|---|---:|---:|
| 代码规范 | 2/5 | 40% |
| 测试 | 3/5 | 60% |
| 文档 | 1/6 | 17% |
| 开发环境 | 2/5 | 40% |
| CI/CD | 3/6 | 50% |
| 代码健康 | 1/3 | 33% |
| 安全 | 0/5 | 0% |

检测到Node和Go项目；不是monorepo。未开启AI的四项为命名、测试质量、README质量、文档对Agent的友好性，均为Skipped，不能写成质量不通过。该版本分级从Level 1起步，即使一级条件未达到80%也显示1；不可把这个标签解释为一级要求全部满足。

## 工具提示与逐项核对

| 检查项 | 工具状态与原始提示 | 本次核对 |
|---|---|---|
| 代码规范 · `linter` | Fail：No linter configuration found. | 没有检测到它列举的专用Linter配置；项目Makefile已有go vet及format-check。可评估是否补前端静态检查，不等于完全没有静态检查。 |
| 代码规范 · `formatter` | Pass：Go has built-in formatting via gofmt/goimports | 本次仅保留工具结果，未据此验证业务正确性。 |
| 代码规范 · `type-checker` | Pass：Go has a built-in static type system | 本次仅保留工具结果，未据此验证业务正确性。 |
| 代码规范 · `pre-commit-hooks` | Fail：No pre-commit hooks found. | 未发现约定的Hook配置。是否启用按团队提交方式选择，不是所有工程都必须本机Hook。 |
| 代码规范 · `editorconfig` | Fail：No .editorconfig found. | 根目录确实缺少.editorconfig，可作为低成本的编辑约定补齐。 |
| 代码规范 · `naming-conventions` | Skipped：Requires --ai flag | 未开启AI，本次未评价。 |
| 测试 · `test-framework` | Pass：Go test files found (44 files) | 本次仅保留工具结果，未据此验证业务正确性。 |
| 测试 · `test-files-exist` | Pass：Found 60 test file(s) | 本次仅保留工具结果，未据此验证业务正确性。 |
| 测试 · `test-script` | Pass："test" target found in Makefile | 本次仅保留工具结果，未据此验证业务正确性。 |
| 测试 · `coverage-config` | Fail：No coverage configuration found. | 未发现覆盖率报告配置；Makefile测试有-race等验证但没有覆盖率报告入口，属于可补的工程信息。覆盖率本身也不能代表断言有效。 |
| 测试 · `e2e-tests` | Fail：No E2E or integration test setup found. | 识别遗漏。tests/browser/下有实际浏览器旅程及Go驱动，Makefile browser执行go run ./tests/browser；不是没有E2E。 |
| 测试 · `test-quality` | Skipped：Requires --ai flag | 未开启AI，本次未评价。 |
| 文档 · `readme` | Pass：README.md found with 11834 characters | 本次仅保留工具结果，未据此验证业务正确性。 |
| 文档 · `contributing` | Fail：No contributing guide found. | 根目录没有CONTRIBUTING.md；已有README/AGENTS，建议整理统一贡献与验证入口，避免重复规则。 |
| 文档 · `api-docs` | Fail：No API documentation found. | 未找到它约定位置的OpenAPI/docs/api；README有调用示例，但尚不能代替完整接口契约，适合补稳定API索引/契约。 |
| 文档 · `codeowners` | Fail：No CODEOWNERS file found. | 未发现CODEOWNERS，可按真实维护责任决定是否补齐。 |
| 文档 · `ai-context` | Fail：No AI context files found. | 误报。根目录AGENTS.md存在，但0.1.3的ai-context规则不识别AGENTS.md；不应为了分数重复造CLAUDE.md。 |
| 文档 · `architecture-docs` | Fail：No architecture documentation found. | 不是没有设计材料：docs/workflow/design.md及历史Run中有architecture/ADR等；缺少稳定、可发现的现行架构入口。建议整理索引和现行基线，不重写历史。 |
| 文档 · `readme-quality` | Skipped：Requires --ai flag | 未开启AI，本次未评价。 |
| 文档 · `docs-agent-friendliness` | Skipped：Requires --ai flag | 未开启AI，本次未评价。 |
| 开发环境 · `lock-file` | Pass：Lock file found: package-lock.json | 本次仅保留工具结果，未据此验证业务正确性。 |
| 开发环境 · `containerization` | Fail：No container configuration found. | 未发现根目录Dockerfile/devcontainer。项目由平台沙箱提供环境；不应仅为分数要求每仓另造容器。需核对项目依赖声明与实际初始化路径。 |
| 开发环境 · `env-documentation` | Fail：No environment variable documentation found. | 它主要找.env.example等固定文件。当前产品通过README所述CLI参数、数据目录和key-file配置；不能据此认定毫无配置说明，不应为了分数引入额外环境变量。 |
| 开发环境 · `setup-script` | Fail：No setup script or dev command found. | README已有构建/初始化/浏览器安装说明，Makefile有verification-deps锁定npm准备；不等于无准备能力。可考虑归并成清晰的一键工程初始化入口。 |
| 开发环境 · `version-pinned` | Pass：Version pinning found: go.mod | 本次仅保留工具结果，未据此验证业务正确性。 |
| CI/CD · `ci-config` | Pass：CI configuration found: .github/workflows/agent-platform-code-review.yml | 本次仅保留工具结果，未据此验证业务正确性。 |
| CI/CD · `ci-runs-tests` | Fail：CI configuration found but no test execution detected. | 识别遗漏。deploy-local.yml调用controller.py prepare，模板controller中实际调用make verify；Makefile verify-gates包含race/HTTP/browser等。这里只证明接线存在，本次没有执行产品测试或判断其通过。 |
| CI/CD · `ci-runs-linters` | Fail：CI configuration found but no linter execution detected. | 它没有追踪上述脚本调用；Makefile verify-gates包含go vet和format-check。专用前端Lint是否需要是另一个工程选择。 |
| CI/CD · `build-automated` | Pass："build" target found in Makefile | 本次仅保留工具结果，未据此验证业务正确性。 |
| CI/CD · `deploy-pipeline` | Pass：Deploy stage found in .github/workflows/deploy-local.yml | 本次仅保留工具结果，未据此验证业务正确性。 |
| CI/CD · `branch-protection` | Fail：No branch protection rules documented or configured. | 工具仅查文件/文档。额外GitHub API真实确认main protected=false；若作为多人交付仓，应评估分支保护和合并检查，当前未改设置。 |
| 代码健康 · `no-outdated-deps` | Pass：package-lock.json was modified 2 day(s) ago | 仅以锁文件本地mtime判断；本次是Git归档文件时间，不能证明依赖版本最新或安全。 |
| 代码健康 · `dead-code-detection` | Fail：No dead code detection tool found. | 未识别专用死代码检测工具；它检测的是检测工具配置，不是已经查到死代码。 |
| 代码健康 · `bundle-analysis` | Fail：No bundle analysis configuration found. | 该项目为Go内嵌静态页面及浏览器测试依赖，是否需要打包体积分析应按实际构建方式评估，不建议机械补Webpack类工具。 |
| 安全 · `license` | Fail：No license file found. | 确实未检测到LICENSE；这是私有测试仓，是否需要开源许可证取决于分发计划，不应为等级随意选择许可。 |
| 安全 · `security-scanning` | Fail：No security scanning configured in CI. | 仓库本地配置未被检测到；它未核GitHub App/组织策略，不等于已完成安全审计或证明没有外部扫描。 |
| 安全 · `secrets-detection` | Fail：No secrets detection tool configured. | 仓库本地配置未被检测到；此前PR71有GitGuardian成功实证，但本次未重审组织/PR级扫描策略，不能据0分断言完全无秘密检测。 |
| 安全 · `security-policy` | Fail：No security policy found. | 未检测到SECURITY.md，是否补漏洞报告与处理入口按项目对外服务范围决定。 |
| 安全 · `dep-update-automation` | Fail：No dependency update automation found. | 未发现Dependabot/Renovate配置，可作为持续依赖维护的改进项。 |

## 建议如何使用这些信息

1. 优先整理稳定的现行项目入口：贡献/验证说明、架构及API文档索引。已有内容复用，减少Agent到历史Run中寻找当前依据。
2. 对确认缺少的覆盖率报告、依赖维护自动化和必要静态/安全检查，按项目定位选择补齐；分支保护属于仓库设置选择，不在本次扫描中更改。
3. 初始化工具适配应先解决AGENTS.md、tests/browser、make verify和脚本委托的识别，不把漏检转换成新增重复文件/工具的要求。
4. 不以总分为目标补不适用的打包分析、额外环境变量或开源许可证；保留原始报告，再记录适用性判断。

## 补充观察：仓库准备成本

完整快照501529318 bytes，其中docs/workflow/runs有5216件、497551307 bytes，占约99.2%；当前代码与其他材料约4MB。只读Git归档仍为303347558 bytes，说明历史产物使初始化和复制成本显著增加。这是本轮准备过程的额外观察，不是Agent Readiness的原生发现。后续可单独规划历史原件与工作代码的保存方式，保留可追溯索引及正式产物；本次没有删除或迁移任何历史证据。

## 执行证据边界

本次没有运行make verify、浏览器旅程、Code Review或独立产品QA，也没有改变Workflow、在途Run、仓库配置或模型连接。静态接线存在不表示测试已执行通过。main的同版GitHub检查中另有prepare=failure；本次只读记录状态，未调查首错或重跑，不能由此次体检覆盖其失败。

## 工具原始改进建议（按其输出顺序）

1. `contributing`：Contributing guide（impact=high，effort=low）。Add a CONTRIBUTING.md with development workflow, coding standards, and PR guidelines
2. `linter`：Linter configured（impact=high，effort=medium）。Add a linter like ESLint or Biome to enforce code style and catch common mistakes
3. `env-documentation`：Environment variables documented（impact=high，effort=medium）。Address the failing "env-documentation" criterion to improve agent readiness
4. `setup-script`：Setup script or dev command（impact=high，effort=medium）。Add a setup script (Makefile, scripts/setup.sh, or npm run setup) to automate development environment setup
5. `codeowners`：CODEOWNERS defined（impact=medium，effort=low）。Create a CODEOWNERS file to define ownership for different parts of the codebase
6. `ai-context`：AI context files（impact=medium，effort=low）。Add AI context files such as CLAUDE.md, .cursorrules, or .github/copilot-instructions.md with project-specific guidance
7. `security-policy`：Security policy（impact=medium，effort=low）。Create a SECURITY.md file describing how to report vulnerabilities
8. `pre-commit-hooks`：Pre-commit hooks configured（impact=medium，effort=medium）。Set up pre-commit hooks with Husky, Lefthook, or pre-commit to run checks before every commit
9. `api-docs`：API documentation（impact=medium，effort=medium）。Add API documentation using OpenAPI/Swagger specs, JSDoc, TypeDoc, or a dedicated docs folder
10. `ci-runs-tests`：CI runs tests（impact=medium，effort=medium）。Address the failing "ci-runs-tests" criterion to improve agent readiness
