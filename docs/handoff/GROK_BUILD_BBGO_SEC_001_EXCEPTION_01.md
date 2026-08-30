# Grok Build Handoff — BBGO-SEC-001 Reviewed Govulncheck Exception 1

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: `ae0b5f5b161006e93a3def2a9d61e9f30768ac46`

The worktree intentionally contains uncommitted BBGO-SEC-001 developer source. Preserve
all existing changes. Read completely before editing:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`, especially Reviewed Govulncheck Exception 1
5. `docs/security/BBGO-SEC-001-EVIDENCE.md`
6. `.github/workflows/security.yml` and `.github/workflows/sbom.yml`
7. `scripts/security_policy.py` and `scripts/security_policy_test.py`

Authorized paths only:

- `scripts/govulncheck_policy_test.py`
- `scripts/govulncheck_policy.py`
- `scripts/security_policy_test.py`
- `scripts/security_policy.py`
- `.github/workflows/security.yml`
- `.github/workflows/sbom.yml`

Author all new/changed test source first. Implement the ticket's fail-closed policy for
both source and binary SARIF. Prefer one small standard-library Python entry point that
executes Govulncheck, captures and validates SARIF, prints the full scanner output or a
clear summary without secrets, and returns success only for a clean scan or the exact
unexpired reviewed result. Do not depend on shell `continue-on-error`, `|| true`, an
unvalidated exit code, mutable online exception data, or a generic ignore list.

Update the existing workflow policy checker/tests so they require this adjudicator,
require the focused diversity test immediately before source scanning, cover the new
script paths in triggers, retain immutable Actions and pinned tool versions, and reject
bypass mutations. SBOM binary scanning must use the same adjudicator before artifact
upload. Note-level non-reachable SARIF results remain reported; any error result other
than the exact exception fails.

Do not run tests, scanners, formatters, builds, or validators. Do not install anything.
Do not edit Go source, `go.mod`, `go.sum`, governance, tickets, handoffs, or evidence. Do
not use Git, commit, push, or change GitHub state.

When finished, stop and report every path/hash/line count, test-source-first order, each
accept/reject fixture and mutation, exact workflow invocation/ordering, exception
metadata and expiry enforcement, ambiguities, and confirmation of no commands beyond
read-only hashes/counts and no out-of-scope edits.
