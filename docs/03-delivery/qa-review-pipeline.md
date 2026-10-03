# QA feedback loop and independent code review

## Agreed behavior

The external CI integration owns the product flow. Agent Platform remains generic.
Requirements, design, development and QA each keep their own conversation while
sharing the task branch/workspace. On a product defect, QA chooses requirements,
design or development and submits evidence. A new Workflow Run resumes that role;
all progress stays on the original Issue. Environment failures do not become
product failures. QA must explicitly select its destination; missing target_stage
is rejected rather than defaulting to PR delivery. Required requirement/design changes go through human review.

After passing QA, the human confirms draft-PR delivery. Code review is a separate
manual workflow entry after PR creation: an independent Agent posts its findings,
and humans decide what to change and whether to merge. Review has no handoff tool
and is not an automatic gate in the QA repair loop. No product PR auto-merge.

## State and recovery

The trusted local registry stores the active stage, round, stage invocation
receipts and last accepted handoff. QA return advances the round, so old callbacks
cannot use previous QA evidence. Immutable UTF-8 document snapshots preserve
handoff evidence; the persistent task worktree holds code, images and current docs.
The same-role native conversation resumes with new input, not a different role's
history. CI concurrency and a per-Issue file lock serialize dispatch consumption.

The sender retains an immutable receipt before dispatch. Accepted requests are
not replayed; rejected/uncertain requests may be explicitly retried unchanged.
The receiver deduplicates using the accepted transition and stable invocation
request ID. Rework Issue notices include the return round, so an identical
unresolved defect remains visible on a later loop while same-round retries deduplicate. It rejects an old callback after the task has advanced. A new Agent
input cannot be inferred merely from a green CI run: Agents may wait for people
or run beyond the CI observation timeout.

Human acceptance is currently enforced through Agent instructions, not a separate
machine-verifiable approval record. The receiver validates stage routing, document
integrity and product digest. Same-machine shared storage is required; no claim
of distributed sandbox migration or remote storage replication is made.

## Verification so far

- Complete local verification script passed: existing web and SDK checks, eight
  focused Python integration tests, MCP HTTP retry/routing tests, Go vet/race tests.
- Simulated integration checks cover QA return to each permitted role, preserved
  conversations, re-execution of subsequent stages, stale/repeated callbacks and
  publication without a code-review gate. These are not real Agent E2E evidence.
- Test-repository workflow maintenance PR #81 merged at
  `889ad317193998f93fe096b72959a2d438d18b21`.
- Earlier real Issue #80 QA passed AC-01–07 with 14 browser runs and 218 actions,
  and remained idle awaiting confirmation under its original configuration.
- The real product run completed the forward path through draft PR delivery.
  QA found no product defect, so actual Agent-selected backward handoff remains
  unobserved. The three return routes and repeated cycles are covered by the
  integration tests above; do not describe them as a real failed-QA E2E run.

## Current real run

Product Issue #82 (undo the most recent book deletion) uses one task workspace
and distinct requirements/design/development native Codex sessions. The operator
answered the name-conflict question, approved PRD, reviewed the running prototype,
and approved the revised design through the platform webpage. Requirements,
design and development dispatches are Actions runs 37002535752, 37003725865 and
37007980863 respectively.

The operator exercised the actual product through the browser: restoration keeps
read state, list order, active filter and another book's unsaved editing draft;
consecutive deletions restore only the latest; duplicate-name refusal preserves
the opportunity and succeeds after renaming; restored state survives refresh.
These are operator observations, not an independent QA result.

Development reported 12 real browser plans / 517 actions and nine unit checks,
including a narrow-screen long-title overflow that it found and fixed. Its shell
verification encountered a sandbox listen EPERM. The same standard verify.py
command subsequently passed in the Runner environment, including core and feature
browser plans, and saved runner-checks evidence. The operator read validation.md
and confirmed handoff to independent QA through the webpage. The registered MCP
dispatched Actions run 37010994014 and created QA conversation
`1b99ef5d9649c895189ae5a47cd9cab3` in the same workspace. Independent QA executed 16 real browser runs / 615 actions and found no product
defect. Its accepted report preserves untested system IME/screen-reader scope.
The operator reviewed the report and approved only draft-PR creation through the
webpage. QA called submit_handoff with target_stage=report; Actions run
37013718462 succeeded and delivered draft PR #85:
https://github.com/big91987/reading_list/pull/85

The final Runner repeated all six standard checks successfully. Delivery head is
`aacc594904bbabbb8d429bd5df1123d59b0e4038`, on `codex/issue-82-platform`; the PR is
open, draft and unmerged. Four distinct native Codex sessions were confirmed for
the four roles, and the product digest remained unchanged throughout QA.
No real failed-QA return occurred; no claim of actual backward-loop execution is
made. Independent code review was validated separately on Issue #74 below, not
claimed as a review of PR #85.

