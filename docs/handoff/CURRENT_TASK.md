# Current Task

Ticket: BBGO-SEC-001

State: CORRECTION AUTHORIZED

Source actor: Sr Dev — Grok Build (Grok 4.6 High)

Integration actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

[BBGO-SEC-002](../../tickets/BBGO-SEC-002.md) is accepted: all tests and remote CI passed,
the obsolete marketplace QA tree is gone, and GitHub reports zero open Dependabot alerts.

[BBGO-SEC-001](../../tickets/BBGO-SEC-001.md) is the only authorized implementation. It
adds pinned maintained-daemon scanning, immutable Action pins, documentation-only CI
filtering, and manual-only SBOM evidence. Its complete prompts are preserved in
[GROK_BUILD_BBGO_SEC_001.md](GROK_BUILD_BBGO_SEC_001.md) and
[CODEX_LUNA_BBGO_SEC_001.md](CODEX_LUNA_BBGO_SEC_001.md). No other implementation is
authorized.

Codex Luna reproduced red, then stopped on a 31-pass/1-error green policy result before
running scanners or Git. The failure is recorded in
[BBGO-SEC-001-EVIDENCE.md](../security/BBGO-SEC-001-EVIDENCE.md). Grok Build may perform
only [Correction Cycle 1](GROK_BUILD_BBGO_SEC_001_CORRECTION_01.md); Codex Luna resumes
integration only after that bounded correction report is preserved.

Correction Cycle 1 is preserved and ready for validation. A resumed Luna attempt was
interrupted before execution when it proposed recursive deletion through an indirect
target. Luna may resume from green only under the explicit no-recursive/unresolved-delete
rules in the ticket and [Codex Luna handoff](CODEX_LUNA_BBGO_SEC_001.md).

A second resume was interrupted before scanning because Luna placed Go scanner binaries
under RAM-backed `/tmp`. The completed binaries were moved intact to the exact ext4-backed
tool directory recorded in the ticket and handoff. Luna may resume only with those fixed
disk-backed tool, cache, temp, and artifact paths; it must not reinstall the four existing
tools or clean the task directories.
