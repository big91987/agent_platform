# GitHub handoff integration verification

Status: end-to-end draft-delivery trial passed. Product human acceptance remains partial as recorded below.

## Implemented path

External stdio `submit_handoff` → GitHub workflow_dispatch → Runner SDK → separate
stage conversation sharing a Runner-created task worktree. Requirements/design
wait for human approval; development uses an external browser tool and native
Stop check, then the report Job rechecks and publishes a task branch/draft PR.
No GitHub-specific endpoint or software-development state was added to platform core.

Reusable entry points:

- `examples/github/setup.py`: register tools and configure stage Agents.
- `examples/github/pipeline.yml`: trusted Issue entry and stage jobs.
- `examples/github/pipeline.py`: worktree, handoff, SDK and delivery integration.
- `examples/pipeline-tool/`: separately registered stdio MCP program.
- `examples/github/browser_tool.py`, `verify.py`, `quality/`: browser and shared quality checks.

## Evidence obtained

- Test repository workflow PR: https://github.com/big91987/reading_list/pull/70 (merged).
- Product trial: https://github.com/big91987/reading_list/issues/71 (rename a book).
- Initial automatic workflow: https://github.com/big91987/reading_list/actions/runs/36887889321 (requirements job succeeded).
- Requirements conversation: `b0e622487d41ec7a8fa8b62d7c0984f9`.
- Browser login as the integration user returned to that exact conversation and
  preserved its history/workspace. The Agent asked three concrete product questions.
- Agent configuration UI showed stage Skills and checked external MCP bindings.
- Existing platform verification suite passed, including native streaming/tool
  approval coverage. Go handoff isolation/deduplication and Python receiver tests passed.
- Trusted browser dependency/localhost/rendering probe passed. Existing reading-list
  browser journey produced `passed: true` in actual browser output.

## Confirmed first handoff

The user authorized continuing through the browser. The integration user submitted
all three recommended requirements choices, opened the PRD and G1 review in the
web preview, and approved the complete PRD through the conversation page.

The native requirements Agent invoked `submit_handoff`; the registered external
MCP returned `delivery: accepted`. GitHub run
https://github.com/big91987/reading_list/actions/runs/36891924332 started the design
job and published conversation `90c968e1da34281ad864216c4978f12e`.

The requirements native thread is `01a0f82c-a5bb-7ea1-b985-fbebdc392b30`; design
uses a distinct thread `01a0f849-87d9-7440-b1c9-a86b32e1fd9b`. Browser inspection
confirmed the design Agent started with the approved handoff and shared task
workspace. The stage Skill sets are separate. Requirements intentionally emitted
an empty final message for the handoff under the existing repository instruction;
the next conversation link is available in GitHub, not in that empty reply.

## Confirmed design handoff and product inspection

The integration user reviewed the runnable prototype, HLD, contracts, architecture
decisions and G2 review through the platform web UI, then approved design there.
The external MCP returned `delivery: accepted` and GitHub run
https://github.com/big91987/reading_list/actions/runs/36896021565 created development
conversation `af228025c7ae574e673a20f923e3fb2f`, native thread
`01a0f868-5999-7a40-be03-71aa1fa0e183`. It has its own development Skills and native
Stop check. No previous stage session was reused.

The shared browser checker rejected Escape despite the approved keyboard contract.
The reusable source fix is https://github.com/big91987/he_skeleton/pull/37
(pending source-repository merge). The test repository installed source commit
`4f36c33fc98885aac85d61989364a7009155d9fb` through the formal installer; consumer
https://github.com/big91987/reading_list/pull/72 is merged. A fixed Escape probe and
the design Agent's formal browser recheck both passed. Earlier failed evidence was
retained. Binary screenshots remain in the shared workspace with a SHA-256 index;
`submit_handoff` transfers UTF-8 document snapshots and does not accept PNG files.

Browser interaction also confirmed the implemented product's empty/duplicate-title
errors, Escape cancellation, trim-and-save persistence across reload, retained read
state/order/count, filter-switch draft discard and no horizontal overflow at 320px.
These checks used the actual product, separately from the prototype. They are
supplemental observations while development is still running, not final release
acceptance. Native message delta events and progressive web text were observed.

## Trial findings addressed

- The browser checker lacked Escape support; fixed in the reusable upstream checker
  and installed at a pinned revision (links above).
- Agent and Runner verification previously wrote the same evidence directory.
  `verify.py --evidence-name runner-checks` now keeps Runner results separate from
  the Agent's `delivery-checks` failure evidence.
- The development Agent treated incomplete OS IME/screen-reader evidence as a bar
  to creating a draft PR. The integration prompt now explicitly distinguishes
  draft review delivery from complete human acceptance; missing human evidence
  remains Partial and must not be reported as passed. A browser follow-up supplied
  that instruction to the already-running trial conversation.
- Sending that follow-up while a turn was running visibly queued it; after the
  first turn ended, it started in the same conversation rather than interrupting
  or replacing the native session.

Ruff lint/format and the existing Python handoff boundary test passed after these
integration-script changes. No platform-core business interpretation was added.

## Final delivery verified

The development Agent invoked the external MCP and received `delivery: accepted`
with digest `bd9ca3f019bc260e9e0fe5adf2ca38a1ef8e79a6305507086d7322bc01f8307a`.
GitHub report run https://github.com/big91987/reading_list/actions/runs/36900960478
completed successfully on attempt 3 and created draft PR
https://github.com/big91987/reading_list/pull/73 at commit
`8023907cff5bd3e05fb744f911773f99c5744c71`.
The Issue received its delivery receipt:
https://github.com/big91987/reading_list/issues/71#issuecomment-5937145060.

The first report attempt failed before verification because the unattended Runner
could not read the desktop login keychain. Delivery now reads an explicitly
configured private `delivery_token_file` (0600), with deployment instructions and
one focused credential-boundary test. The second attempt passed all checks but
rejected Skill-produced `.trellis/spec/` documents. That exact documentation
subtree is now permitted alongside product/docs/tests; other control files remain
outside the delivery allowlist. Both repairs are in reusable integration code;
no task state was edited or Agent handoff replayed. Only the failed report Job was
rerun, using the saved digest and workspace.

Runner evidence contains six successful checks: diff whitespace, Prettier, ESLint,
JavaScript syntax, fixed core journey (16 actions), and feature journey (62
actions). The Agent additionally retained 18 browser integration cases and actual
keyboard/mobile evidence, including a failed fixture run and its corrections.
The task worktree is clean after publication. Earlier Agent sandbox failure logs
remain separate from successful Runner evidence.

Browser-operated acceptance covered requirements clarification/approval, design
review/approval, continued same-session dialogue, real streamed messages, queued
input, prototype interaction, and the actual product's edit/save/cancel/error,
filter/read-state persistence, delete/reload and 320px layout. Final platform UI
shows the development turn ended and the actual accepted handoff message.

## Explicit remaining product acceptance

The end-to-end platform/GitHub draft-delivery path has passed. Product AC-12 still
requires real OS Chinese IME and screen-reader evidence; synthetic composition
and DOM/ARIA checks do not replace that evidence. PR #73 remains a draft and the
feature has not been merged. This is not a claim of complete product acceptance.

Upstream browser fix PR #37 remains pending source-repository merge; the consumer
uses its pinned code commit through the formal installer. Test repository PR #72
is merged. No unapproved source-repository merge was performed.
