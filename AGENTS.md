# Agent Platform

Build the smallest useful generic Agent platform. Go backend, same-machine native executors, platform-owned persistent conversations, API and browser continuity. Do not hardcode software-development stages or webpage output.

Use the product PRD and architecture as the shared contract. Native executors own reasoning, Skills and Hooks; the platform owns authorization, persisted input, scheduling, process lifecycle and presentation.

Keep real behavior verifiable. Repair supported paths rather than patching individual conversations. Never silently replace a missing native session or replay uncertain side effects. Secrets, native histories, generated workspaces and screenshots from local experiments stay in ignored storage.

Use gofmt and go vet. Keep tests small and focused on meaningful behavior: isolation, queue ordering, cancellation, restart, deduplication, authorization and real executor continuity. Do not pile up assertions about constants, prose or implementation structure.

## Native output presentation

Display native Agent messages without rewriting, summarizing, or interpreting business fields. Preserve tool calls and results as execution events; do not synthesize narrative progress from them. Platform indicators describe only execution lifecycle, connection, queue and pending tool approval. Externally registered handoff tools use the same presentation and approval path as other tools.

## Product interface style

Organize pages around the user's task, choices, results and next actions. Keep compatibility handling, protocol versions, migration history and implementation commentary in code or maintenance documentation unless the user must act on them.

Show help or warnings only when they explain a current choice, a meaningful consequence or a real problem the user can resolve. Describe settings by their observable behavior in plain language; do not explain tool registration or server enforcement in ordinary field help. Necessary technical configuration can retain precise technical names.

Compatibility must not add routine configuration work. When migration or a conflict requires user action, explain the concrete effect and provide the relevant action at that point. Do not add reassurance about normal behavior or narrate development decisions.

Before delivering interface changes, remove explanations, duplicate status, internal identifiers and controls that do not help the user decide or act. These rules govern platform-authored interface content; preserve native Agent output under the rules above.

## Agent configuration and task input

Treat an Agent as a collaborator with a name, executor, model, role, Skills, tools and execution permissions. Reuse the same execution configuration form and validation in standalone Agents and workflow nodes. Each editable setting must survive save/reload and affect the supported execution path; show both executor and model, and let users edit displayed names.

Concrete work arrives through User Input, corrections and handoff. Do not add a separate Session Prompt or node work-description field for new workflows. Native workspace AGENTS.md supplies project rules; do not copy it into platform prompts. One Run shares its startup workspace. Add shared workflow variables only for a demonstrated need beyond this workspace, not as a general configuration framework.

Keep handoff/completion/wait controls specific to orchestration. Preserve frozen legacy runs during upgrades, and reject incompatible or externally edited configuration rather than silently dropping user instructions.
