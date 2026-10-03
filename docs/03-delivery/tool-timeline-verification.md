# Native tool timeline verification

Status: complete. Native tool presentation and the new Issue #74 browser-driven pipeline trial passed, ending in a draft PR after independent Runner checks.

## Change

The conversation now merges native messages and tool items in event order. A tool
item is identified by its input message and native item ID; output deltas update
the same card, and completion exposes the actual result, exit code and duration.
Errors remain visible. Parameters, full results and the original event envelope
are expandable; messages are not rewritten or interpreted as business state.

Presentation follows the lifecycle strategy in Codex:

- `codex-rs/tui/src/chatwidget/tool_lifecycle.rs`
- `codex-rs/tui/src/history_cell/mcp.rs`
- `codex-rs/tui/src/exec_cell/model.rs`

The existing persisted `native_item` is exposed on message responses to retain
message/tool ordering. No new workflow state or business-specific tool handling
was added. Large payloads are rendered only when expanded.

## Verification

- Platform verification script passed: JavaScript, Python SDK and handoff tests,
  Ruff, Go vet, race tests and build.
- Browser deployment caught an omitted static route for the new transcript asset.
  The route is fixed; one HTTP regression check resolves every script referenced
  by the actual entry page and verifies JavaScript is served.
- Issue #71 development history replays 56 native tool cards. The external
  `submit_handoff` call exposes its real parameters and accepted return value.
  Expanded/collapsed controls work and no browser console errors were observed.
- New product Issue: https://github.com/big91987/reading_list/issues/74.
  Requirements conversation: `2aa9b026fec2fb7be054ecee41383ebf`.
  Actual browser interaction confirmed live command cards, results and queued
  follow-up input while the Agent was running.

The stage handoff and final delivery below are evidence from this new trial,
not substitutions from previous Issue #71.

## Requirements handoff and prototype interaction

Browser review opened the actual PRD, then submitted approval as the integration
user. The native `submit_handoff` completed successfully once; history reload
restored 15 tools including that single handoff with no browser errors. GitHub
created design conversation `cc101be607453d32fbdf3c150b231efa`, and the design Agent
started from approved requirements without reusing the requirements thread.

The prototype was operated in a real browser: filter changes retained global
counts, add/read/delete updated counts, reload preserved state, and simulated
storage failure retained both the book and counts with an error. A four-digit
fixture remained legible at 390px without document horizontal overflow. These
are prototype observations, not product-release evidence. Native browser MCP
calls and their actual results were visible in the design conversation.

GitHub browser navigation timed out in this environment. The issue was created
with the authenticated GitHub CLI; platform dialogue, document review, approval
and prototype interaction used browser controls, not API substitutes.

## Design approval and development start

The browser user reviewed the actual design index, HLD and contracts and approved
A-001 after operating the prototype. The native design Agent saved that approval
and called `submit_handoff`; its actual result was accepted. GitHub then created
development conversation `776f3e5c420173d6b6251c87ebe6b3d2`.

Independent native threads are confirmed:

| Stage | Native thread |
| --- | --- |
| Requirements | `01a0f8db-0ea7-7c00-a2ea-c385d7dde47b` |
| Design | `01a0f8e3-cc72-72a0-aaed-af5884b763bf` |
| Development | `01a0f8fb-38ea-7183-b75b-b745240d81e1` |

The product acceptance browser holds a three-book baseline entered through the
unchanged product UI before implementation (one read, two unread). This will
verify that the implemented counters work with existing persisted data, rather
than only with a newly seeded test fixture.

## Product browser acceptance and regression-plan ownership

The real product (not the prototype) was operated through the browser. Previously
saved three-book data displayed 3/2/1; filter changes preserved global counts.
Read-state changes, add, rename, delete, reload, zero-category and completely
empty lists produced the expected counts. Empty and duplicate input retained
counts and displayed the product error. A 390px viewport had no horizontal
page overflow, and Enter activated filtering. These observations do not claim
physical-device touch or screen-reader acceptance.

A native browser call exposed a genuine integration defect: legacy core tests
matched button names without counters, and their location under `.harness/`
prevented product Agents from maintaining them. The external GitHub example now
uses `tests/browser/core.json` when present, retaining the legacy path only when
no product plan exists. Empty/invalid product plans fail. The protected execution
configuration is unchanged; product plans are reviewed with the product PR.
One focused boundary test covers migration selection and rejection of an empty
plan. The same plan is exercised independently by the delivery Runner.

A queued browser reply instructed the development Agent to preserve every legacy
business action/assertion and update only the three obsolete locator names.
The platform's full verification script passed again after this integration fix.
Native failed calls remain visible in the timeline rather than being erased.

## Final handoff and independent delivery

The browser Stop and Continue Queue controls were exercised before handoff:
the current turn stopped, retained the pending input and workspace, then resumed
the existing development conversation to process the correction. No persisted
conversation state was manually edited. The migration retained all 16 core
actions and assertions; only the three filter-button names changed.

The development Agent's external `submit_handoff` was accepted. The conversation
ended normally. Reloading the page restored all 64 native tool cards and exactly
one development handoff; no browser console errors were observed.

- Delivery run: https://github.com/big91987/reading_list/actions/runs/36919733520
- Draft PR: https://github.com/big91987/reading_list/pull/75
- Development conversation: `776f3e5c420173d6b6251c87ebe6b3d2`

