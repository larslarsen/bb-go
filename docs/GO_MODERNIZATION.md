# Go modernization

BitBook is tested with Go 1.27.0. Use `scripts/go.sh` for builds, tests, and
development commands:

```sh
./scripts/go.sh build -o bitbookd ./bitbookd.go
./scripts/go.sh test -vet=off ./api ./net/service ./core ./cmd
./scripts/go.sh run bitbookd.go start --social-only --disablewallet
```

The wrapper creates a temporary GOPATH entry and selects Go 1.27.0 through
Go's toolchain support. Set `BB_GO_GOPATH` to override the temporary GOPATH or
`GOTOOLCHAIN` to test another compiler.

## Module migration

The embedded OpenBazaar IPFS fork predates Go modules and imports dozens of
content-addressed `gx/ipfs/...` packages. Adding a superficial module file
would bypass or break those pinned packages. Moving to modules therefore needs
to happen together with replacing the embedded IPFS/libp2p stack, not as an
independent metadata change.

The replacement has started in [`modern/`](../modern). It is a conventional Go
1.27 module pinned to the maintained networking versions used by Kubo 0.43:

- Boxo 0.42.1;
- go-libp2p 0.49.0; and
- go-libp2p-kad-dht 0.42.1.

It now owns peer identity, persistent network and social storage, the BitBook
DHT, Bitswap block transfer, signed IPNS roots, profiles, posts, and following
lists. The `modern/cmd/bitbookd` command exposes those functions through the
social-only portion of the historical `/ob` API. Its integration tests start
two real libp2p peers, publish signed social state, resolve it by peer ID, and
retrieve and verify the content from the reader.

```sh
make modern_test
```

The legacy daemon remains buildable through `scripts/go.sh` while followers,
direct messaging, media, and any required desktop compatibility are moved
behind the new boundary. The maintained daemon already has no `gx`
dependency; the root `vendor/gx` tree can be removed after the remaining
legacy runtime is retired.

## Protocol generation

The maintained DHT keeps the existing `/bitbook/kad/1.0.0` ID. Bitswap moves
to `/bitbook/ipfs/bitswap/1.2.0`: Boxo's upstream prefix mechanism deliberately
retains the `ipfs` component. This creates an isolated second-generation
BitBook network without carrying a permanent Boxo fork solely to reproduce the
2018 protocol spelling.
