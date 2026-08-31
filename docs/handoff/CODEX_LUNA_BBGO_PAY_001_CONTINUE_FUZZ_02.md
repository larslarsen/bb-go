# Codex Luna Handoff — BBGO-PAY-001 Continue Fuzz 02

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. Ephemeral chat is not
authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Modern module: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`, `docs/testing/BBGO-PAY-001-GREEN-RECOVERY-01.md`, and
`docs/testing/BBGO-PAY-001-FUZZ-DIAGNOSTIC-REVIEW-01.md`.

## Preflight

From the repository root, require `HEAD` and upstream to equal the governance baseline.
Require exactly the eight recovered dirty paths and every hash/line count in green
recovery 01, only the accepted direct-dependency `go.mod` diff, clean `go.sum`, clean
`git diff --check`, empty seven-path read-only `gofmt -d`, and no active Go/fuzz process.

## Exact remaining fuzz gates

From `modern/`, run each command serially through a PTY/pollable foreground exec. Use an
initial yield no longer than 30 seconds. If a command yields a session ID, poll only that
same session. Never duplicate it. Stop after any nonzero exit, watchdog action, panic,
timeout/dump, failing input, fuzz artifact, or zero execution count.

```text
/usr/bin/timeout --signal=TERM --kill-after=5s 30s env GOMAXPROCS=1 GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test -json ./payment -run '^$' -fuzz '^FuzzParseFrame$' -fuzztime=3s -fuzzminimizetime=1x -parallel=1 -timeout=15s
/usr/bin/timeout --signal=TERM --kill-after=5s 30s env GOMAXPROCS=1 GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test -json ./payment -run '^$' -fuzz '^FuzzJCSDeterminism$' -fuzztime=3s -fuzzminimizetime=1x -parallel=1 -timeout=15s
/usr/bin/timeout --signal=TERM --kill-after=5s 30s env GOMAXPROCS=1 GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test -json ./payment -run '^$' -fuzz '^FuzzSignatureMutation$' -fuzztime=3s -fuzzminimizetime=1x -parallel=1 -timeout=15s
/usr/bin/timeout --signal=TERM --kill-after=5s 30s env GOMAXPROCS=1 GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test -json ./payment -run '^$' -fuzz '^FuzzReplayKeyCollision$' -fuzztime=3s -fuzzminimizetime=1x -parallel=1 -timeout=15s
```

Each must exit 0 with a nonzero execution count. Preserve complete JSON output, exact
counts/interesting totals, package/wall elapsed time, and watchdog state. After the last
attempt, perform only read-only process/status/hash/diff checks and report.

Do not rerun targets 1 or 2, any earlier green gate, or a completed command. Do not edit
evidence/source/module/tests, run security/scanners, mutate Git, use another cache, build
binaries, or generate an SBOM. Do not use `/tmp`, root, `sudo`, deletion, cleanup,
background execution, public peers, or external services.
