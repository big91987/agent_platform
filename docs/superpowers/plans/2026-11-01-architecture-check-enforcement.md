# Architecture Contract Enforcement Implementation Plan

> Use superpowers:executing-plans inline. The user approved implementing and verifying deterministic constraints; do not request another approval. Preserve existing unrelated changes and do not modify Model Relay or running environments.

**Goal:** Make the current architecture checker enforce real Go package boundaries and the approved Application/Domain separation, with an independently runnable SaaS reference.

**Architecture:** Reuse the existing parser adapters and rule engine. A Go package has one architecture owner. Optional `allow_external` restricts third-party imports while Go standard-library imports remain available. The SaaS reference separates business rules in `accounting/domain` from application operations in `accounting/application`; SQLite preserves complete atomic operations.

**Tech Stack:** Existing Python standard-library engine, Go official parser/build metadata, current locked Go/Node dependencies. No domain-model DSL, new runtime, external credentials or product migration.

**Spec:** User-approved deterministic template contract in this conversation; examples/arch-template/principles.md and tool/README.md, with the approved separation superseding the old same-package SaaS example.

## Global constraints

- Keep one reusable source in examples/arch-template; exports run without the platform worktree.
- Do not retain a second legacy SaaS package or silently accept incompatible configuration.
- Fail incomplete on unsupported scope, missing declarations or invalid source; never turn analysis failure into conformance.
- Keep rule checks, semantic review and real product QA distinct. Do not call model endpoints.
- Preserve atomic reservation, duplicate handling, settlement and rollback behavior.

## Review focus

- Two files in one Go package cannot pretend to be independent architecture modules.
- Domain code cannot introduce a new third-party dependency outside its explicit allowlist.
- HTTP cannot bypass Application to import Domain; Application cannot import SQLite.
- The split must retain tenant isolation, concurrency, idempotency and both-write rollback.
- Fresh exported projects must carry the rules and changed analyzer, without a source-worktree dependency.

## Task 1: Close deterministic-check gaps

Files: tool/archcheck/engine.py, tool/archcheck/analyzers/go.go, tool/tests/test_check.py.

- [x] Add failing real-parser tests for split Go-package ownership and third-party import allowlists, including standard-library and explicitly allowed imports.
- [x] Enforce one architecture owner per Go package; add official standard-library classification and `allow_external` matching.
- [x] Run the full tool suite, including parser/scope/semantic-review protocol regressions.

## Task 2: Apply the approved package structure

Files: backend/go/saas/template/internal/accounting/{application,domain}, httpapi, SQLite adapter, cmd, architecture.json, scenario documentation; tests/test_scaffold.py.

- [x] Add failing exported-project boundary tests for HTTP-to-Domain and Application-to-SQLite bypasses.
- [x] Split existing rule/application code into real Go packages, update callers and delete the old package files.
- [x] Keep the repository's atomic operation contract; preserve existing behavior tests and add pure quota-boundary coverage.
- [x] Run Go race/vet/build and architecture checks in the source and fresh exported projects.

## Task 3: Verify portability and report exact limits

- [x] Update tool/scenario instructions, version, and validation evidence; retain historical verification as historical.
- [x] Run complete tool/scaffold suites, both Go samples and Vue checks, a fresh exported setup/verify, and source-repository Go tests/vet.
- [x] Perform one fresh independent final review; fix important findings with failing regressions and rerun affected/full checks.

## Execution record

- Existing implementation and historical evidence were inspected before editing. The current SaaS sample was still a single accounting package; this conflicts with the subsequently approved package contract and is corrected explicitly.
- The initial scope remains templates/checker only. Model Relay migration and external CI are separate, not implied by local success.

## Final verification and rulings

- Task 1 complete: three new real-parser regressions observed RED then GREEN (package ownership, external dependency policy, Go file-public misuse). Full tool suite 31/31 passed.
- Task 2 complete: SaaS Application/Domain split and explicit dependency rules; exported bypass cases fail as intended. Existing transaction tests and new pure quota boundaries passed.
- Task 3 complete: scaffold suite 7/7; fresh independent setup/verify, both Go samples, Vue, root Go tests/vet and Ruff passed. Exact limits are in examples/arch-template/validation.md and validation.json.
- Ruling: allow_external restricts third-party dependencies; Go standard-library metadata comes from go/build. Explicit forbid_imports remains authoritative for standard-library HTTP/SQL. Cost if wrong: unexpected standard-library classification; fixed-runtime tests and the build gate remain required.
- Final review: one independent read-only reviewer found two Important issues. Both were reproduced RED and fixed GREEN: component-order allowlist loss and SQL subpackage bypass. The complete scaffold suite and final exported verify.py passed afterward.
- Review coverage note: the proposed pure quota boundary tests were already an explicit Task 2 acceptance requirement; completed them rather than deferring an unfinished task as optional polish. No remaining review findings.
- No domain DSL, external model credentials, Model Relay changes, running environment changes, source main merge or platform PR.
