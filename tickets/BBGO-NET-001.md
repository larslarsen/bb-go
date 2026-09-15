# BBGO-NET-001 — Default to public IPFS bootstrap peers when none configured

Status: ACTIVE — ticket only, no execution authorized yet.
Reviewer: Codex. Owner: Lars.

## Problem

`modern/cmd/bitbookd/main.go` requires `--bootstrap` flags or the daemon logs
"no bootstrap peers configured; this node will save locally until a peer is
supplied". Keel's daemon (`keel/daemon/swarm/swarm.go` lines 471-477) falls back
to `dht.GetDefaultBootstrapPeerAddrInfos()` (public IPFS bootstrap peers) when
none are configured. BitBook should behave the same way — work out of the box
without manual bootstrap configuration.

## Change

Single file: `modern/network/node.go`.

In the `Open` function, when `len(cfg.BootstrapPeers) == 0`, default to
`dht.GetDefaultBootstrapPeerAddrInfos()` instead of leaving the DHT
unconfigured. Existing explicit `--bootstrap` behavior is unchanged.

## Verification

No tests required. After the change, running `bitbookd` without `--bootstrap`
should produce no "no bootstrap peers configured" warning and the node should
join the public DHT network.

## Scope

Source path: `modern/network/node.go` only. No other files. No tests.
No governance changes. No publication scope.
