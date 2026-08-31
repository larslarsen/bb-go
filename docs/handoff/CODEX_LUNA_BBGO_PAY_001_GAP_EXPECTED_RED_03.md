# Codex Luna Handoff — BBGO-PAY-001 Gap Expected Red 03

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. This is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Working directory for the sole Go command: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md`,
`docs/testing/BBGO-PAY-001-GAP-TEST-SOURCE-REVIEW-01.md`,
`docs/testing/BBGO-PAY-001-TEST-COMPILE-CORRECTION-REVIEW-01.md`, and
`docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-02.md`.

## Preflight

From the repository root, require `HEAD` and upstream to equal the governance baseline.
Require exactly the same eleven dirty paths, line counts, and SHA-256 values specified
by gap-test source review 01 and test compile-correction review 01. Require a clean
`git diff --check` and empty read-only `gofmt -d` over all eleven paths. Stop on any
difference.

## Sole execution authorization

Request an execution sandbox override for exactly this one foreground command, explaining
that it permits only ephemeral loopback sockets for an in-process libp2p test and does
not grant root access. Run it once from `modern/` after approval:

```text
GOTOOLCHAIN=go1.27.0 go test ./payment -run '^(TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal|TestCanonicalJSONNestingBoundaries|TestStatusNonceMustDifferFromLinkedRequest)$' -count=1
```

Do not broaden the command or request a reusable broad prefix. The accepted result must
compile and exit nonzero solely with these three behavioral failures:

- invalid typed UTF-8 is signed instead of returning `SCHEMA`;
- depth 33 canonicalizes instead of returning `SCHEMA`, with depths 31 and 32 passing;
- linked status/request nonce reuse is accepted instead of returning `REPLAY`.

Reject any compile, dependency, public-network, environment, bind, timeout, panic,
fixture, or unrelated failure. Do not rerun for different output.

## Evidence and integration

If and only if the result is acceptable expected red:

1. Create `docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-03.md` with the governance
   baseline, all eleven hashes/line counts, preflight, exact command, exit code, complete
   diagnostics, explanation of all three intended failures, and the fact that the
   override enabled only local loopback execution without root or public services.
2. Update `docs/handoff/CURRENT_TASK.md` to `GAP EXPECTED RED 03 CAPTURED — XHIGH
   ACCEPTANCE REQUIRED`, link the evidence, and keep production correction unauthorized.
3. Stage exactly the four modified test paths, the new evidence, and `CURRENT_TASK.md`
   with literal paths. Commit once as `test(payment): freeze gap regressions` and push
   `master`.
4. Leave the seven production paths dirty and uncommitted at reviewed hashes. Report the
   pushed SHA, exact final non-clean status, command result, and committed paths.

Do not edit tests, production, fixtures, ticket, module/lock, workflow, policy, or any
unrelated path. Do not stage the already-committed governance handoff/reviews. Do not use
root, `sudo`, deletion, cleanup, `rm`, `/tmp`, globs, unresolved targets, public peers,
external services, or background work. Do not run another command beyond exact preflight,
the one authorized test command, and explicit evidence/integration Git operations.
