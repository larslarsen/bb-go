# Codex Sol Handoff — BBGO-PAY-001 Fuzz-Hang Audit 01

You are **Principal Dev — Codex Sol** using `gpt-5.6-sol` at High. This is a static audit
only. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/testing/BBGO-PAY-001-GREEN-RECOVERY-01.md`, and
`docs/testing/BBGO-PAY-001-FUZZ-HANG-01.md`.

Inspect only the already-present source in:

- `modern/payment/fuzz_test.go`, especially `FuzzDecodeWireEnvelope`;
- `modern/payment/frame.go`;
- the strict JSON parser/canonical helpers in `modern/payment/canonical.go`; and
- the relevant Go module/toolchain settings already visible in `modern/go.mod`.

Determine, from source only:

1. whether any accepted/mutated input can cause nontermination, effectively unbounded
   work, runaway allocation, blocking I/O, recursive-depth escape, or fuzz minimization;
2. whether the round-trip property can fail for a decoder-accepted envelope and thereby
   trigger a long minimization phase;
3. whether using 12 fuzz workers can expose shared state, resource, or initialization
   behavior unique to this target; and
4. the smallest safe next diagnostic command(s), with a hard wall-clock watchdog and
   constrained worker count, that distinguish source hang, fuzz minimization, Go tool
   process behavior, and executor/tool transport failure without rerunning completed
   target 1.

If a test-source or production defect is statically proven, specify the exact regression
test and minimal correction, but do not edit it. If not proven, say so and prescribe a
bounded diagnostic sequence with explicit expected outcomes and stop rules.

Do not run any command, test, formatter, Git, process inspection, scanner, network action,
binary, or SBOM tool. Do not edit any file. Report reasoning and recommendation only for
Codex XHigh review.
