# BBGO-NET-001 — use the public IPFS swarm

Status: QUEUED — owner direction recorded 2026-09-17; not implemented.
Reviewer: Codex, High. Owner: Lars. ACC-002 remains the active source task.

## Owner direction

Use the existing public IPFS swarm and its bootstrap infrastructure instead of
requiring BitBook-operated seed nodes. Normal startup should discover the network
automatically, without a user Connect button or manual bootstrap configuration.
Explicit bootstrap overrides remain available for development and administration.
This replaces the earlier isolated-network direction for the shared networking
substrate; it does not change account identity, naming or trust decisions.

## Existing implementation and correction

The earlier version of this ticket already proposed public IPFS bootstrap peers,
but incorrectly described that as a single-file, test-free change. That proposal
has not been implemented and its instructions are superseded by this section.

Reviewed at daemon HEAD `dd1abf272deb6ff1f5d8f31643094d8808b65704`:

- `modern/network/protocols.go` selects `/bitbook/kad/1.0.0` and
  `/bitbook/ipfs/bitswap/1.2.0`. Public bootstrap addresses alone do not make these
  protocols compatible with public IPFS peers.
- `modern/network/node.go` constructs the DHT and Bitswap in `New`, not `Open`.
  One failed explicit bootstrap connection currently closes the node.
- `modern/cmd/bitbookd/main.go` emits the no-bootstrap warning from the raw CLI
  arguments. Changing node.go alone cannot correct that message.
- The pinned go-libp2p-kad-dht v0.42.2 provides public default bootstrap addresses;
  its `amino/defaults.go` identifies the public DHT as `/ipfs/kad/1.0.0`.
  Findings use the locally cached, pinned dependency source.

## Implementation requirements

The next bounded networking contract must cover public IPFS routing and bootstrap
compatibility, and resolve the current Bitswap prefix as part of public content
exchange. Retain BitBook's application-specific direct/payment protocols and their
authentication; an arbitrary IPFS peer is not automatically a BitBook account or a
trusted actor. Preserve the existing private/public content boundary: private
messages, authority snapshots, contacts and wallet data do not become public blocks.

Default startup must tolerate unavailable bootstrap peers, bound connection work,
and report connectivity accurately. Preserve explicit override behavior and provide
an explicit no-public-network configuration for local/offline tests. Existing
protocol-isolation assertions will need to reflect the intended shared substrate.
Prove interoperability and discovery with controlled local peers using the public
protocols; tests must not depend on reachable public bootstrap services. Public
IPFS routing does not itself supply BitBook application discovery or guarantee
replication/availability; identify any existing application discovery work still
needed when bounding the implementation.

This is networking direction, not a requirement to operate a seed fleet. Public
bootstrap peers are initial entry points, not authorities over accounts or trust.
Community trust-profile bootstrap is a separate concept and is unchanged.

## Authorization and reviewer publication

No source, test execution, daemon restart or network migration is authorized by this
queued ticket. Freeze exact implementation/test paths and commands when it becomes
active. ACC-002's isolated durable account storage can proceed unchanged.

This owner-direction update is part of one reviewer-only publication based on the
HEAD above, limited to `tickets/BBGO-NET-001.md`, `tickets/BBGO-ACC-002.md` and
`docs/handoff/CURRENT_TASK.md`. The latter two also record the concurrent ACC-002
test-source review. No developer source, tests or unrelated work are included.
