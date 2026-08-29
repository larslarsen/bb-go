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

## Why there is no `go.mod` yet

The embedded OpenBazaar IPFS fork predates Go modules and imports dozens of
content-addressed `gx/ipfs/...` packages. Adding a superficial module file
would bypass or break those pinned packages. Moving to modules therefore needs
to happen together with replacing the embedded IPFS/libp2p stack, not as an
independent metadata change.

The current checkpoint makes the application reproducibly compile and test on
Go 1.27 while preserving its network protocol. The next infrastructure phase
is to replace the `gx` dependency graph with maintained Kubo/libp2p modules and
then generate a conventional `go.mod` and `go.sum`.
