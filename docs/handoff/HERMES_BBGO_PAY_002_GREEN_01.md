# BBGO-PAY-002 phase A — Hermes integrated green 01

Actor: Hermes, free Nous Portal model, manually relayed by owner.
Reviewer: Codex. Status: COMPLETE — runtime evidence accepted in
[green review 01](../testing/BBGO-PAY-002-GREEN-REVIEW-01.md).
Next authority: [publication 01](HERMES_BBGO_PAY_002_PUBLISH_01.md).
The command authority below is historical; do not rerun these completed gates.

Read AGENTS.md, TESTING.md, CURRENT_TASK.md, tickets/BBGO-PAY-002.md,
[production review 01](../testing/BBGO-PAY-002-PRODUCTION-SOURCE-REVIEW-01.md),
[test review 02](../testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md), and
[accepted expected red](../testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-02.md).

## Baseline and scope

Require HEAD `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Accepted main.go: 225 lines, SHA-256
`6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b`.
Frozen payment_test.go: 733 lines, SHA-256
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
Require all other eight inputs in the original
[inventory](GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_01.md) to match. Preserve every prior
log/report, unrelated dirty governance file, and owner-owned modern/bitbookd.

The reviewed drop is already present in the worktree. Integrate it by verifying its
exact identity; do not recopy, reformat, or modify source except the falsification
specified below. Hermes may read status/diff, files, hashes, counts, tool versions,
filesystem and owned-process metadata. Stop on any unexplained baseline difference.

Writable paths only:

- `modern/cmd/bitbookd/main.go`: temporary exact falsification and exact restoration;
- `docs/testing/BBGO-PAY-002-GREEN-01.md`: consolidated evidence;
- the active PAY-002 block of `docs/handoff/CURRENT_TASK.md`: evidence state/pointer;
- `modern/dist/pay002-green-01/`: owned logs, metadata, backup, and test temp state;
- existing `modern/dist/pay002-red-01/cache/`: reusable Go build cache.

No test authoring/editing, dependency/lock changes, production repairs, Git mutations,
commits, pushes, or edits to previous reports. Integration does not confer acceptance.

## Toolchain and runner

From `modern`, use the same verified cached executable as accepted expected red:

```text
BBGO_PAY002_GO="$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go"
sha256sum "$BBGO_PAY002_GO"
timeout --signal=TERM --kill-after=5s 15s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off "$BBGO_PAY002_GO" version
```

Require hash `1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`
and version `go version go1.27.0 linux/amd64`, exit 0. No auto selection or downloads.
Recheck filesystem type and ordinary owned/non-symlink directory components before
creating modern/dist/pay002-green-01/tmp and logs. Use disk-backed storage and the
existing cache; leave cleanup of runner artifacts to the owner/OS. No deletion.

Keep command stdout/stderr, literal command/environment, source hash at execution,
start/end wall time, elapsed whole-command time, and actual shell exit code in separate
named logs/metadata per stage. Do not lose the command exit status through a pipeline.
Outer timeouts must own the entire child process group; do not use --foreground.

## Stage 1 — integrated focused green

Run once on the accepted source:

```text
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-green-01/tmp" TMPDIR="$PWD/dist/pay002-green-01/tmp" "$BBGO_PAY002_GO" test ./cmd/bitbookd -run '^TestPaymentDaemon(ReceivesAndPersistsRequests|RejectsWrongPayer)$' -count=1 -v -timeout=180s
```

Require exit 0 and both behavior tests passing before falsification.

## Stage 2 — falsify registration, then restore regardless of result

Save a byte-exact backup of accepted main.go under the owned runner directory and
verify its accepted hash. Install guaranteed restoration with a finally/trap mechanism
before mutation. Temporarily remove only the added payment import and this exact
five-line block:

```go
paymentService, err := payment.NewService(node.Node)
if err != nil {
    return err
}
defer paymentService.Close()
```

Preserve all other bytes, including surrounding whitespace. The resulting main.go
must equal the original 219-line baseline SHA-256
`9c7aba19576d162b322dce3dddb61990f8d2a5a65f12b681100bf6e23f8a12ab`.
On mismatch, restore and report without executing. Do not edit the frozen tests.

Run the exact Stage 1 command once against this falsified source, with separate logs.
Require exit 1 with BOTH named tests failing only on unsupported
`/bitbook/payment/1.0.0`, after ordinary daemon readiness. An unused-import/compile
failure, timeout, generic delivery failure, cleanup error, or unexpected pass is not
successful falsification. Restore the saved accepted main.go immediately even if
execution fails or times out; verify its exact accepted hash before any further step.
If restoration cannot be confirmed, stop and report it as the primary blocking issue.

## Stage 3 — restored focused green

After successful falsification and confirmed restoration, run the exact Stage 1
command once more, with its own logs. Require exit 0 and both tests passing.
Recheck all frozen hashes, including go.mod/go.sum and the test file.

## Stage 4 — broader race acceptance

Only after Stages 1–3 meet their expected outcomes, run once:

```text
timeout --signal=TERM --kill-after=10s 900s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-green-01/tmp" TMPDIR="$PWD/dist/pay002-green-01/tmp" "$BBGO_PAY002_GO" test -race -json ./cmd/bitbookd ./payment ./api ./direct ./network -count=1 -timeout=300s
```

Require exit 0, all five packages passing, and no race, panic, timeout, or cleanup
diagnostic. Record per-package test/subtest counts from the retained JSON; distinguish
top-level tests from subtests and helpers. Do not claim a native fuzz campaign from
ordinary fuzz seed execution. This is the ticket's authorized broader suite.

Stop at the first unexpected outcome and record it; do not repeat commands, fix source,
weaken tests, widen suite scope, switch toolchains, or work around bind/environment
failures. The only mandatory continuation on failure is restoring falsified source.
No scanner/download, standalone daemon, owner binary, wallet/coin, public-peer,
cross-repository, or release-artifact operation is authorized in this handoff.

## Evidence and stop

Create BBGO-PAY-002-GREEN-01.md even if a stage stops. Include actual free model,
HEAD, initial/final status, source/test hashes and line counts, all eight other frozen
input identities, formatter/source invariance, toolchain version/hash, filesystem
placement, exact command results and whole-command times, stage log hashes, complete
focused/falsified/restored diagnostics, race package outcomes/counts, and limitations
of owned-child cleanup observation. Record exact falsified and restored source hashes.
Summarize the six-line production change and link accepted source/red reviews.
Use repository-relative artifact references and omit local home paths/private material.

If every gate passes, set only the active CURRENT_TASK block to
`INTEGRATED GREEN CAPTURED — REVIEWER ACCEPTANCE REQUIRED`, link evidence, and state
no further source or Git work is authorized. Otherwise use
`GREEN ATTEMPT STOPPED — REVIEWER ACTION REQUIRED` with the exact failing stage and
restoration status. Leave historical PAY-001 records and ticket disposition alone.
Return to the owner; Codex accepts or rejects evidence and separately authorizes any
publication. No commit or push in this handoff.
