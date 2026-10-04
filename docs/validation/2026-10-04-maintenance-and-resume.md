# Framework maintenance checks and stage recovery

Validated on 2026-10-04. Reusable implementation lives in `examples/github`; the product Agent has not been granted workflow/framework editing permissions.

## Maintenance PR gate

Source revision `9c4ddbb` adds a maintainer command, configured file scope and argv checks, exact PR head/main validation, persistent receipts, failure recovery, workflow syntax checking and pinned installation dependencies. Unregistered PRs no longer silently wait for a result. Registered product PRs continue independent QA/review and cannot use the maintainer command.

Real repository validation:

- [reading_list PR #102](https://github.com/big91987/reading_list/pull/102), head `1de7a348d5fa525e6c0717fc6ca82e1fe8c064ca`, synchronized the reusable workflow template and documented the supported maintainer entry point.
- [Actions run 37195279547](https://github.com/big91987/reading_list/actions/runs/37195279547) successfully scanned it and explicitly reported `pipeline/refresh` failure: maintainer verification required; no Agent dispatched.
- The supported `pr_refresh.py --maintenance-pr 102 --workspace <clean-pr-checkout>` command ran 11 deployment tests and parsed/syntax-checked both deployment and refresh workflows. It verified main `3376c95e4042b1f9e0f514b28091761914ca2e58` before and after the checks and reported success for the exact PR head.
- GitHub reported CLEAN/MERGEABLE and successful `pipeline/refresh`; PR #102 merged as `b21c4a1c8875a727adc5eab7af90a8bc70b9ac3e` using normal squash merge with head matching, without administrator bypass.
- Check logs and receipts are in the private repository registry, under `maintenance/`; no tokens, local paths or user data are included here.

## Deployment PR #99 after merge

At merged commit `3376c95e4042b1f9e0f514b28091761914ca2e58`, the real deployment test suite passed 11 tests including subprocess cancellation, rollback and data preservation. Deployment YAML/bash syntax passed.

After PR #102 merged, [new deployment run 37195462828](https://github.com/big91987/reading_list/actions/runs/37195462828) prepared successfully and superseded [run 37192834836](https://github.com/big91987/reading_list/actions/runs/37192834836), which was cancelled. This verifies real Actions latest-run cancellation for an older waiting deployment. The new deployment waits at the existing environment approval gate. No approval was bypassed and no claim is made that the new main has been activated. Production preview remained at commit `10c5e03d04bad026524e20476efd124db7e3a907` when checked. Activation interruption is covered by subprocess regression tests, not by deliberately interrupting the live site.

## Accumulated evidence blocked QA return

Issue #100's QA return to development failed in `controls(workspace)` with `Workspace exceeds import limit`. The real workspace contained 1,241 files totaling 60,067,634 bytes; control fingerprinting unnecessarily loaded screenshots and other unrelated artifacts against a 60 MB full-import budget. Python quality discovery had the same coupling.

Revision `78d939f` filters control and Python file sets before reading and applying their budgets. Full-import limits, protected-file content/mode hashes, symlink rejection and oversized-control-file rejection remain intact. Original upstream attribution is retained in `tooling/SOURCE.json`, with local file hashes for the adapted sources.

A regression with 62 MB of unrelated evidence failed before the fix and passed afterward. The real retained workspace now yields 85 control files and 16 Python files without deleting any evidence or changing task state. [Run 37194509333, attempt 2](https://github.com/big91987/reading_list/actions/runs/37194509333/attempts/2) was retried through GitHub's failed-job entry point; the development conversation entered running state. This confirms recovery into development, not completion of product rework or QA acceptance.

## Verification and remaining boundaries

`scripts/verify.sh` passed after the checkpoint fix: SDK 7, GitHub adapter 96, local deployment 11 tests, frontend/browser checks, Ruff, Go vet/race/build. The native linker emitted its existing LC_DYSYMTAB warning; Go tests and build exited successfully.

Regression coverage includes failed engineering commands, stale main/head, policy change, dirty workspaces, out-of-scope files, missing checks and malformed shell syntax. Those failure cases are local regression tests; the live GitHub test covers missing verification → checks → successful normal merge. A second independent repository has not been run end to end. Installer and upgrade instructions ship with the example; source PR #1 remains subject to owner merge.

## Report-stage browser output initialization

The latest Issue #100 report run failed writing `runner-checks/feature/accessibility-44.json`: the direct Node entry point did not create its output directory before AX evidence was written. The MCP wrapper created it, while Playwright screenshots created it later, hiding the first-run defect in other entry paths.

Revision `c5a1dbe` makes the browser CLI create its own output directory before actions and exposes delivery verifier stderr in the report job. A real-browser direct-entry regression failed with ENOENT before the fix and passed for both first run and retry afterward. Full verification passed (GitHub adapter now 97 tests), and the real Issue #100 gate passed all 8 checks, including 16 existing and 48 feature browser actions.

[Report run 37198874408, attempt 2](https://github.com/big91987/reading_list/actions/runs/37198874408/attempts/2) then passed that gate and encountered a separate, legitimate delivery boundary rejection: changes to `scripts/install_local_preview.py` are outside the current published product scope. The task also modifies `scripts/local_deploy.py` and adds `scripts/recommendations.py`, as anticipated in its accepted product design. The current adapter's product digest/publication allowlist does not support those backend/deployment artifacts. No allowlist was broadened, no protected changes were silently published and no new PR was created. Framework integration review and a consistent artifact/verification boundary are still required before delivery can complete; the browser fix alone does not make the report Run successful.

## Backend delivery scope and fresh verification

Revision `be458f5` repairs the delivery boundary in the reusable example. Trusted repository configuration can name exact `extra_product_files`; directories, globs, symlink paths and Harness controls are rejected. The same scope feeds the QA fingerprint, publication and review. A scope change invalidates old QA. Publication checks both sides of renames, stages deletions and non-ASCII names literally, and refuses any unconfigured script. Script conflicts still require maintainer resolution. Verification now discovers all `tests/*_test.py`, instead of running only deployment tests.

For reading_list, the maintainer reviewed the accepted backend integration and configured three explicit files: `scripts/recommendations.py`, `scripts/local_deploy.py`, and `scripts/install_local_preview.py`. This is persistent repository configuration, not an Issue exception. The endpoint serves only the validated public catalogue; private settings remain inaccessible. Collector service installation is optional and does not enable collection. Existing preview data paths remain separate. This review does not install or activate the controller or authorize product merge; main synchronization and merge checks remain applicable.

The QA-fingerprint regression failed before implementation. The actual Issue #100 workspace passed all 8 Runner checks with the expanded Python check running all 47 tests, plus existing browser/Node checks. Local regressions cover extra-file changes, scope changes, path traversal, protected files, directories, symlinks, sibling scripts, deleted files and non-ASCII filenames. Full verification passed on the final implementation: SDK 7, GitHub adapter 101, local deployment 11 tests, frontend/browser checks, Ruff, Go vet/race/build.

[Fresh QA run 37203181849](https://github.com/big91987/reading_list/actions/runs/37203181849) was launched using the existing `after=verify` workflow entry point. It resumes the existing QA conversation with a new verification round. No database/checkpoint or handoff receipt was manually changed. That Run completed successfully and its observer continued the same invocation until QA finished. The actual report handoff is recorded below.


### Completed real report delivery

The fresh QA completed 12 acceptance criteria, 47 Python tests, 25 Node tests and 209 browser actions. Its historical extra command still used the workspace's old `full_harness/quality.py`, which hit the import budget and led the QA worker to losslessly archive evidence. Revision `e8d461e` supplies the trusted host verification command to QA/review as well as development, and includes configured Python backends in that command's Python lint/format checks. It does not overwrite protected legacy workspace tooling. The current QA honestly documented one earlier rolling AX snapshot overwritten by a repeated gate; current-round evidence is complete. Do not claim all historical snapshots survived byte-for-byte.

The final implementation passed `scripts/verify.sh`: SDK 7, GitHub adapter 102, deployment 11 tests plus frontend/browser, Ruff and Go checks. The backend-quality regression failed before implementation and passed afterward. The actual report reran **9** checks, all successful, including all 47 Python tests and the trusted bundled Python quality command.

[Report Run 37204223873](https://github.com/big91987/reading_list/actions/runs/37204223873) completed successfully and created [draft PR #103: 书籍推荐、类型与简介](https://github.com/big91987/reading_list/pull/103), head `3f93282a7e6d084bb370079ac227b7ac88da9cea`, for Issue #100. This verifies the supported QA → handoff → report → push → PR creation path with the retained workspace, not a mocked or manually assembled PR. The PR is open/draft and behind main; Ready PR synchronization, any protected-script conflict handling, independent review and merge remain subsequent work. No merge, preview activation, collector installation or source PR merge was performed in this recovery.

## Ready PR updated remotely before refresh

PR #103 was marked ready and updated on GitHub: remote head `4e50635f011aa1e4899d90462fa634267d72a7c7` merged main `b21c4a1c8875a727adc5eab7af90a8bc70b9ac3e` into the delivered head `3f93282a7e6d084bb370079ac227b7ac88da9cea`. Both triggered integrate runs rejected the older local HEAD before publishing a status, leaving the required `pipeline/refresh` absent.

Revision `19f4510` fetches and verifies the exact current remote head and permits only a clean fast-forward of the registered branch. Dirty work, divergent local history, existing Git operations and a moving remote are rejected without reset/stash/force push. Preparation failures report an explicit failed check. Real Git regressions cover the original remote-main merge, preservation of dirty/divergent work, remote races and retry after fast-forward before state save. Full verification passed: GitHub 107, SDK 7, deployment 11 tests and frontend/browser/Ruff/Go checks.

[Original failed Run 37205361857, attempt 2](https://github.com/big91987/reading_list/actions/runs/37205361857/attempts/2) resumed through GitHub's failed-job retry. The registered local HEAD safely advanced to the exact PR head and persisted a new integration QA round with no conflicts. This verifies recovery into independent QA; the final combined QA/review check is still pending at this recording. The required check was not removed, no pass was fabricated, and neither product nor source PR was merged.


## Independent Review host tools (2026-10-04)

- Root cause: setup registered browser tools only for design/development/QA;
  the MCP stage parser excluded Review and the Runner skipped Review workspace
  registration. Its prompt instead suggested shell verification, causing a native
  Chromium sandbox denial. The first approved invocation also used an unsupported
  `code-review-checks` evidence name and exited with argument error.
- Fix: register/discover/bind the same `check`/`verify` tools for Review on both
  installation and `--tools-only` upgrade, accept the Review stage, register its
  workspace, and direct quality checks to zero-argument `verify`. Review continues
  to have no handoff tools. Keep explicit existing approval policies and custom
  instructions. Handle old API responses that omit empty `tool_servers`.
- Regression: missing Review binding failed before the fix; 108 GitHub example
  tests passed, including a real Review MCP browser success and intentional
  assertion failure. Separate old-install upgrade regression passed after fixing
  the omitted field. These are framework regressions, not the post-merge E2E.
- Applied through the documented `setup.py --tools-only` entry. Admin API confirmed
  `browser-review` discovery of `check` and `verify`, both attached with `auto`,
  matching the QA browser binding. No platform restart or product code change.
- Existing review conversation `40e79038d1519aea81e5c64a0d03e425` retains its old
  tool snapshot. At verification time its second native approval remained pending.
  The installation does not retrofit that in-flight session or resolve approvals.
  PR #103 was not merged, its gate was not bypassed, and post-merge E2E has not run.