The report Job passed. Its independent evidence under
`docs/05-validation/tasks/74/runner-checks/` records six commands with exit code
zero, 16 existing-journey actions and 72 feature-journey actions, both passed with
no browser errors. The Runner committed the delivery and left the task workspace
clean. The PR remains a draft; it has not been merged. Physical touch,
screen-reader and other explicitly deferred human acceptance remain unverified.

The platform verification script passed with the final code: six frontend tests,
six SDK tests, three integration boundary tests, Ruff, Go vet, Go race tests and
build. This is a tested local deployment; the platform repository has no remote,
so these platform changes are not represented by the product draft PR.

## Current presentation: a flat chronological transcript

Per the latest accepted interaction, there is no turn-end grouping, reordering,
or automatic process collapse. Native thinking summaries, Agent messages, and
tool calls render in their original sequence. Tool completion updates its status
and output in place; turn completion leaves the same layout intact. Each turn has one Agent avatar/name above its chronological thinking, text and
tool entries, both while running and after completion. Intervening user messages
remain in place; continued output from the same turn does not repeat the name.
Tools are never children of a user message. Parameters and raw events remain
manually inspectable. Tool details now default to collapsed as described below.

Seven focused frontend checks pass, including chronology across intervening user
messages, summary delta/final deduplication, Agent tool ownership, and unchanged
visible output after turn completion and replay. The local service rebuilt with
no active conversations. Browser replay confirmed all tool rows belonged to the
Agent, no process groups existed, and prior tool outputs remained visible.

Browser verification confirmed a completed turn has one Agent name and avatar,
followed by progress, the actual command/output, and the final reply, in order.


## Compact execution feedback

Execution feedback no longer occupies a separate panel between the transcript
and input. A compact status disclosure sits in the composer footer; its popover
contains connection, queue, elapsed-time and failure details. Failures and failed
status synchronization open the details on transition. The active turn shows a
ring around its Agent avatar. Before any native output arrives, a short processing
placeholder appears there and is replaced by native content when available.
Native content keeps the same chronology and never gains synthetic Agent prose.
The existing frontend checks cover waiting, first output and completion. Browser
checks confirmed the closed footer status occupies about 30 pixels and its
popover shows actual execution timing and queue behavior.


## Compact tool disclosures

Tool calls stay in chronological order under the Agent. Running and completed
calls default to one row with the native tool name, an optional truncated native
title/command, lifecycle status, and a disclosure indicator. Parameters, complete
results, exit code, duration and the original event are available on expansion.
A manually expanded tool stays expanded across output updates and completion;
there is no turn-end regrouping. Failure remains visible in the collapsed row.
Existing frontend checks cover collapsed output, expansion, completion, replay,
chronology, and escaped tool output. No model-generated summary is introduced.

Verification for this change: seven frontend tests, Go vet, build and diff checks
passed. Browser interaction against the actual renderer with simulated events
confirmed default collapse, explicit expansion, live-to-completed preservation
of manual expansion, manual collapse, and a visible failure indicator. This is
presentation verification, not a new native execution or pipeline acceptance.
The rebuilt binary was activated with the file-preview update after the live
service became idle. The real design conversation now shows compact tool rows.


## On-demand workspace preview

The conversation sidebar is hidden on entry. The sidebar selector exposes files,
file preview, conversation information, or execution logs; closing it restores
conversation width. Workspace files in native Markdown links and inline code
references open in the right panel, including workspace-absolute paths and line
references. Files can also be searched and opened from the file list. Markdown
previews render tables and resolve links relative to the current document;
source view, images and downloads use the existing authorized artifact endpoint.
Single-file HTML uses an opaque-origin sandbox: inline script/style can run,
but platform APIs, network resources and browser storage are unavailable. This
is document preview, not hosting a multi-file application. Other binary or text
files larger than 2 MiB retain a download action.

Verified on the deployed service using the existing Issue #76 design conversation:
- Initial load and reload hide the panel.
- The reply's desktop/mobile screenshot links load actual images in the panel.
- The design index citation opens readable Markdown; source mode and document
  relative links work, including links to another screenshot.
- The single-file prototype displays in its isolated iframe.
- The file list filters and opens files; information/log views can be selected.
- Desktop (1440px) keeps chat and preview side by side; narrow (390px) preview
  overlays within the viewport and can be closed, without page overflow.
- No browser console errors were observed during these checks.

Eight frontend checks, targeted artifact authorization/path/static-route Go tests,
Go vet, build and diff checks passed. The service was restarted only while idle.

## Persistent event recovery (2026-10-03)

Conversation refresh now loads persisted execution events in cursor-based pages,
independently of SSE delivery. SSE continues to deliver live events. Both paths
deduplicate by event ID; a newer streamed event cannot discard older history.
Opening a conversation restores history before presenting its transcript, with
native tools and public thinking summaries kept in chronological order and
collapsed by default.

The regression test reproduced missing tool rows without SSE before the fix and
passed afterward, including history beyond 300 events and out-of-order arrival.
A real development conversation restored 39 tool calls and 10 public thinking
summaries after browser reload; expanding tool details was also verified. No
conversation records were edited. Full frontend, SDK, GitHub adapter and Go race
checks passed before publication.
