# Grok Build Handoff — BBGO-SEC-001 Reachable Finding Correction 1

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: `7e03c6bc8c17e00e9151606d1f8e881894059295`

The worktree intentionally contains the uncommitted BBGO-SEC-001 scanner/workflow drop.
Do not alter, stage, or clean it. Before editing, read completely:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`, especially Reachable Finding Correction 1
5. `docs/security/BBGO-SEC-001-EVIDENCE.md`
6. `modern/network/node.go`
7. `modern/network/node_test.go`
8. the local upstream implementations and defaults under
   `/home/lars/go/pkg/mod/github.com/libp2p/go-libp2p-kad-dht@v0.42.1`

Authorized source paths only:

- `modern/network/node_test.go`
- `modern/network/node.go`
- `modern/go.mod`

Author the regression test first. It must fail on the current source because the DHT has
no routing-table IP-diversity filter, and must verify behavior rather than merely search
source text. Then configure every constructed BitBook DHT with
`dht.NewRTPeerDiversityFilter` and the upstream Amino defaults. The
`AllowPrivateAddresses` option may relax address/query/routing peer filters for local or
LAN operation, but it must not disable routing-table IP diversity. Change the direct DHT
requirement from `v0.42.1` to current upstream `v0.42.2`. Do not edit `modern/go.sum`;
Codex Luna owns module-graph regeneration and command execution.

Do not author a scanner exception, allowlist, wrapper, suppression, or workflow change.
Do not run tests, builds, formatters, module commands, or any other command. Do not
install anything. Do not edit governance, handoffs, tickets, or evidence. Do not use Git,
commit, or push.

When finished, stop and report:

- every modified path, SHA-256, and line count;
- test-source-first confirmation and why the test is red on the baseline;
- the exact production mitigation and why private-address mode retains it;
- the dependency version change;
- any ambiguity or unimplemented requirement; and
- confirmation that no command, install, Git operation, or out-of-scope edit occurred.

## Delivered Source Report

Execution date: 2026-08-29

Grok Build reported test-source-first authoring and no test, build, formatter, module,
install, or Git execution. Only `sha256sum` and `wc -l` were used for this report.

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/network/node_test.go` | 448 | `2c791449967c412bc35756400dc832b3ec31557f0d9d868882e5103a4ea4ba74` |
| `modern/network/node.go` | 261 | `5add3a890d232af2ed8f53fcb9bd062660b69608937fdb0ea0fa6e0d86e057d9` |
| `modern/go.mod` | 133 | `a69dbd8a9ab76f75f8329782d1f3309122ac933c1ea10ab8503267f400d3b2ce` |

The new `TestDHTRoutingTableEnforcesIPDiversity` has default and private-address
subtests. It constructs real nodes, rejects a nil diversity-stat result, connects four
same-IP-group peers, requires both admission and a diversity-filter rejection, and
independently recounts whole-table and common-prefix-length IP-group limits. Production
now installs `NewRTPeerDiversityFilter` with Amino's per-CPL limit 2 and whole-table
limit 3 before applying mode-specific options. Private-address mode no longer clears it.
The direct DHT requirement is `v0.42.2`; `modern/go.sum` remains for Luna to regenerate.

Reviewer independently read the complete diff and verified all hashes, line counts,
authorized paths, behavioral assertions, and the absence of scanner-policy changes.
Execution evidence remains required; source inspection is not acceptance.
