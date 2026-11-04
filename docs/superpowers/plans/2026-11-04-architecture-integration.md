# Architecture tool integration

> Execute inline using executing-plans. The user explicitly requested integration, initial inspection and submission; SonarQube is excluded following the user's correction.

**Goal:** Install the existing portable archcheck in Model Relay and connect version-bound deterministic inspection to the platform workflow and Issue/PR delivery.

**Architecture:** Keep examples/arch-template as the only checker source. Existing repositories install the checker without replacing their product files. A normal execution node checks the declared project scope, archives JSON and Markdown, continues on conformance, returns proven violations to development, and stops on incomplete analysis. Existing fixed tests, independent review and QA remain.

**Spec:** The user's approved tool/template division and current integration request. Initial rules describe actual package boundaries; a passing scan must not imply the proposed domain refactor is implemented.

## Constraints and review focus

- No SonarQube installation, credentials or integration.
- Preserve unrelated dirty files and frozen/in-flight runs; upgrade only through revisioned API after checking active tasks.
- Preserve source provenance, refuse modified installed tools, no runtime-only patch.
- Bind reports to actual source hashes and Git head, retain each inspection independently; never turn scanner failure into conformance.
- No product redesign, main merge or product deployment. Publish dedicated integration commits/PRs and actual check evidence.

## Tasks

- [x] Add supported adoption installation and report entry; test independent installation, drift refusal, violations/incomplete statuses and archived evidence.
- [x] Add opt-in template composition and explicit revisioned workflow upgrade; prove routes and failure handling without replacing tests/review/QA.
- [x] Declare current platform and Model Relay scope; run real initial scans and preserve precise coverage and limitations.
- [ ] Apply source installation to an independent Model Relay branch; connect CI/Issue evidence; run supported platform entry where available.
- [ ] Independent review, affected regression, commits/PRs and final status with verified links.

## Execution notes

- Existing worktree is isolated on codex/opensandbox-workflow; unrelated assessment/workflow docs and outputs are left untouched.
- 112 API is reachable through approved execution networking. Existing tmux ds112 is idle and logged in; no new server or privilege changes were performed.

## Verified delivery and remaining gate

- Maintenance PR: https://github.com/big91987/agent_platform/pull/8 (base codex/opensandbox-workflow, no main merge).
- Model Relay PR: https://github.com/big91987/model-relay/pull/78 ; task https://github.com/big91987/model-relay/issues/77 .
- Tool 0.3.1: 33 tool tests and 11 adoption/workflow tests passed. Source go vet and existing real archive/recovery tests passed. Independent review has no remaining P1/P2 findings.
- Actual GitHub architecture CI on maintenance head0308f6e passed; downloaded report confirms version0.3.1, 74 files, snapshot7d8c67b9a3148d526f9a10810bde1ae3a145b37b141fcd09eadf9fcd9b3c7726. PR merge checkout22cf8c289e9029661a09fc24650b3ddeb39be4ed is recorded distinctly from source head.
- Model Relay standard install + supported upgrade both exercised; all24 installed maintenance-source files match SOURCE.json and source. Current make architecture-check passed29 production files with zero declared-rule violations; fixed product tests and productQA were not run.
- Found and fixed: large Node stdin EAGAIN; file-only platform artifact contract; invalid manifests/extra modules/bytecode bypass; stale-report publication failure; scoped Issue notifications; behavior drift and paginated in-flight guards; generated Python caches excluded at maintenance source.
- 112 GET confirmed existing revision15 development workflow and older waiting Runs. No original graph or Run was modified. New graph POST from Mac reset; subsequent GET confirmed no graph was created. Source transfer needed for server-local formal installation was rejected by automatic approval; explicit user permission remains pending. Do not retry transfer or claim actual112 execution/archive/notification success without that permission and real receipts.
- Source integration and GitHub delivery are complete; actual112 installation/Run/evidence remains an open gate. No SonarQube, model endpoint, merge or product deployment.
