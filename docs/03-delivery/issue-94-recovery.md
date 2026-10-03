# Issue 94 browser validation recovery

The development stage stopped because the declared browser tool could not perform hover-only interactions, touch taps, geometry checks, or storage-write observations. A direct shell verification also hit sandbox port restrictions. A later host Stop Hook succeeded, but the Agent report still described the older failure. The Runner returned after 900 seconds and lost the final reply that arrived later.

## Changes

- Extend the reusable Harness browser plan with bounded hover/pointer/tap, first-step touch contexts, geometry snapshots/comparison, and storage write observation. Plans still cannot execute arbitrary scripts or external network requests.
- Allow an owner-managed, pinned browser source checkout through `setup.py --tools-only --browser-source <harness-checkout>`; the task checkout is unchanged.
- Expose host verification as an externally registered MCP tool. Include existing product unit tests and deployment server tests in that gate. Failed attempts replace the current success receipt with actual failed records.
- Continue timed-out result observation in a separate GitHub Run, bound to the same conversation and input. No Agent invocation or Session replacement occurs. Ignore replaced inputs/iterations and deduplicate Issue comments.

## Evidence

- Real Chromium tool contract: hover, touch, geometry, storage observation, plus a negative storage-write case passed.
- Existing real browser integration and three browser-delivery regressions passed.
- External GitHub adapter: 58 regressions passed; Ruff check/format passed.
- The formal MCP host verification for Issue 94 passed all eight commands, including 13 Node tests and 8 deployment server tests.
- Existing development conversation resumed; no new Issue or native Session was created. The Agent reports 16 product plans and 851 real browser actions passed; independent QA remains a separate stage.
- GitHub Run 37122147826 successfully executed the new observation job and posted the previously missing design result to Issue 94. This was run against the repair branch, not an assertion that main already contains the workflow change.
- Workflow deployment is PR 95 in the product repository; product merges remain human-controlled.

Recovery completed: the original development conversation submitted its accepted handoff and GitHub Run 37124021843 started the independent QA stage. Task 94 is now `active_stage=qa`; QA has its own conversation and actual verification remains pending. No product PR was merged.

The observer is currently deployed through the already-tested repair branch via the documented `observer_ref` setting; workflow PR 95 remains unmerged. Browser source PR 40 has passed its CI.
