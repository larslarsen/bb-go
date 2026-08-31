# Codex Luna Handoff — BBGO-PAY-001 Expected Red

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. This is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Working directory for commands: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Production source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`, and
`docs/testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md`.

## Preflight

Run preflight from the repository root. Require `HEAD` and its upstream to equal the
governance baseline. Require the worktree to contain exactly the seven untracked paths
frozen in review 02 and no tracked change. Recompute every line count and SHA-256 in
review 02. Require the copied fixture to be byte-identical to
`../bb-desktop/test/fixtures/wallet-contract/golden-v1.json`. Require a clean read-only
`gofmt -d` over the six Go test files. If any preflight differs, stop without running Go,
editing, staging, committing, or pushing.

## Sole execution authorization

Run these commands serially in the foreground from `modern/`, preserving complete
stdout/stderr and exit status:

```text
GOTOOLCHAIN=go1.27.0 go test ./payment -count=1
GOTOOLCHAIN=go1.27.0 go test ./network -run TestPaymentProtocolCurrent -count=1
```

Both commands must exit nonzero only because the reserved payment production API and
`network.PaymentProtocolCurrent` do not yet exist. Undefined identifiers named by the
accepted test source are the expected test-first red state. A syntax error, malformed
test fixture, import error, module/dependency error, environmental failure, panic, hang,
or failure in existing production is not acceptable red. Do not run either command a
second time merely to obtain different output. Do not run any other Go/test/build/fuzz,
race, vet, scanner, network, daemon, wallet, rate, transaction, hardware, device, release,
binary, or SBOM command.

## Evidence and integration

If and only if both results are acceptable expected red:

1. Create `docs/testing/BBGO-PAY-001-EXPECTED-RED.md` with the baselines, all seven
   hashes/line counts, fixture equality, clean format result, exact commands, exit codes,
   complete diagnostics, and why every diagnostic is missing-production red.
2. Update `docs/handoff/CURRENT_TASK.md` to state `EXPECTED RED CAPTURED — REVIEWER
   ACCEPTANCE REQUIRED`, link the evidence, and state that no production implementation
   is authorized.
3. Stage exactly the seven frozen test paths, the evidence, and `CURRENT_TASK.md` using
   explicit paths. Commit once with message
   `test(payment): reserve signed payment contract` and push the current branch.
4. Report the commit SHA, pushed ref, final clean status, exact command results, and all
   changed paths. Do not claim engineering acceptance; Codex XHigh owns it.

Do not edit any test source, fixture, protocol test, ticket, production, module/lock
input, workflow, policy, or unrelated file. Do not use root, `sudo`, deletion, cleanup,
`rm`, `/tmp`, globs, unresolved targets, public peers, external services, or background
commands. Stop on any unexpected condition.
