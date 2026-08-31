# Codex Luna Handoff — BBGO-PAY-001 Security Phase B 01

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. Ephemeral chat is not
authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: the commit containing this handoff

Pinned Gitleaks: `/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/gitleaks`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-GREEN-RECOVERY-01.md`,
`docs/testing/BBGO-PAY-001-FUZZ-COMPLETION-REVIEW-01.md`,
`docs/testing/BBGO-PAY-001-SECURITY-A-RECOVERY-01.md`,
`docs/testing/BBGO-PAY-001-SECURITY-A-REVIEW-01.md`, `tickets/BBGO-SEC-001.md`, and
`docs/security/BBGO-SEC-001-EVIDENCE.md`.

## Purpose and non-duplication boundary

All ordinary, falsification, race, fuzz, policy-unit, workflow-lint, Govulncheck, and
Gosec gates are complete. Run only the reviewed Gitleaks baseline validator, the exact
pinned redacted history scan, and final read-only state checks. Do not rerun any earlier
gate.

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
`go.mod` diff, clean `modern/go.sum`, clean `git diff --check`, and no active Gitleaks
process. Verify the pinned binary's embedded module path/version with `go version -m` and
require `github.com/zricethezav/gitleaks/v8@v8.30.1`. Stop on any mismatch.

## Reviewed baseline and exact history scan

Run the validator from the repository root:

```text
python3 scripts/gitleaks_baseline.py
```

Require exit 0 and exactly the reviewed safe metadata: 25 entries, owner Lead
Engineer/Reviewer — Codex, expiry 2026-11-29, and SHA-256
`ac71e27a9f2954f7d148b8dd9d630c587abbb92b2a183f53b267e7739f418e00`.

Immediately afterward, with no intervening command, run this exact scan in a PTY-backed
foreground session. Use a yield of at most 30 seconds; if the execution API yields a
session ID, poll that same session until completion. Do not start another invocation.

```text
/usr/bin/timeout --signal=TERM --kill-after=10s 900s /home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/gitleaks git --redact=100 --no-banner --baseline-path security/gitleaks-baseline.json .
```

Require exit 0 and zero findings outside the reviewed baseline. Preserve only safe
summary metrics such as commit count, scanned size, duration, and exit status. Never
print, record, inspect, or persist a secret or matched value. The known exhaustive-rename
limit warning is informational; any finding, other scanner error, exit 124/137, or agent
execution-channel failure stops immediately without retry, repair, suppression, or
baseline change.

## Report and scope boundary

Perform only read-only final status, hash, diff, and Gitleaks-process checks. Report the
validator safe metadata, scanner summary/exit, any warning, and final state.

Do not rerun tests, fuzz, race, vet, policy suites, Actionlint, Govulncheck, or Gosec. Do
not edit evidence, mutate Git, build binaries, regenerate an SBOM, install, use `/tmp`,
delete or clean anything, use root or `sudo`, access an unredacted report, contact any
network/service/peer, or perform wallet, rate, transaction, hardware/device, desktop,
release, or unrelated work.