## Browser acceptance prerequisite

The undo feature includes failed localStorage writes and subsequent retry. The
shared browser tool now supports `storage_write_failure` with a boolean `enabled`.
The fault affects only current-page localStorage.setItem and ends when disabled
or reloaded. It never seeds storage or changes product code. The existing real
Chromium tool probe covers failed write, unchanged saved data, recovery and actual
readback, with Escape and Unicode download checks retained. These synthetic-page
checks establish tool behavior, not product QA success.

Source PR: https://github.com/big91987/he_skeleton/pull/39 (open for review).
Test repository maintenance PR: https://github.com/big91987/reading_list/pull/83
(merged). Formal install_light.py synchronization pins source commit
`09d1aeb530a99e25f84db0453a45aae306b612d1`; reinstall reports no changes. The trusted
Runner tool checkout was fast-forwarded to the merged test-repository main, and
standard external MCP discovery refreshed the supported-action description.


Independent review entry was dispatched separately for existing test Issue #74 /
draft PR #75 in Actions run `37006210086`. It created reviewer conversation
`d9ba5ae8b1a7768eb3bc37ab2b2125f1`; the browser shows the Reviewer inspecting actual
code and test differences with no handoff or merge responsibility. Review completed and Actions succeeded. Its findings were posted to PR #75 at
https://github.com/big91987/reading_list/pull/75#issuecomment-5952332426 and the
original Issue. No proven implementation defect was found; remaining acceptance
gaps were explicitly left for human judgment. The product digest is unchanged,
and no handoff/merge was triggered. This does not imply the older QA blockers
were resolved.

Test-repository PR #84 merged the matching Actions form description for explicit
QA destinations. Missing-target sender/receiver checks and two-cycle Issue notice
checks extend existing tests; no extra test family was introduced.

The standard platform verification script passed again after these changes:
web checks, SDK tests, eight pipeline integration tests, Ruff, Go vet/race and
build. This does not replace real QA feedback-loop acceptance.

QA initially treated untested system IME/screen-reader claims as mandatory gates.
The operator requested a clause-level review through the webpage, without
waiving a required criterion or inventing test evidence. QA reread the accepted
PRD/contracts and corrected these to explicitly untested scope, not blockers.
External QA instructions now require every release gate to trace to an approved
clause. This rule belongs to the integration, not generic platform code.

The reusable QA instruction correction was deployed through standard setup.py;
Ruff and the eight integration tests passed after this prose-only change.


## Ready-for-review entry verification

The owner marked product PR #85 ready at 2026-10-02 14:02:11 UTC, but no review
started: the installed workflow only listened to issues and workflow_dispatch.
Maintenance PR #86 added pull_request.ready_for_review and was merged at
0286bd39f254a1d8bc54fbb402a5adafdece1b7b. Nine integration tests pass, including
trusted delivery-record resolution from PR #85 to Issue #82 and rejection of a
wrong branch, commit, fork or uncommitted product files. Review start now posts
its conversation link directly on the PR.

Replaying Ready through the official GitHub CLI did not generate a Run for the
existing PR. Its merge ref 06220e05ff7c565e4a1847ee111db2929c647ca7 still uses base
be597911480d2c836ee694149e81cb5ef5b4c8e6 and does not include the new listener.
The browser connection timed out, so this is CLI event evidence, not a successful
webpage click test. A proposed default-branch pull_request_target listener was
blocked by automatic approval review as a security-boundary change; it has not
been applied and user approval is pending. Native-button end-to-end verification
is therefore incomplete; do not report it as fixed.

The existing manual code_review entry successfully started an independent review
of PR #85 in Run 37019371767. Conversation 933ee7f2080084cb2e97500f2c271bac and
its start notice are visible at:
https://github.com/big91987/reading_list/pull/85#issuecomment-5954551448
Review was running at this checkpoint. Product PR #85 remains open and unmerged.


## General backward handoffs (2026-10-03)

All product stages now use the same submit_handoff tool for the normal next
stage or any earlier stage. Requirements has no earlier stage. Explicit user
requests are sufficient; Agent-proposed returns require human confirmation,
except evidenced QA product defects. Review is still separate. The sender,
receiver and workflow agree on destinations. Every backward transition starts a
new round, retains role conversation IDs, and rejects obsolete callbacks.

The six earlier-stage routes pass integration tests covering continuity, retry
and invalidation. The full platform verification script passed. Test-repository
maintenance PR #88 installed the workflow routes; the local tool was rebuilt and
setup.py refreshed the standard stage configurations and tool discovery. Existing
conversations preserve their instruction snapshots; new configurations apply to
new conversations, while the shared tool binary is used on later invocations.
This is not a claim of a real Agent-selected failure/repair/QA cycle.
