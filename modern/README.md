# BitBook maintained daemon

This module is the maintained replacement for the daemon's 2018 `gx`-based
IPFS fork. It targets Go 1.27 and the dependency versions used by Kubo 0.43.

This module now contains:

- current libp2p host and transports;
- Kademlia isolated at `/bitbook/kad/1.0.0`;
- Boxo Bitswap isolated at `/bitbook/ipfs/bitswap/1.2.0`;
- persistent Ed25519 peer identities;
- signed IPNS roots that preserve peer-ID-based social data discovery;
- signed profiles, posts, and following lists backed by immutable blocks; and
- a social-only `/ob` HTTP API and runnable `bitbookd`.

Build and run:

```sh
cd modern
GOTOOLCHAIN=go1.27.0 go build -o bitbookd ./cmd/bitbookd
./bitbookd \
  -bootstrap /ip4/203.0.113.10/tcp/4001/p2p/12D3KooW...
```

The API binds to `127.0.0.1:4002` by default and the libp2p transports bind
to TCP and QUIC port 4001. Identity and state live under
`~/.bitbook/modern`. Repeat `-bootstrap` for more than one seed, or use
`-dht-server` when operating a publicly reachable bootstrap node. For a
private-address development network, add `-allow-private`.

Implemented endpoints include config and peer status, local and remote
profiles, signed posts, following lists, and explicit publication. Wallet and
marketplace endpoints deliberately return 404. Followers and chat still need
their direct peer protocols and are the next migration boundary.

Run all modern tests independently of the legacy daemon:

```sh
GOTOOLCHAIN=go1.27.0 go test ./...
```

The new Bitswap namespace is a deliberate protocol-generation break. Boxo's
supported prefix mechanism retains `/ipfs` within the protocol name. Using it
lets BitBook stay on upstream Boxo rather than maintaining another fork. The
legacy network has no maintained bootstrap service, so compatibility with the
2018 Bitswap namespace would not provide a usable migration path.
