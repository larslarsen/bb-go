# Grok Build Handoff — BBGO-PAY-001 Test Source

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt; ephemeral chat is not authoritative.

Do not start before 2026-08-30 19:53 PDT.

Repository: `/home/lars/OpenBazaar/bb-go`

Baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`,
`docs/engineering/PAYMENT_ROADMAP_ROUTING.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`, `../bb-desktop/docs/architecture/BBD-WAL-001-REVIEW.md`
§§11 and 14, and the accepted oracle fixture named in the ticket. Do not read the Node
canonical implementation; this must remain an independent Go oracle.

Your sole task is the test-source phase in `tickets/BBGO-PAY-001.md`. Copy the named
fixture byte-for-byte, author all required tests and fuzz seeds before production source,
and stop. Do not invent or widen schemas, transport authority, status policy, persistence
fields, API routes, or capability.

Use `apply_patch`. You may use read-only inspection plus `wc -l` and `sha256sum` over the
seven authorized paths and the named source fixture. Do not execute Go, tests, builds,
formatters, fuzzers, scanners, Git, GitHub, network commands, daemons, public peers,
wallets, rate providers, transactions, hardware, or devices. Do not use root, `sudo`,
`/tmp`, deletion, cleanup, `rm`, globs, or variables/substitutions as destructive targets.

Stop after authoring and report:

- each changed path, line count, and SHA-256;
- exact copied-fixture equality and hash;
- every test/fuzz name and total count per file;
- the production Go API reserved by the tests;
- why the two-node, signature, persistence, and replay cases are non-vacuous;
- the exact expected compilation failures against the baseline;
- confirmation that no command outside the allowed read-only/reporting set ran and no
  unlisted path changed.

Lead Engineer/Reviewer — Codex at XHigh must inspect and accept this source before Codex
Luna executes anything. You have no production, execution, integration, evidence, Git,
commit, or push authority.
