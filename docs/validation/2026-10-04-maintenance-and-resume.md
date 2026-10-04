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
