# Grok Build Handoff — BBGO-SEC-001 Reviewed Gitleaks Baseline 1

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: `f392cbf7392d6e0c17415cc68e82cd996f59a316`

The worktree intentionally contains uncommitted BBGO-SEC-001 developer source. Preserve
all existing changes. Read completely before editing:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`, especially Reviewed Gitleaks Baseline 1
5. `docs/security/BBGO-SEC-001-EVIDENCE.md`
6. `scripts/security_policy.py`, its tests, and `.github/workflows/security.yml`

Authorized redacted input only:

`/home/lars/OpenBazaar/.security-artifacts/bb-go-sec-001-20260829/gitleaks-redacted.json`

It is already verified: SHA-256
`ac71e27a9f2954f7d148b8dd9d630c587abbb92b2a183f53b267e7739f418e00`, 21,758 bytes,
25 entries, every `Secret` literal `REDACTED`. Do not inspect unredacted source values.

Authorized output paths only:

- `security/gitleaks-baseline.json`
- `scripts/gitleaks_baseline_test.py`
- `scripts/gitleaks_baseline.py`
- `scripts/security_policy_test.py`
- `scripts/security_policy.py`
- `.github/workflows/security.yml`

Author all changed test source first. Copy the verified redacted report exactly as the
baseline, then implement the ticket's fail-closed stdlib validator and workflow policy.
The validator must reject added/removed/changed identities, non-redacted content,
duplicates, any `modern/` path, wrong hash/count, or expiry. Tests must independently
mutate each condition and prove rejection. Workflow tests must require validator
adjacency, exact baseline/redaction flags and paths/triggers, and reject broad allowlists
or alternate baselines. Preserve every existing pin, scanner, exception, safety, and
ordering invariant.

Do not run tests, Gitleaks, Actionlint, validators, formatters, builds, or any command
except final read-only hashes/counts. Do not read unredacted reports/matches, install,
edit Go/dependencies/go.sum/SBOM workflow/go workflow/governance/evidence, use Git,
commit, push, or change GitHub state.

When finished, report final paths/hashes/line counts, test-source-first order, baseline
safe metadata only, each mutation/rejection, exact workflow commands/order, expiry and
removal enforcement, ambiguities, and confirmation of no out-of-scope action.

## Delivered source report — 2026-08-29

Grok Build reported the required test-source-first order and stopped without executing
tests, validators, scanners, formatters, builds, Git, or GitHub operations. It authored
only the six authorized paths. Reviewer read-only verification confirmed that
`security/gitleaks-baseline.json` is byte-identical to the independently generated
redacted artifact: 21,758 bytes and SHA-256
`ac71e27a9f2954f7d148b8dd9d630c587abbb92b2a183f53b267e7739f418e00`.

Delivered paths, line counts, and hashes:

- `security/gitleaks-baseline.json`: 527 lines,
  `ac71e27a9f2954f7d148b8dd9d630c587abbb92b2a183f53b267e7739f418e00`
- `scripts/gitleaks_baseline_test.py`: 483 lines,
  `30de5346929619d37224db2d85108f8f5b743a92436344dfb6a25b05f01efa98`
- `scripts/gitleaks_baseline.py`: 338 lines,
  `35934472be90c09ee440273d7e91974928904d5a3bc19d99b81ffdd866bf8932`
- `scripts/security_policy_test.py`: 858 lines,
  `1532c9909c3e22be7e9a6eb12d6dbf78aabf76d6b9f4dadd72f7c37668b01a0e`
- `scripts/security_policy.py`: 1,253 lines,
  `4bd1efad2e6e28909829606965b76b8f59bfffeaa51c83e01d99bc54b5356e36`
- `.github/workflows/security.yml`: 78 lines,
  `a965014cab9cbf60737c91e85e5f5d3e36d0fd0138f4c4553b9dbfa00fa8fa8f`

The validator freezes the complete redacted bytes plus the 25 exact
`(RuleID, File, Commit, StartLine)` identities, rejects duplicates and `modern/` paths,
and fails on 2026-11-29 or later. The workflow requires the validator immediately before
exactly:

```text
gitleaks git --redact=100 --no-banner --baseline-path security/gitleaks-baseline.json .
```

Reviewer source inspection found no widening or unreviewed suppression. Execution
acceptance remains owned by Codex Luna.

## Codex Luna integration continuation

Verify the delivered hashes before execution. Run the complete 25-test Gitleaks baseline
suite and 51-test workflow-policy suite, targeted mutation selectors for added identity,
non-redacted content, `modern/`, expiry, alternate baseline, missing adjacency, and
missing full redaction, then run the actual validator, Actionlint, and the exact pinned
Gitleaks command above. A new finding, test failure, validator failure, or changed source
hash stops integration before later checks and Git.

Use only the existing pinned binary under
`/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829` and the existing explicit
disk-backed cache/temp/artifact directories. Do not reinstall tools, use local `/tmp`,
clean or delete any task path, expose a finding body, or run Git until the entire ticket
acceptance sequence passes. If the exact Gitleaks scan reports zero new findings, resume
the remaining maintained race, explicit disk-backed daemon build and binary
adjudication, CycloneDX generation/validation, and `git diff --check` sequence already
specified by BBGO-SEC-001.
