# Conversation streaming verification

Verified on 2026-10-01 against the installed native Codex and a separate local platform instance.

## Runtime contract

All Codex turns use the app-server protocol. `item/agentMessage/delta` accumulates text under the native item ID; persisted `item.updated` events reach the browser through SSE. Completion updates the same message instead of appending a duplicate. Tool execution and final turn completion remain separate events.

The prepared native configuration, workspace, selected Skills, trusted Hooks and thread ID are preserved. Conversation replies default to natural language/Markdown; an explicitly requested structured response is still allowed. This does not implement an API output-schema option or a business pipeline handoff.

## Evidence

- Before the transport change, the real ordinary execution test produced zero text updates and failed the streaming assertion.
- After the change, the same test receives multiple updates, uses the selected progressive Skill and external MCP tool, runs the native Stop Hook and resumes its native thread.
- Native tool verification passed for automatic execution, explicit acceptance and refusal, including same-thread continuation and no write after refusal.
- A real browser conversation produced 1,212 text updates. The first arrived at 14:30:23 UTC, the last at 14:31:12 UTC; turn completion followed at 14:31:14 UTC. The browser visibly displayed a growing natural-language reply while execution was active, then displayed the completed reply and idle status.
- The project verification script passed formatting, Go vet, race-enabled Go tests, browser presentation tests, Python SDK tests and build. The macOS linker emitted its existing LC_DYSYMTAB warning during race-test linking; the tests passed.

Private native histories and screenshots are local verification artifacts and are not committed.
