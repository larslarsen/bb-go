# Codex Luna Handoff — BBGO-PAY-001 Gap Expected Red 02

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. This is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Working directory for the sole Go command: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Integrated frozen-test baseline: `403df23a63f413c11e13085719fc7e767c2f15be`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md`,
`docs/testing/BBGO-PAY-001-GAP-TEST-SOURCE-REVIEW-01.md`,
`docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-01.md`, and
`docs/testing/BBGO-PAY-001-TEST-COMPILE-CORRECTION-REVIEW-01.md`.

## Preflight

Run preflight from the repository root. Require `HEAD` and its upstream to equal the
governance baseline. Require exactly eleven dirty paths: the ten paths and hashes in
gap-test source review 01 plus modified `modern/payment/transport_test.go` at 397 lines
and SHA-256 `65fc2eeb90c967d59f1e00b514a47c48edbf4ee2d366f512157391c43c811362`.
Require a clean `git diff --check` and an empty read-only `gofmt -d` over all eleven
paths. If any preflight differs, stop without running Go, editing, staging, committing,
or pushing.

## Sole execution authorization

Run exactly this command once, serially in the foreground from `modern/`, preserving
complete stdout/stderr and its exit status:

```text
GOTOOLCHAIN=go1.27.0 go test ./payment -run '^(TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal|TestCanonicalJSONNestingBoundaries|TestStatusNonceMustDifferFromLinkedRequest)$' -count=1
```

The command must compile and exit nonzero only because the preserved production source
fails the three new behavioral assertions:

- invalid typed UTF-8 reaches signing instead of returning `SCHEMA`;
- nesting depth 33 canonicalizes instead of returning `SCHEMA`, while depths 31 and 32
  pass; and
- a linked status reusing its request nonce is accepted instead of returning `REPLAY`.

A compile, syntax, import, module/dependency, environment, bind, timeout, panic, fixture,
or unrelated-test failure is not acceptable expected red. Do not rerun merely to obtain
different output. Do not run any other Go/test/build/fuzz, race, vet, scanner, public
network, daemon, wallet, rate, transaction, hardware, device, release-binary, or SBOM
command.

## Evidence and integration

If and only if the result is acceptable expected red:

1. Create `docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-02.md` with the baselines, all
   eleven hashes and line counts, exact preflight results, exact command, exit code,
   complete diagnostics, and why each diagnostic is the intended missing-protection
   failure. Note that attempt 01 never reached test execution and is superseded only as
   execution evidence, not erased.
2. Update `docs/handoff/CURRENT_TASK.md` to state `GAP EXPECTED RED 02 CAPTURED — XHIGH
   ACCEPTANCE REQUIRED`, link the new evidence, and state that production correction
   remains unauthorized.
3. Stage exactly `modern/payment/canonical_test.go`,
   `modern/payment/signature_test.go`, `modern/payment/service_test.go`,
   `modern/payment/transport_test.go`, the new evidence, and
   `docs/handoff/CURRENT_TASK.md`, using explicit literal paths. Commit once with message
   `test(payment): freeze gap regressions` and push `master`.
4. Leave all seven production paths dirty and uncommitted at their reviewed hashes.
   Report the commit SHA, pushed ref, exact non-clean `git status --short`, command
   result, and every committed path. Do not claim a clean worktree or engineering
   acceptance; Codex XHigh owns acceptance.

Do not edit any test source, production source, fixture, ticket, module/lock input,
workflow, policy, or unrelated file. Do not stage or commit this handoff/review (they are
already in the governance baseline). Do not use root, `sudo`, deletion, cleanup, `rm`,
`/tmp`, globs, unresolved targets, public peers, external services, or background
commands. Stop on any unexpected condition.
