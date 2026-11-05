# Provider-independent architecture review implementation plan

**Goal:** Separate executable rule violations from model judgments, make the selected architecture requirements auditable, and exercise the same tool with the user-authorized DeepSeek V4.1 Flash endpoint.

**Architecture:** Keep the current parser and provider-neutral command boundary. Rules remain deterministic; model review must account for every selected check, distinguish scope notes from blockers, and submit exact source quotations whose locations the tool resolves. A model finding remains a candidate after citation validation. Do not alter frozen historical reports or introduce compatibility aliases for the changed review contract.

**Execution:** The user explicitly requested autonomous fixes and multiple real model rounds. Implement in this existing isolated worktree, using focused red/green tests, without another design approval loop.

## Tasks

- [x] Capture an actual DeepSeek baseline using the current tool and the same fixed Model Relay bundle; retain returned model ID, usage, timing and raw response.
- [x] Add failing behavioral tests for normal scope notes versus missing context, per-check coverage, exact unique quote resolution and ambiguous/missing evidence. Implement the minimum new review contract in review.py and configuration validation in engine.py. Add explicit rule results and model review sections, update current consumers and bump the tool version.
- [x] Add transport regressions for truncated/blank responses and configurable output bounds. Keep URL/token/model external, never substitute a model or retry failed API requests automatically.
- [x] Add a reusable scan entry that builds the bundle, executes the configured reviewer, validates its untouched JSON and retains separate raw/model/rule reports. Include relevant HTML/CSS, project verification entry and selected tests as explicitly declared context, not pretend parser support.
- [x] Update current tests, docs and scenario defaults; run the tool/scaffold/workflow suites and lint. Install/update through scaffold.py and verify source provenance.
- [x] Run at least two independent post-change DeepSeek scans on one fixed Model Relay source/standards snapshot, without seeding prior findings. Compare requirement coverage, evidence validity, consistent findings and cost; keep model accuracy claims limited to these runs.
- [x] Publish sanitized standard reports and source changes on the existing draft PR and update the linked Issue. No product refactor, merge, deployment or server-permission changes.

## Review focus

- Required checks cannot disappear or be duplicated while reviewed_files claims full coverage.
- A normal static-review boundary must not become an execution failure; missing required evidence must remain incomplete.
- Exact text found at multiple locations is ambiguous and must not be guessed; source drift still invalidates results.
- Rule violations cannot be cleared by model output, including empty or failed model results.
- No credential, private history or local absolute path enters published artifacts; API truncation never becomes a clean report.


## Actual result

Tool0.4.0: 43 tool tests,12 scaffold/install/workflow tests and Ruff passed. Independent review's artifact-isolation and source-version findings were reproduced and fixed; no remaining Important findings. Standard upgrade applied to Model Relay branch; current rule scan remains separate from model findings.

Seven authorized DeepSeek calls were retained: one0.3.1 baseline and six development/verification rounds. Final SSE round6 accounted for all11 checks and returned7 citation-checked candidate findings; round5 omitted coverage declarations and was rejected. Earlier full-line quote false rejection and HTTP body interruptions were retained and led to fixes. No hidden retry or hand-authored model conclusion. Actual112 Workflow execution/upgrade, product refactor, QA, merge and deployment remain outside this task's completed scope.
