# Codex Sol Handoff — BBGO-PAY-001 Production Gap Tests 01

You are **Principal Dev — Codex Sol**, using `gpt-5.6-sol` at High. This is the complete
durable prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: the commit containing this handoff

Read completely before acting: `AGENTS.md`, `TESTING.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md`, and the complete current
payment test and production source.

The seven uncommitted production paths are preserved, rejected source. Do not edit them.
Your sole task is to add exactly three non-vacuous regression tests by editing only:

- `modern/payment/signature_test.go`
- `modern/payment/canonical_test.go`
- `modern/payment/service_test.go`

Use `apply_patch`. Do not edit any other byte or path.

## Exact tests

1. Add `TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal`. Build a valid live request,
   replace `Memo` with a Go string containing at least one invalid UTF-8 byte, first prove
   `utf8.ValidString` is false, call `SignRequest`, and require stable `SCHEMA`. The test
   must fail against the current source because the current producer signs JSON's U+FFFD
   replacement.
2. Add `TestCanonicalJSONNestingBoundaries`. Construct canonical arrays containing one
   safe string leaf at 31, 32, and 33 nested containers. Require depths 31 and 32 to
   succeed unchanged and depth 33 to fail with stable `SCHEMA`. Prove every generated
   candidate has the requested distinct depth/byte length. The public maximum is exactly
   32 nested containers.
3. Add `TestStatusNonceMustDifferFromLinkedRequest`. Use two real in-process payment
   nodes and the actual stream handler. Send and persist a valid request, build a valid
   payee-signed cancellation whose event nonce exactly equals that request's nonce, send
   it with `SendSigned` to prevent local convenience linkage from substituting for the
   receiver path, require stable `REPLAY`, prove its event ID is absent, and prove the
   payer has zero stored status records.

Do not weaken or replace an existing test. Do not change the frozen fixture, production,
protocol test, module/lock input, docs, evidence, workflow, policy, or other path. Do not
execute Go, tests, builds, formatters, fuzzers, race, vet, scanners, Git, GitHub, network
commands, daemons, public peers, wallets, rates, transactions, hardware, devices,
release work, binaries, or SBOM generation. Do not use root, `sudo`, `/tmp`, deletion,
cleanup, `rm`, globs, or unresolved destructive targets.

Stop after the three tests are authored. Report their exact names, why each is
non-vacuous and red against the current source, every changed line count/SHA-256, and
confirmation that production and all unlisted paths were untouched. Codex XHigh reviews
the test correction before Luna executes the single expected-red command.
