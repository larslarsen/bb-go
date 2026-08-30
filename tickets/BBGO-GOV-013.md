# BBGO-GOV-013 — Correct Targeted Unittest Selectors

Status: ACCEPTED

Reviewer: Lead Engineer/Reviewer — Codex

Source baseline: `6dad5c0060898a8dfa7210d083ebdc3df47c81cb`

## Acceptance

- Record that both complete suites passed 42/42 and 49/49.
- Replace only the non-importable dotted targeted selector with five exact file-path
  `-k` selectors from the repository root.
- Each selector must load and pass exactly one existing rejection test.
- No source/test edit, install, cleanup, deletion, Git, or expanded authority.

## Reviewer Acceptance

Accepted as a bounded test-runner addressing correction.
