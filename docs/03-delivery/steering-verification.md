# Queued message steering

A queued user message offers an “引导” button while its conversation is running.
The authenticated POST /api/conversations/{id}/steer endpoint accepts message_id;
it reserves only that conversation's queued input, then sends Codex turn/steer
with threadId and expectedTurnId. It does not stop or restart the execution.

Native acknowledgement changes steering to steered. Explicit rejection restores
the queue; uncertain delivery is failed and never automatically replayed. Inputs
not selected stay in queue. Duplicate clicks and cross-user requests are rejected.
Stop remains a separate operation. Platform restart marks unresolved deliveries
failed. No development-stage concepts are added to the generic platform.

## Verification

Full scripts/verify.sh passed: web, SDK, nine pipeline integration tests, Go vet,
race tests and build. Focused tests cover native success/rejection/disconnection,
queue ordering, duplicate submission and endpoint authorization.

Through the actual platform webpage, a fresh generic Codex conversation executed
a waiting command. The operator submitted a second message, clicked “引导”, and
observed “已引导” followed by the requested STEERED-BLUE answer. The persisted
record contains exactly one platform.execution.started, one turn.started, one
platform.input.steered and one platform.execution.completed. Reload retained the
accepted input and reply. This demonstrates actual native steering, not a mocked
transport. No product conversation was modified for this check.

Conversation: 92805867e2bac41fdcfc44a4380ca43a.
