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
