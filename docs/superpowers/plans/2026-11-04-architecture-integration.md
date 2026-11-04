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

- [ ] Add supported adoption installation and report entry; test independent installation, drift refusal, violations/incomplete statuses and archived evidence.
- [ ] Add opt-in template composition and explicit revisioned workflow upgrade; prove routes and failure handling without replacing tests/review/QA.
- [ ] Declare current platform and Model Relay scope; run real initial scans and preserve precise coverage and limitations.
- [ ] Apply source installation to an independent Model Relay branch; connect CI/Issue evidence; run supported platform entry where available.
- [ ] Independent review, affected regression, commits/PRs and final status with verified links.

## Execution notes

- Existing worktree is isolated on codex/opensandbox-workflow; unrelated assessment/workflow docs and outputs are left untouched.
- 112 API is reachable through approved execution networking. Existing tmux ds112 is idle and logged in; no new server or privilege changes were performed.
