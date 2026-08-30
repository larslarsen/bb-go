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
