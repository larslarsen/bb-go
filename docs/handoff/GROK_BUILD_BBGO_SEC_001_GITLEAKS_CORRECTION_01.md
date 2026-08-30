# Grok Build Handoff — BBGO-SEC-001 Gitleaks Baseline Correction 1

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: `9cac1a7c`

The worktree intentionally contains uncommitted BBGO-SEC-001 developer source. Preserve
all existing changes. Read completely before editing:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`
5. `docs/handoff/GROK_BUILD_BBGO_SEC_001_GITLEAKS_01.md`
6. `docs/security/BBGO-SEC-001-EVIDENCE.md`
7. `scripts/gitleaks_baseline_test.py`
8. `scripts/gitleaks_baseline.py`

Codex Luna verified all six delivered hashes, then ran the complete baseline suite. It
stopped at 25 tests / 1 failure:

`ContentMutationRejectionTest.test_non_redacted_match_is_rejected`

The mutation contains `UNREDACTED_MATCH`. The validator currently accepts it because it
only checks whether the string contains the substring `REDACTED`; that substring also
occurs inside `UNREDACTED`. This is a real fail-closed defect. No later command ran.

Authorized paths only:

- `scripts/gitleaks_baseline_test.py`
- `scripts/gitleaks_baseline.py`

Preserve the existing failing test. Author any additional boundary tests first, then make
the smallest validator correction. A valid match must contain `REDACTED` as a complete
redaction marker, not merely as a substring of a larger ASCII word/identifier. Prove
rejection of prefix and suffix identifier forms such as `UNREDACTED_MATCH`,
`PREFIX_REDACTED`, and `REDACTED_SUFFIX`, while continuing to accept every exact reviewed
baseline entry. Error messages must not echo the mutated match. Preserve the exact
baseline bytes/hash, 25 identities, owner, expiry, workflow policy, and every other
invariant.

Do not run tests, validators, scanners, formatters, builds, Git, or GitHub operations.
Do not edit the baseline JSON, workflows, policies, Go/dependencies, governance, evidence,
or any other path. Do not install, use `/tmp`, clean, delete, or inspect unredacted values.
Final read-only hashes and line counts of the two authorized paths are allowed.

When finished, report the test-source-first order, precise predicate, added rejection
boundaries, final hashes/line counts, ambiguities, and confirmation of no out-of-scope
action. Codex Luna owns execution and falsification.
