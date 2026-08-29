# BitBook modern core

This module is the maintained replacement for the daemon's 2018 `gx`-based
IPFS fork. It targets Go 1.27 and the dependency versions used by Kubo 0.43.

The first migration boundary is intentionally small:

- current libp2p host and transports;
- Kademlia isolated at `/bitbook/kad/1.0.0`;
- Boxo Bitswap isolated at `/bitbook/ipfs/bitswap/1.2.0`;
- persistent Ed25519 peer identities;
- a block API covered by a real two-peer transfer test; and
- signed IPNS roots that preserve peer-ID-based social data discovery.

Run its tests independently of the legacy daemon:

```sh
GOTOOLCHAIN=go1.27.0 go test ./...
```

The new Bitswap namespace is a deliberate protocol-generation break. Boxo's
supported prefix mechanism retains `/ipfs` within the protocol name. Using it
lets BitBook stay on upstream Boxo rather than maintaining another fork. The
legacy network has no maintained bootstrap service, so compatibility with the
2018 Bitswap namespace would not provide a usable migration path.
