# Grok Build Correction Handoff — BBGO-SEC-001 Cycle 1

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Read completely:

1. `AGENTS.md`
2. `TESTING.md`
3. `tickets/BBGO-SEC-001.md`, especially Correction Cycle 1
4. `docs/security/BBGO-SEC-001-EVIDENCE.md`
5. `scripts/security_policy.py`
6. `scripts/security_policy_test.py`

Codex Luna's green attempt ran 32 tests; 31 passed and the repository-check test errored
because `check_sbom_workflow` reported that the daemon build and SBOM generation were not
in the same step. Reviewer inspection found the defect: the checker compares two wrapper
tuples returned by separate `_find_step_containing` calls using object identity. The
tuples are necessarily different objects even when their contained parsed step is the
same object.

Correct only `scripts/security_policy.py` so the existing valid SBOM workflow passes only
when build and generation genuinely share the same parsed step. Comparing the underlying
step objects at tuple index 2 is the expected minimal correction; use an equivalent only
if it is equally narrow and exact.

Do not change test source or any workflow. Do not refactor unrelated checker code. Do not
run tests, scanners, builds, validators, or installs. Do not edit governance/evidence or
use Git.

Stop after the edit and report the exact changed line, final SHA-256 and line count, why
the original comparison was wrong, why the correction cannot accept different steps,
and confirmation that no other path, execution, install, or Git operation occurred.

## Delivered Correction Report

Completed 2026-08-29.

Grok changed only line 557 of `scripts/security_policy.py`:

```python
if build_step[2] is not sbom_step[2]:
```

The previous code compared the newly created wrapper tuples themselves. The correction
compares the underlying parsed step mappings: identical only when both commands were
found in the same actual step, while commands in distinct steps still raise the existing
`PolicyError`.

Final checker SHA-256:
`4d80708822bf22e1e05abe56bb131db6176c217047001f68895dfdab3dc7058a`; line count: 1,010.

Grok reported no other line/path change and no test, scanner, build, validator, install,
or Git execution. Codex independently inspected the exact changed line and confirmed the
reported hash and line count. Acceptance remains with Codex Luna's existing test oracle
and the remaining ticket gates.
