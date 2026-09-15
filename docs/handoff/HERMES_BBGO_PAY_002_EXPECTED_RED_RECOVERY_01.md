# BBGO-PAY-002 — Hermes expected-red recovery 01

Actor: Hermes, free Nous Portal model, manually relayed by owner.
Reviewer: Codex. Status: COMPLETE — recovery accepted in
[expected-red review 02](../testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-02.md).
Next authority: [Grok daemon wiring 01](GROK_BUILD_BBGO_PAY_002_DAEMON_WIRING_01.md).
The recovery commands below are historical; no further execution is authorized here.

Read AGENTS.md, TESTING.md, CURRENT_TASK.md, tickets/BBGO-PAY-002.md,
[source review 02](../testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md), and
[expected-red review 01](../testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-01.md).
This handoff supersedes the first execution handoff. Do not repeat its failing
toolchain-selection command or overwrite its report/logs.

## Scope and preflight

Require HEAD `801f5d55d80fe02c6eb512ff35f8c09acfd679af`, the 733-line test hash
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`,
and all nine original frozen-input hashes. Preserve unrelated changes and the owner
binary. Read-only status/diff, hashes, line counts, tool lookup/version, filesystem,
log, and owned-process inspection are authorized.

Writable paths only:

- `docs/testing/BBGO-PAY-002-EXPECTED-RED-RECOVERY-01.md`;
- the active PAY-002 block in `docs/handoff/CURRENT_TASK.md`;
- `modern/dist/pay002-red-recovery-01/` for new logs and owned temporary state;
- the existing `modern/dist/pay002-red-01/cache/` for reused Go build cache.

No source, formatting, dependency, original evidence/log, ticket, Git, or other writes.
Reverify disk backing and non-symlink ownership before creating the new `tmp` and
`logs` directories. Preserve old logs, including the toolchain failure.

From `modern`, resolve only this existing cached executable:

```text
BBGO_PAY002_GO="$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go"
sha256sum "$BBGO_PAY002_GO"
timeout --signal=TERM --kill-after=5s 15s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off "$BBGO_PAY002_GO" version
```

Require bin/go SHA-256
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`
and output `go version go1.27.0 linux/amd64`, exit 0. Record the cache-relative
executable identity. A missing executable, hash/version mismatch, or timeout means
stop and report; do not try a different Go binary, enable auto selection, or download.

## Two authorized test commands

Run serially, once each, from `modern` with this verified variable. Capture the literal
command, start/end wall time, full stdout/stderr, and the actual shell exit code in
separate new files. Do not pipe in a way that records tee's status instead of the test
command's status. Keep the external process-group watchdog for both commands.

```text
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-red-recovery-01/tmp" TMPDIR="$PWD/dist/pay002-red-recovery-01/tmp" "$BBGO_PAY002_GO" test ./cmd/bitbookd -run '^TestParseDaemonIdentity$' -count=1 -v -timeout=30s
timeout --signal=TERM --kill-after=10s 600s env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/dist/pay002-red-01/cache" GOTMPDIR="$PWD/dist/pay002-red-recovery-01/tmp" TMPDIR="$PWD/dist/pay002-red-recovery-01/tmp" "$BBGO_PAY002_GO" test ./cmd/bitbookd -run '^TestPaymentDaemon(ReceivesAndPersistsRequests|RejectsWrongPayer)$' -count=1 -v -timeout=180s
```

Require parser exit 0 and eight passing table cases in the retained parser log before
running the second command. Require daemon exit 1 with both named tests failing only
at payment negotiation: `protocols not supported: [/bitbook/payment/1.0.0]`.
Any other failure, unexpected pass, missing log, cleanup problem, watchdog exit, or
environment/bind issue requires stopping and reporting. Do not retry or work around
the failure. Tests run solely on loopback with existing cached dependencies; no public
peers, network downloads, broader suites, race, fuzz, scanners, or standalone binaries.

## Record and stop

The new evidence must acknowledge the earlier toolchain failure and missing parser
success log. Link review 01 and leave the original evidence unchanged. Report the
actual free model identifier, HEAD, initial/final status, all source hashes/line
counts, filesystem placement, verified toolchain hash/version, exact command environment,
shell exit codes, whole-command elapsed times, complete raw test outputs, test counts,
log hashes, and owned-child cleanup observation with any visibility limits. Do not
claim unseen process state or reconstruct missing output. Keep artifact references
repository-relative and do not embed machine-specific home paths or private material.

If both gates match, update the active CURRENT_TASK block to
`EXPECTED RED RECOVERED — REVIEWER ACCEPTANCE REQUIRED` and link the new record.
Otherwise use `RECOVERY STOPPED — REVIEWER ACTION REQUIRED`, with the actual reason.
No production work or Git mutation is authorized. Return the evidence to the owner;
Codex decides acceptance and the next source task.
