# Grok Build Handoff — BBGO-SEC-001

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Source-content baseline: `5289c564490a54f1adc5be1d451277d2576f7090`

Read completely before editing:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`
5. `.github/workflows/go.yml`

Implement exactly `BBGO-SEC-001`. Author `scripts/security_policy_test.py` first, then
the checker and three workflow files. Preserve the existing Go workflow commands while
pinning its Actions and restricting its triggers exactly as ticketed. Use only Python's
standard library in policy code. Do not change any dependency or other path.

Do not run tests, scanners, builds, validators, or acceptance commands. Do not install
tools. Do not edit governance, tickets, handoffs, or evidence. Do not use Git, commit, or
push.

When finished, stop and report:

- every added/modified path, SHA-256, and line count;
- test-source-first confirmation;
- the invariants independently checked by each policy test;
- the exact workflow triggers, permissions, pins, tool versions, and uploaded paths;
- confirmation that routine jobs do not build/upload binaries and SBOM is manual-only;
- any ambiguity or unimplemented requirement; and
- confirmation that no command, install, Git operation, or out-of-scope edit occurred.

## Delivered Source Report

Pending Grok Build execution.
