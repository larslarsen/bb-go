# BBGO-NET-001 execution 01 — consolidated publication record

Date: 2026-09-18. Actor: Hermes, free Nous Portal model (meituan/longcat-2.0:free).
Reviewer: Codex. Review 16 verified and accepted.

## Current HEAD

`633d31e86360f9c4e7069cc00007301f46d90a94`

## Source identities

| Path | SHA-256 |
| --- | --- |
| modern/go.mod | `1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783` |
| modern/go.sum | `4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a` |
| modern/network/node.go | `ee15f7a120468679a7f52a8e0fa73aa38aa86813ea5a62a493bfd990daf13555` |
| modern/network/protocols.go | `a17f85edf8bb8dd52a12f6d826d1637ad32cd5d27daf534bac672943f1eafd03` |
| modern/network/discovery.go | `4ba9e0b795872301e4350cfb4a43868c2206fb7b1539de85726d9533161b300c` |
| modern/api/handler.go | `678511fee172b1b9f0aef0c85eb5e724f162aa233d65e535bcbe8708630054fd` |
| modern/cmd/bitbookd/main.go | `fe2d3b2c07d3ddc0948889dc37158b90f5aecda79bd83d0974d1f61548f1caea` |
| modern/network/node_test.go | `9abc12fc479252c390f78802c9df546e6c4a4e78d71b09acf9db20ba815ab000` |
| modern/network/open_test.go | `29feea3dfb533bd28e2b1c46a37bf33515787b35867fdc732d72d578cad2ef3d` |
| modern/network/bootstrap_test.go | `cc1376f0f8dec262c460c06b40a8283b74b5e8381cccc4bd1fe4a88a559792ad` |
| modern/network/discovery_test.go | `c8b0a9890509691a2d488066c0040d9ce87e39885436627457a44ea703d0ea66` |
| modern/network/discovery_fuzz_test.go | `c4f15be93af81e4d732fedafc80275aadf16b3bdc401bdcc0d3ca0acfcde8b0e` |
| modern/api/handler_test.go | `92460e5731b2e41e5d70d1c81e96d90036408e9813a38d0df90059170a22c09b` |
| modern/cmd/bitbookd/bootstrap_test.go | `b348eabe4afcef497200767dac8e4a39743441b88cefb0be98d299d1dcd50ef5` |
| modern/cmd/bitbookd/payment_test.go | `f1ac520574b48f22e41b8700d7852214479ccf3cf4b919ffd8be0deaf06eea31` |

All pins verified.

## Expected red (review 03, historical)

Command: `go test ./network ./api ./cmd/bitbookd -run '^TestNET001' -count=1 -timeout=180s`
Exit: 1. Build failed on undefined symbols (production absent).

## Diagnostic capture (review 08)

Command: `go test ./network -run '^TestNET001(IndependentPublicIPFSInterop|ReopenPreservesPublicContentAndKeepsPrivateDatastorePrivate)$' -v -count=1 -timeout=90s`
Exit: 1. Control PASS, target FAIL — `context deadline exceeded` on local block retrieval after restart.

Captured: `modern/dist/net001/diagnostic01/`

## Corrected diagnostic (review 10, diagnostic02)

Same command after Node.Get local-first correction (review 09-10).
Exit: 0. Both tests pass.

Captured: `modern/dist/net001/diagnostic02/`

## Regression and falsification (review 11-14)

Per disposable isolated copies:

| Step | Result |
|---|---|
| Old-source reconnect regression | FAIL (expected) |
| Corrected-source reconnect regression | PASS |
| Outbound-validation falsification | FAIL (fault effective) |
| Restored-source validation | PASS |

## Developer targeted results (reviews 12-14, developer05)

| Check | Result | Artifact |
|---|---|---|
| Focused network/API (count=20) | PASS | `modern/dist/net001/developer05/01-focused-count20.json` |
| Race (count=10) | PASS | `modern/dist/net001/developer05/02-race-count10.json` |
| Three-package TestNET001 (count=5) | PASS | `modern/dist/net001/developer05/03-targeted.json` |

## Full acceptance suite (review 15, acceptance06)

| Gate | Result |
|---|---|
| `go test ./... -count=1 -timeout=300s` | PASS |
| `go test -race ./... -count=1 -timeout=600s` | PASS |
| `go vet ./...` | PASS |
| `FuzzPeerHello` 30s | PASS (222,817 executions) |
| DHT diversity | PASS |
| Govulncheck policy | PASS (DHT GO-2024-3218 within scope) |
| `go build -o bitbookd ./cmd/bitbookd` | PASS |

Artifacts: `modern/dist/net001/acceptance06/`

### Gosec findings (11 findings, all test-only, non-blocking per review 16)

| Finding | Site | Disposition |
|---|---|---|
| G115 | discovery_test.go:966 | Bounded conversion |
| G204 | payment_test.go:285; bootstrap_test.go:243 | Safe os.Executable |
| G304 | identity_test.go:84; localclient_test.go:110 | Owned temp reads |
| G104 | handler_test.go:591,596,602,620,625,631 | Fixture cleanup |

### Build identity

- Path: `modern/bitbookd`
- SHA-256: `b885b1d23fe3da0b7ec7de2b1817a4f9827ac45a32be1386bca3dfaa3af3ef6e`
- Size: 44,001,199 bytes
- VCS: 633d31e8..., modified=true

## Publication

Staged: 13 source files + this report.
Secret scan: 0 leaks (102 KB).
Whitespace check: clean.

### Feature commit

```
0951c837 feat(network): public IPFS bootstrap, BitBook peer discovery, local-first block reads
14 files changed, 3413 insertions(+), 41 deletions(-)
```

Push: `e445e13c..0951c837  HEAD -> master`

### CI

GitHub Actions `Go 1.27` run 35320581742: **success** (completed 07:42:35 UTC).
