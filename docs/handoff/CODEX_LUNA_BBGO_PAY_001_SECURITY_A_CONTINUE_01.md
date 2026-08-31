# Codex Luna Handoff — BBGO-PAY-001 Security Phase A Continuation 01

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. Ephemeral chat is not
authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Modern module: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Pinned tools: `/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-GREEN-RECOVERY-01.md`,
`docs/testing/BBGO-PAY-001-FUZZ-COMPLETION-REVIEW-01.md`,
`docs/testing/BBGO-PAY-001-SECURITY-A-RECOVERY-01.md`, `tickets/BBGO-SEC-001.md`, and
`docs/security/BBGO-SEC-001-EVIDENCE.md`.

## Purpose and non-duplication boundary

Security policy unit suites and Actionlint already passed and must not be rerun. The
first source Govulncheck attempts produced no verdict; Gosec did not run. Perform only
the bounded official-database probe, policy-adjudicated source Govulncheck, conditional
Gosec, and final read-only state checks below.

## Preflight

From the repository root, require `HEAD` and upstream to equal the governance baseline.
Require exactly these eight dirty paths and no others:

```text
modern/go.mod
modern/network/protocols.go
modern/payment/canonical.go
modern/payment/frame.go
modern/payment/replay.go
modern/payment/service.go
modern/payment/signature.go
modern/payment/types.go
```

Require every hash/line count in green recovery 01, only the accepted direct-dependency
`go.mod` diff, clean `modern/go.sum`, clean `git diff --check`, an empty seven-path
read-only `gofmt -d`, and no active Go, Govulncheck, or Gosec process. Require
`/usr/bin/timeout` and `/usr/bin/curl` to exist. Stop on any mismatch.

## 1. Bounded official-database probe

Run exactly this command from the repository root with network authority limited to the
official Go vulnerability database:

```text
/usr/bin/timeout --signal=TERM --kill-after=5s 30s /usr/bin/curl --connect-timeout 10 --max-time 20 -fsS -o /dev/null -w 'HTTP %{http_code} total=%{time_total}\n' https://vuln.go.dev/index/db.json
```

Require exit 0 and HTTP 200. Stop on any other exit, status, timeout, or transport
result. Do not print the response body and do not try another host.

## 2. Bounded policy-adjudicated source Govulncheck

Only if the probe passes, run this from the repository root in a PTY-backed foreground
session. Use a yield of at most 30 seconds; if the execution API yields a session ID,
poll that same session until completion. Do not start another invocation.

```text
/usr/bin/timeout --signal=TERM --kill-after=10s 300s env PATH=/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829:/usr/bin:/bin GOMAXPROCS=1 GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 python3 scripts/govulncheck_policy.py source
```

Require exit 0. The only accepted reviewed exception is `GO-2024-3218` on
`github.com/libp2p/go-libp2p-kad-dht@v0.42.2`, with its non-reachable notes; it expires
2026-11-29. Exit 124/137, any other nonzero exit, a new/unreviewed vulnerability, or a
policy rejection stops immediately. Do not suppress, repair, add an exception, or rerun.

## 3. Conditional bounded Gosec

Only after a successful Govulncheck policy verdict, run this from `modern/` in the same
PTY-backed foreground/poll pattern:

```text
/usr/bin/timeout --signal=TERM --kill-after=10s 300s env GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 /home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/gosec ./...
```

Require exit 0 and zero issues. A watchdog or scanner failure stops immediately.

## Report and scope boundary

Perform only read-only final status, hash, diff, and process checks. Report the database
probe status/timing, complete Govulncheck policy verdict and exit, complete Gosec metrics
and exit if authorized, plus the final repository/process state.

Do not rerun policy unit tests, Actionlint, tests, fuzz, race, or vet. Do not run Gitleaks,
edit evidence, mutate Git, build binaries, regenerate an SBOM, use `/tmp`, delete or clean
anything, use root or `sudo`, contact public peers or another database, or perform wallet,
rate, transaction, hardware/device, desktop, release, or unrelated work.
