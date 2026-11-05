# Provider-independent architecture review implementation plan

**Goal:** Separate executable rule violations from model judgments, make the selected architecture requirements auditable, and exercise the same tool with the user-authorized DeepSeek V4.1 Flash endpoint.

**Architecture:** Keep the current parser and provider-neutral command boundary. Rules remain deterministic; model review must account for every selected check, distinguish scope notes from blockers, and submit exact source quotations whose locations the tool resolves. A model finding remains a candidate after citation validation. Do not alter frozen historical reports or introduce compatibility aliases for the changed review contract.

**Execution:** The user explicitly requested autonomous fixes and multiple real model rounds. Implement in this existing isolated worktree, using focused red/green tests, without another design approval loop.

## Tasks

- [ ] Capture an actual DeepSeek baseline using the current tool and the same fixed Model Relay bundle; retain returned model ID, usage, timing and raw response.
- [ ] Add failing behavioral tests for normal scope notes versus missing context, per-check coverage, exact unique quote resolution and ambiguous/missing evidence. Implement the minimum new review contract in review.py and configuration validation in engine.py. Add explicit rule results and model review sections, update current consumers and bump the tool version.
- [ ] Add transport regressions for truncated/blank responses and configurable output bounds. Keep URL/token/model external, never substitute a model or retry failed API requests automatically.
- [ ] Add a reusable scan entry that builds the bundle, executes the configured reviewer, validates its untouched JSON and retains separate raw/model/rule reports. Include relevant HTML/CSS, project verification entry and selected tests as explicitly declared context, not pretend parser support.
- [ ] Update current tests, docs and scenario defaults; run the tool/scaffold/workflow suites and lint. Install/update through scaffold.py and verify source provenance.
- [ ] Run at least two independent post-change DeepSeek scans on one fixed Model Relay source/standards snapshot, without seeding prior findings. Compare requirement coverage, evidence validity, consistent findings and cost; keep model accuracy claims limited to these runs.
- [ ] Publish sanitized standard reports and source changes on the existing draft PR and update the linked Issue. No product refactor, merge, deployment or server-permission changes.

## Review focus

- Required checks cannot disappear or be duplicated while reviewed_files claims full coverage.
- A normal static-review boundary must not become an execution failure; missing required evidence must remain incomplete.
- Exact text found at multiple locations is ambiguous and must not be guessed; source drift still invalidates results.
- Rule violations cannot be cleared by model output, including empty or failed model results.
- No credential, private history or local absolute path enters published artifacts; API truncation never becomes a clean report.
