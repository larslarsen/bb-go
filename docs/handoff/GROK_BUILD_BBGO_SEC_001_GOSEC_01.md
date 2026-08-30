# Grok Build Handoff — BBGO-SEC-001 Gosec Finding Correction 1

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: `a41ba3a634c75aa205a52ea369fae3fac8cf85b3`

The worktree intentionally contains uncommitted BBGO-SEC-001 developer source. Preserve
all existing changes. Read completely before editing:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`, especially Gosec Finding Correction 1
5. `docs/security/BBGO-SEC-001-EVIDENCE.md`
6. all five authorized source/test paths

Authorized paths only:

- `modern/direct/service_test.go`
- `modern/direct/service.go`
- `modern/network/identity_test.go`
- `modern/network/identity.go`
- `modern/network/open.go`

Author all changed test source first. For G115, preserve the four-byte big-endian frame
wire format and 128 KiB maximum while introducing a checked size conversion whose logic
can be tested without huge allocation. Cover below/at/above the protocol boundary and
above `math.MaxUint32`, and retain non-vacuous frame read/write behavior.

For G304, change the identity API to accept a data-directory root and always use the
fixed `identity.key` name through Go 1.27 `os.Root`. Confine read, temporary creation,
permission setting, and rename to that root. Preserve atomic persistence and mode 0600.
Add an escape-symlink test whose outside file is unchanged and whose contents are not
accepted as an identity. Update `network.Open` to the root-scoped call.

Do not run tests, Gosec, formatters, builds, or any other command except read-only final
hashes/counts. Do not install, edit scanner policy or governance, add suppressions, change
dependencies, use Git, commit, push, or change GitHub state.

When finished, stop and report every final path/hash/line count, test-source-first order,
red reasons, exact boundary/security invariants, API/call-site changes, ambiguities, and
confirmation of no out-of-scope action.
