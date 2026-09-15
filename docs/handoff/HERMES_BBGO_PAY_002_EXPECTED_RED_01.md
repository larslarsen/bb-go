# BBGO-PAY-002 phase A — Hermes focused expected red 01

Actor: Hermes, using a free Nous Portal model, manually relayed by owner.
Reviewer: Codex. Status: CLOSED — submitted evidence not accepted in
[review 01](../testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-01.md).
Superseded by [recovery 01](HERMES_BBGO_PAY_002_EXPECTED_RED_RECOVERY_01.md).
The original command authority below is historical and must not be rerun.

Read AGENTS.md, TESTING.md, docs/engineering/DEVELOPMENT_ROLES.md,
tickets/BBGO-PAY-002.md, CURRENT_TASK.md, and
[source review 02](../testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md).

## Baseline and permitted paths

Require HEAD `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Incoming `modern/cmd/bitbookd/payment_test.go`: 733 lines, SHA-256
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
Verify all nine frozen inputs against the original
[inventory](GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_01.md).
Read-only Git status/diff, file reads, hashes, line counts, filesystem inspection,
tool lookup, and owned-process inspection are authorized preflight/report operations.

Preserve the existing dirty governance files, PAY-002 ticket/handoffs/reviews,
untracked test drop, and owner-owned `modern/bitbookd`. Do not require a clean worktree
or stage unrelated changes. Stop on baseline/input mismatch or unexpected source edits.

Only these writes are authorized:

- `modern/cmd/bitbookd/payment_test.go`: mechanical gofmt only, no semantic edits;
- `docs/testing/BBGO-PAY-002-EXPECTED-RED-01.md`: execution evidence;
- the active PAY-002 block of `docs/handoff/CURRENT_TASK.md`: result pointer and state;
- `modern/dist/pay002-red-01/`: owned cache, temporary test state, and raw logs.

## Filesystem and execution setup

The reviewer observed ext4 for modern with `findmnt -T modern -n -o FSTYPE,TARGET`.
Recheck before allocating artifacts. Use repository-relative
`modern/dist/pay002-red-01/cache`, `modern/dist/pay002-red-01/tmp`, and
`modern/dist/pay002-red-01/logs`; require ordinary owned directories with no symlink
components. Do not use RAM-backed temporary storage, delete state, or change Git ignores.
The existing `dist` ignore rule covers runner artifacts. Create these directories only
after verifying the parent filesystem. Record the relative paths and filesystem type.

From `modern`, format only the reviewed drop:

```text
gofmt -w cmd/bitbookd/payment_test.go
gofmt -d cmd/bitbookd/payment_test.go
```

Require the second command to emit no diff. Record incoming/formatted SHA-256 and line
counts, and preserve the formatter-only diff in runner logs. Do not manually repair
syntax or tests. All nine other input hashes must remain unchanged.

Run the following two commands serially, once each, from `modern`, capturing complete
stdout/stderr, exit status, and duration in the owned logs directory. The outer timeout
owns the command process group, including child test processes; do not use --foreground.

```text
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=go1.27.0 GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-red-01/tmp" TMPDIR="$PWD/dist/pay002-red-01/tmp" go test ./cmd/bitbookd -run '^TestParseDaemonIdentity$' -count=1 -v -timeout=30s
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=go1.27.0 GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-red-01/tmp" TMPDIR="$PWD/dist/pay002-red-01/tmp" go test ./cmd/bitbookd -run '^TestPaymentDaemon(ReceivesAndPersistsRequests|RejectsWrongPayer)$' -count=1 -v -timeout=180s
```

Require parser exit 0 with all eight cases passing before running the daemon command.
The daemon command must exit 1 with BOTH named tests failing at payment protocol
negotiation because the unchanged daemon has no payment service registration. Ordinary
startup and direct-protocol readiness must have succeeded, with normal child cleanup.
Any compilation/dependency/fixture error, readiness timeout, loopback-bind failure,
panic, watchdog exit, cleanup error, unexpected pass, or different failure is NOT
acceptable expected red. Stop and record it; do not broaden commands or retry to obtain
different output. A denied loopback bind may be reported for a separate execution
environment authorization. Tests require no public peers or downloads.

Do not launch the owner binary, run a standalone daemon, edit production, run module
tidy/downloads/scanners/full suites/race/fuzz/falsification, or change the selected model
to a paid model. Do not author tests, make Git mutations, commit, or push.

## Evidence and stop

Create the evidence record whether execution succeeds or stops unexpectedly. Include
the model identifier actually used, HEAD, initial/final status, file hashes/line counts,
format-only changes, filesystem placement, exact commands/environment, durations,
exit codes, test/subtest counts, complete relevant diagnostics, and owned-child cleanup
status. Keep private request/signature material and machine-specific absolute paths
out of committed evidence; record artifact paths relative to the repository.

On acceptable results, update only the active CURRENT_TASK block to
`EXPECTED RED CAPTURED — REVIEWER ACCEPTANCE REQUIRED`, link the evidence, and state
that no production implementation is authorized. Otherwise record
`EXPECTED RED ATTEMPT STOPPED — REVIEWER ACTION REQUIRED` and the concrete reason.
Do not change ticket acceptance or historical PAY-001 records. Report to the owner and
stop; Codex alone accepts the evidence and issues the next source handoff.
