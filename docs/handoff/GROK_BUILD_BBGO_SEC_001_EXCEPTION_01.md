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

## Reviewer Correction Cycle 1

The delivered source is not yet accepted. Before execution, reviewer inspection found:

- the SBOM step deletes the variable-resolved `"${binary}"`, contrary to standing
  destructive-action safety; leave it unuploaded for the ephemeral runner instead; and
- the successful adjudicator prints raw SARIF, which is approximately 220,000 lines for
  the real source scan; print only the already validated concise summary, including all
  note IDs/messages and exception metadata.

Authorized correction paths only:

- `scripts/govulncheck_policy_test.py`
- `scripts/govulncheck_policy.py`
- `scripts/security_policy_test.py`
- `scripts/security_policy.py`
- `.github/workflows/sbom.yml`

Author changed tests first. Add workflow-policy coverage that rejects any SBOM command
which deletes a target expressed through an environment/shell variable, substitution,
glob, or symlink-derived value. Add adjudicator coverage proving successful output is a
concise summary and does not echo the raw SARIF document. Preserve every other
fail-closed invariant, workflow order, pin, and behavior. Do not run commands except
read-only final hashes/counts; all original scope, no-test, no-install, no-Git, and stop
rules remain in force. Report a new complete correction table and the exact new tests.

### Correction Cycle 1 Delivered Report

Execution date: 2026-08-29

Grok reported test-source-first correction and no test, scanner, formatter, build,
validator, install, or Git execution. Only read-only hashes/counts were run.

| Path | Lines | SHA-256 |
|---|---:|---|
| `scripts/govulncheck_policy_test.py` | 699 | `8c278f1830245992d86f8a777852588979049cc70f2f4f39a864df7206e8020d` |
| `scripts/security_policy_test.py` | 721 | `19fb7ab17b06556e526d7baa75089b5471a2351c09170658dffcd04ac1cd4d54` |
| `scripts/govulncheck_policy.py` | 356 | `709cb00d44c62ef6e2d394f457407183d6fb90bc98958c80db0261607bc3c77c` |
| `scripts/security_policy.py` | 1,172 | `36f866c0fc19cdbac373afce6edf7b2342b1be9af419ff12fa789b228890f671` |
| `.github/workflows/sbom.yml` | 58 | `7f782b60c33d565120ea0bf9605421da989b01b26e42c68d7f089bbd2864300b` |

The correction adds three successful-CLI tests proving source, exception, and binary
results emit only the concise validated summary; the exception case must retain every
note ID/message plus owner and expiry. Four workflow mutations cover variable,
substitution, glob, and syntactically symlink-derived deletion targets. The committed
SBOM workflow contains no deletion command and leaves its unuploaded binary for runner
disposal. All original SARIF accept/reject fixtures and workflow invariants remain.

Reviewer independently verified these hashes/counts and inspected the corrected main
output path, deletion detector/tests, and SBOM workflow. Execution is still required.
