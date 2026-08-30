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
