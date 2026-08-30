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

Policy, falsification, and Actionlint subsequently passed. Govulncheck stopped before
analysis because its executable was built with Go 1.26 and could not load the Go 1.27
module. Luna may perform only the exact Local Toolchain Correction 1 in the ticket,
verify all embedded tool build versions, and resume from Govulncheck.

All five tools now report Go 1.27.0 and exact pinned versions. The next Govulncheck attempt
was blocked before analysis by sandbox DNS/network policy. Luna may resume only the exact
source and later binary Govulncheck commands with approved network access; all other
commands retain ordinary sandbox authority.

The network-enabled scan then found reachable `GO-2024-3218` in the DHT module and Luna
stopped. Reviewer triage found that the maintained single-DHT construction omits the
upstream IP-diversity mitigation. Sr Dev — Grok Build is now authorized only for the
test-first source correction in `docs/handoff/GROK_BUILD_BBGO_SEC_001_FINDING_01.md`.

The first correction red run was blocked before its assertion by sandbox denial of a
loopback ephemeral-port bind. Luna restored the exact source hash. Localhost Socket
Execution Correction 1 now authorizes only the targeted red/green and maintained race
commands outside that socket restriction, still using the named disk-backed paths.

That correction then passed red/falsification, targeted green, and the full maintained
race suite. Govulncheck still reports only reachable `GO-2024-3218` on exact DHT
`v0.42.2`. Grok Build is now authorized only for the expiring, fail-closed SARIF policy
drop in `docs/handoff/GROK_BUILD_BBGO_SEC_001_EXCEPTION_01.md`.

Reviewer inspection returned that drop for Correction Cycle 1: remove the
variable-target binary deletion, enforce that safety invariant, and avoid emitting the
approximately 220,000-line raw SARIF into CI logs. Grok may edit only the five correction
paths enumerated in the same handoff.

The corrected full policy suites passed 42/42 and 49/49. A redundant targeted
dotted-module selector failed before loading. Luna may run only Targeted Unittest
Selector Correction 1's five file-path selectors, then resume from Actionlint.

Those selectors, Actionlint, and source adjudication passed. Gosec then found G115 in
direct frame sizing and G304 in identity path loading. Grok Build is authorized only for
the test-first correction in `docs/handoff/GROK_BUILD_BBGO_SEC_001_GOSEC_01.md`.

That correction passed red/green and Gosec now reports zero issues. Gitleaks then found
25 inherited-history matches. Redacted triage found only root-tree test/docs/examples and
a PGP public-key block, with nothing under `modern/`. Grok is authorized only for the
exact expiring baseline in `docs/handoff/GROK_BUILD_BBGO_SEC_001_GITLEAKS_01.md`.

Grok delivered that six-path baseline drop test-first and stopped without execution.
Reviewer read-only inspection verified the baseline is byte-identical to the reviewed
redacted artifact and found no widened allowlist. Codex Luna now owns the exact validation
and remaining ticket acceptance sequence recorded in the same handoff. It must use only
the fixed disk-backed tool/cache/temp/artifact paths, perform no cleanup or deletion, and
stop before Git on any changed hash, failure, or new finding.

Luna verified all six hashes, then the 25-test baseline suite stopped with one failure:
the match-redaction predicate accepted `UNREDACTED_MATCH` because `REDACTED` appeared only
as a substring. No later command ran. Grok is authorized only for the two-path bounded
fix in `docs/handoff/GROK_BUILD_BBGO_SEC_001_GITLEAKS_CORRECTION_01.md`; Luna resumes the
same gate only after that source report is preserved.

Grok delivered that correction test-first: the existing failing case plus new prefix and
suffix identifier cases are rejected by an explicit complete-marker predicate. Reviewer
inspection accepted the two-path source report and hashes. Luna may now verify the two
new hashes, rerun the 27-test suite and targeted boundaries, then resume the exact prior
continuation sequence.
