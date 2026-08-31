# Codex Luna Handoff — BBGO-PAY-001 Fuzz Diagnostic 01

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. This is a bounded diagnostic on a
fresh agent turn. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Modern module: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`, `docs/testing/BBGO-PAY-001-GREEN-RECOVERY-01.md`,
`docs/testing/BBGO-PAY-001-FUZZ-HANG-01.md`, and
`docs/testing/BBGO-PAY-001-FUZZ-HANG-AUDIT-REVIEW-01.md`.

## Preflight

From the repository root, require `HEAD` and upstream to equal the governance baseline.
Require exactly the eight dirty paths and all hashes/line counts in green recovery 01.
Require only the recorded direct-dependency `modern/go.mod` diff, clean `modern/go.sum`,
clean `git diff --check`, empty read-only `gofmt -d` over all seven production paths, and
no active `go test`/fuzz process. Stop on any mismatch.

## Exact diagnostic

Run from `modern/`. For every command, use a PTY/pollable foreground exec session. Set
the execution call's initial yield to no more than 30 seconds. If it yields a session ID,
poll only that same session; never duplicate the command. The outer watchdog must make
each process terminate within 35 seconds even if Go's internal timeout fails.

Run command 1:

```text
/usr/bin/timeout --signal=TERM --kill-after=5s 30s env GOMAXPROCS=1 GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test -json ./payment -run '^FuzzDecodeWireEnvelope$' -count=1 -parallel=1 -timeout=15s
```

This ordinary seed-corpus run must exit 0. Stop on test failure, internal timeout/dump,
outer-watchdog exit 124/137, or controlling session failure. Only after success run
command 2:

```text
/usr/bin/timeout --signal=TERM --kill-after=5s 30s env GOMAXPROCS=1 GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test -json ./payment -run '^$' -fuzz '^FuzzDecodeWireEnvelope$' -fuzztime=1x -fuzzminimizetime=1x -parallel=1 -timeout=15s
```

This must exit 0 with a real native fuzz result. If it hangs/fails, do not proceed or use
a fresh cache. Only after success run command 3:

```text
/usr/bin/timeout --signal=TERM --kill-after=5s 30s env GOMAXPROCS=1 GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test -json ./payment -run '^$' -fuzz '^FuzzDecodeWireEnvelope$' -fuzztime=3s -fuzzminimizetime=1x -parallel=1 -timeout=15s
```

This must exit 0 with a nonzero execution count. Preserve complete JSON stdout/stderr,
exit statuses, watchdog behavior, fuzz execution/interesting counts, and elapsed results.
Any reported failing input, panic, timeout, or fuzz artifact stops the turn; do not
minimize again, repair, rerun, delete, or proceed.

After the last attempted command, perform read-only status/hash/process checks and report
the exact result. Do not run target 1, targets 3–6, another target-2 variant, a fresh-cache
comparison, tests, vet, race, scanners, security suites, Git mutation, binaries, or SBOM.
Do not edit any path. Do not use `/tmp`, root, `sudo`, deletion, cleanup, public peers,
external services, or background execution.
