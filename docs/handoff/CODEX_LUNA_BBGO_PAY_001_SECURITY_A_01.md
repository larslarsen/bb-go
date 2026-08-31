# Codex Luna Handoff — BBGO-PAY-001 Security Phase A 01

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. Ephemeral chat is not
authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Modern module: `/home/lars/OpenBazaar/bb-go/modern`

Governance baseline: the commit containing this handoff

Pinned tools: `/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`, `docs/testing/BBGO-PAY-001-GREEN-RECOVERY-01.md`,
`docs/testing/BBGO-PAY-001-FUZZ-COMPLETION-REVIEW-01.md`,
`tickets/BBGO-SEC-001.md`, and `docs/security/BBGO-SEC-001-EVIDENCE.md`.

## Preflight

From the repository root, require `HEAD` and upstream to equal the governance baseline.
Require exactly the eight dirty paths and every hash/line count in green recovery 01,
only the accepted direct-dependency `go.mod` diff, clean `go.sum`, clean
`git diff --check`, empty seven-path read-only `gofmt -d`, and no active Go/fuzz process.
Verify with `go version -m` that pinned `govulncheck`, `gosec`, `gitleaks`, and
`actionlint` are the reviewed versions built by Go 1.27.0. Stop on any mismatch.

## Phase A commands

Run serially in the foreground from the repository root unless a working directory is
specified. Preserve complete concise output and exit status.

1. Run all current policy suites:

```text
python3 -m unittest scripts/security_policy_test.py
python3 -m unittest scripts/govulncheck_policy_test.py
python3 -m unittest scripts/gitleaks_baseline_test.py
```

2. Run pinned Actionlint:

```text
/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/actionlint .github/workflows/go.yml .github/workflows/security.yml .github/workflows/sbom.yml
```

3. Run source Govulncheck only through the policy adjudicator. Use the official
vulnerability database and request network authority only if the sandbox blocks that
database. The policy must exit 0 with only reviewed exception `GO-2024-3218` on DHT
v0.42.2 and its non-reachable notes; the exception expires 2026-11-29.

```text
PATH=/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829:/usr/bin:/bin GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 python3 scripts/govulncheck_policy.py source
```

4. From `modern/`, run pinned Gosec and require zero issues:

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 /home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/gosec ./...
```

Any test failure, workflow lint, new/unreviewed vulnerability, policy rejection, or
Gosec issue stops immediately. Never suppress, baseline, repair, or print secret values.

After Gosec, perform only read-only status/hash/diff/process checks and report exact
versions, unittest counts, Actionlint result, Govulncheck adjudication, Gosec metrics,
exit statuses, and final state. Do not run Gitleaks validation/history scan, tests, fuzz,
race, vet, Git mutation, evidence editing, binary builds, binary Govulncheck, or SBOM.
Do not use `/tmp`, root, `sudo`, deletion, cleanup, background work, public peers, or
external application services.
