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

## Delivered Source Report

Execution date: 2026-08-29

Grok reported test-source-first authoring and no test, Gosec, formatter, build, install,
Git, scanner-policy, suppression, or dependency action. Only final read-only
hashes/counts were run.

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/direct/service_test.go` | 268 | `aafadcd22068645522116ab3bec8ee53cd288b6269ad3bbe54f48ae56a89f026` |
| `modern/direct/service.go` | 769 | `207946dc49c71333d8095161daf7cd3b0b95cdfceee7d01fc7c82cf81d7ef2ea` |
| `modern/network/identity_test.go` | 99 | `29f52babeafdd14dbb25c8c964ebc40f1214ffa4450b0d08435fd22ea73e8d11` |
| `modern/network/identity.go` | 95 | `b43f6ad90149ce41526aa3b0f85329dbbf847feaaf0442529a204c637833b4fa` |
| `modern/network/open.go` | 67 | `96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b` |

The frame tests cover below/at/above 128 KiB, above `math.MaxUint32` without allocation,
real big-endian round trip, and invalid prefixes. Production derives `uint32` only after
the checked helper. Identity tests retain persistence/0600 and reject an outside-root
symlink while proving its bytes and link remain unchanged. Production uses the fixed
`identity.key` through `os.Root`; `network.Open` supplies only its data directory.

Reviewer independently read the complete five-file diff and verified hashes, counts,
wire-format preservation, root confinement, and scope. Execution remains required.
