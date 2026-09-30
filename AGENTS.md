# Agent Platform

Build the smallest useful generic Agent platform. Go backend, same-machine native executors, platform-owned persistent conversations, API and browser continuity. Do not hardcode software-development stages or webpage output.

Use the product PRD and architecture as the shared contract. Native executors own reasoning, Skills and Hooks; the platform owns authorization, persisted input, scheduling, process lifecycle and presentation.

Keep real behavior verifiable. Repair supported paths rather than patching individual conversations. Never silently replace a missing native session or replay uncertain side effects. Secrets, native histories, generated workspaces and screenshots from local experiments stay in ignored storage.

Use gofmt and go vet. Keep tests small and focused on meaningful behavior: isolation, queue ordering, cancellation, restart, deduplication, authorization and real executor continuity. Do not pile up assertions about constants, prose or implementation structure.
