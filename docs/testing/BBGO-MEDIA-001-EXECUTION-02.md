# BBGO-MEDIA-001 M2B — Sol High execution record

Date: 2026-09-18. Actor: Principal Dev, Codex Sol High. Scope: the active M2B
durable public attachment-retention assignment only. No Git mutation, daemon build,
restart, HTTP/API wiring, social/post integration, private-file behavior or UI work was
performed.

## Baseline and test-first red

All frozen inputs matched before editing:

| Path | SHA-256 |
| --- | --- |
| `modern/network/files.go` | `5a2f7a8515fa4578edbedf3ec9899791c5f8145c570aa9d568c232a10a727492` |
| `modern/network/files_test.go` | `ec2df5adaebe5a0793e0020404158c876e9b2642b1a8a6cb0d56259bccee96c2` |
| `modern/network/files_fuzz_test.go` | `1951689375fb002eb81d34acf3e5c9e8c1cbf55411a679683ccc83bd2a37944a` |
| `modern/network/node.go` | `ee15f7a120468679a7f52a8e0fa73aa38aa86813ea5a62a493bfd990daf13555` |
| `modern/network/open.go` | `96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b` |
| `modern/go.mod` | `df7f1e5d4fa1d20083c1fcc872415a4203f6afefb4a7b2a980d45cdd286b3c38` |
| `modern/go.sum` | `58614c1e27170525cf205f0f03c54085c70e4b37e51ee0aebf73a75ff955e800` |

`modern/attachment` and this report did not exist. Tests were authored before
production. At red, `store_test.go` was 566 lines, SHA-256
`35b879b740e0c7089e87095db93749223fee59dd97e1805762923f100e7d8169`,
and `store_fuzz_test.go` was 23 lines, SHA-256
`5edda2649728915599d20a152a872c078eebfa8c70f5d9721d0bd57607426a8e`.

From `modern`, the intended red command was:

```sh
go test ./attachment -run '^TestMEDIA001M2B' -count=1 -timeout=240s
```

Exit: 1. It failed to compile only because the new package contract and internal
record codec were absent (`NewReferenceID`, `ReferenceID`, `ParseReferenceID`,
`Store`, `decodeRecord` and related new identifiers were undefined). Raw output:
`modern/dist/media001/m2b-developer01/raw/01-contract-red.txt`, SHA-256
`783cc07f6a1286b7eb26cfe917097d71e182bf003a67c01e215425e1bf3acaf3`.
The exit capture contains `1` plus a newline, SHA-256
`4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865`.

## Implementation

The new `attachment` package provides the frozen public API and stable `errors.Is`
sentinels for invalid IDs, missing/not-ready references, conflicts, quota exhaustion,
corrupt persisted state and closed stores.

- Reference IDs are exactly 128 random bits from `crypto/rand`, encoded as 32
  lowercase hexadecimal characters. Parsing rejects zero, uppercase, padding,
  separators, whitespace and wrong lengths.
- Strict bounded JSON records live only below
  `/bitbook/attachment/public/v1/reference/`. The decoder rejects duplicate or
  unknown fields, absent fields, unsupported versions/states, invalid ID/CID/length
  values, oversized records and trailing data.
- A namespaced Boxo v0.42.1 `dspinner` uses the node's existing datastore,
  blockstore and Bitswap exchange. Recursive pins use the package name prefix
  `bitbook-attachment-public-v1:`. The store closes only its pinner.
- Import reserves one reference plus `maxBytes` before reading. Known-root retain
  reservations account a distinct CID/length once, including while concurrent work
  is pending. Successful imports reconcile their maximum reservation to the actual
  unique-root length. Checked arithmetic and configured ceilings prevent overcommit.
- Retain validates the complete M2A file through `CopyPublicFile` before persisting a
  transition. A ready ID retry is returned without reading an import body; an exact
  retain retry is idempotent and a mismatched descriptor is a conflict.
- Retain ordering is durable `retaining` record, recursive pin and pinner flush, then
  durable `ready` replacement. Release writes `releasing` before the last unpin and
  deletes the record only after unpin/flush. Successful `Put` followed by failed
  `Sync` remains internally not-ready and recoverable rather than becoming usable.
- Open bounds record loading, verifies every file from local blocks without network
  fallback, reconstructs quota accounting, fails closed on ready records without the
  exact package recursive pin, completes retaining/releasing transitions, removes
  only package-named unreferenced pins, and preserves unrelated pins.
- Operations use coordinated admission and owned cancellation. `Close` rejects new
  work, cancels admitted work, waits for it, then closes the pinner without closing
  shared Bitswap or the node.

No dependency version or classification changed; `modern/go.mod` and `modern/go.sum`
retain their baseline hashes.

## Test coverage

The final suite contains 11 `TestMEDIA001M2B...` top-level tests and one decoder fuzz
target. It covers canonical/random-source ID behavior; explicit limit boundaries;
reference and logical-byte reservations; overflow; no-read quota and duplicate-ID
proofs; empty, one-chunk and multi-chunk imports; every reachable DAG block's pin
status; a disk-backed datastore reopen with byte-for-byte copy; duplicate root
accounting and release order; simultaneous identical imports; missing, corrupt,
malformed and length-mismatched files; retaining/releasing recovery; package orphan
cleanup and unrelated-pin preservation; datastore `Put`, `Sync` and `Delete` failures;
pinner `Pin`, `Flush` and `Unpin` failures; ready-without-pin fail-closed behavior;
import/validation/pinning cancellation; concurrent get/retain/release; close during
admitted work; post-close rejection; strict record fixtures; and actual decoder fuzzing.

The disk reopen fixture stores its temporary LevelDB under the disk-backed
`modern/dist/media001/` tree and removes it through test cleanup. All network tests use
controlled loopback nodes with no public bootstrap peers, credentials, user daemon or
user data.

## Restart-retention falsification

Against the final production source, the recursive `pinner.Pin` call was temporarily
replaced with a successful no-op. The temporary source SHA-256 was
`5d82b6690296fe1434df31a3bc0620a78b8621380f4cd9b3839f1ad810ad1cef`.
The retained temporary diff is
`modern/dist/media001/m2b-developer01/raw/02-pin-noop-falsification.diff`, SHA-256
`44154a2334ec0097227d3c10a700461c8a2afd0a05e6dd534135b666891eb483`.

The exact falsification command was:

```sh
go test ./attachment -run '^TestMEDIA001M2BReadyPersistsRecursivePinAfterReopen$' -count=1 -timeout=240s
```

Exit: 1, expected. Reopen failed specifically because the ready reference root was not
recursively pinned with its package name. Raw output:
`03-pin-noop-falsification-red.txt`, SHA-256
`708d2fb86d62b5b26d14422230e07d21949961c9b12f27ec90f4e018e589d10b`.

The exact production file was restored byte-for-byte to SHA-256
`9be0ddc1b12dc8650f070cc5532cda68cbde25ba89dd129f128a1764b7297aea`.
The same command then exited 0; package time 0.013s. Raw output:
`04-pin-restored-green.txt`, SHA-256
`95787b13b7dba1f15662be0dc5159ffc2712259a71d9256c4849c0ba9e93bc74`.
The red and green exit captures contain `1` and `0`, respectively.

## Controlled environment and final results

Commands ran from `modern` with the cached Go 1.27.0 linux/amd64 executable,
SHA-256 `1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`,
and `GOTOOLCHAIN=local`, `GOWORK=off`, `GOENV=off`, `GOPROXY=off`, `GOSUMDB=off`,
`GOFLAGS='-mod=readonly -p=2'`, `GOMAXPROCS=2`, plus the accepted disk-backed
developer01 module/build caches. The fresh capture filesystem reported `ext2/ext3`.
Environment, version, executable-hash and filesystem capture SHA-256 values are,
respectively, `0be2ef7501fc36f7093fb7f7b3b172df74ba32c0bcd4f0ba98520af49e9b6004`,
`76227025cc0bc2be7067aa45d11e09cacfd49c58f498f4c2e4f6a9872a607bf9`,
`21916595f2947bb24476ee74b7e2ba472af1f64719c219fb985004eb5b7f7ce5`, and
`212f060701ef6bb38aa6d992cc93903732c7a687f486095dd6cc568f5380b275`.

The exact final commands were:

```sh
go test ./attachment -run '^TestMEDIA001M2B' -count=1 -timeout=240s
go test -race ./attachment -run '^TestMEDIA001M2B' -count=1 -timeout=360s
go test ./attachment -run '^$' -fuzz '^FuzzMEDIA001M2BRecord$' -fuzztime=30s -parallel=2
```

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| targeted green | 0; 11 top-level tests passed; package 0.218s | `05-targeted-green.txt`; `d9cb0f9baa4a8c94e30a1393cae0e56af9a1f704f7ccbdea78ad754636c50319` |
| targeted race | 0; the same 11 tests passed under the race detector; package 1.722s | `06-targeted-race-green.txt`; `641520224ee4289e8281f9f2079294b1bb3e01a687c8df34942493680b848e79` |
| record fuzz | 0; 229,063 executions, 251 total interesting inputs, no failure; package 30.560s | `07-record-fuzz.txt`; `8618817f73f99da323207267223b5f38253f0fa7690800b94a492e4a42034675` |

Each final `.exit` capture contains `0` plus a newline, SHA-256
`9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.

## Final identities and limitations

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/attachment/types.go` | 96 | `58d94fd115f2fb35ab2f862cad9f359fae439bc41ca97273095ee0160c26604f` |
| `modern/attachment/codec.go` | 154 | `4cb3fbe7881830f563753607ded8ca6f4a34c7c72fde5dc46c4b3493b46efbbc` |
| `modern/attachment/store.go` | 785 | `9be0ddc1b12dc8650f070cc5532cda68cbde25ba89dd129f128a1764b7297aea` |
| `modern/attachment/store_test.go` | 1,038 | `5aab04d1b6eb1d5a7fc8f75c8202a699230094bd182ae6b1a19f29528a14c415` |
| `modern/attachment/store_fuzz_test.go` | 23 | `5edda2649728915599d20a152a872c078eebfa8c70f5d9721d0bd57607426a8e` |

Formatting is clean. The M2A source/tests and module files remain at their frozen
hashes. Generated captures are ignored evidence and are not publication inputs.

`RetainPublic` intentionally permits controlled Bitswap retrieval, so callers must
provide cancellation/deadlines appropriate to remote availability. Arbitrary blocking
reader I/O remains caller-owned and cooperative, as in M2A. Failed imports can leave
unreferenced partial blocks, and failed/crashed transitions can leave conservative
pins; block deletion, GC and cache eviction remain later work. The store must be closed
before its node, as required by the contract. No broad package/module tests, scanner,
dependency fetch, binary build, restart or Git publication was performed. The complete
source/test/report drop is ready for reviewer inspection.

## Review 08 correction — Sol High

This bounded correction changes only the four authorized attachment source/test files.
`Release` now persists the `releasing` transition before deleting every ready claim,
including a claim whose root has another owner. A failed delete sync therefore leaves
the uncertain claim unavailable while the surviving claim remains usable and pinned.
`Open` validates the package-owned pinner dirty marker as exactly one byte containing
zero or one before calling Boxo. Strict record decoding now rejects JSON `null` for all
five required scalar fields. The disk-reopen fixture uses `t.TempDir` and registers
datastore, node and attachment-store cleanup immediately after each acquisition.

The new tests were written before the production changes. The required focused command
ran against production files restored to the submitted baseline hashes
`9be0ddc1b12dc8650f070cc5532cda68cbde25ba89dd129f128a1764b7297aea`
for `store.go` and
`4cb3fbe7881830f563753607ded8ca6f4a34c7c72fde5dc46c4b3493b46efbbc`
for `codec.go`:

```sh
go test ./attachment -run '^TestMEDIA001M2B(SharedReleaseSyncFailure|CorruptPinnerMarker|RecordNullScalar)$' -count=1 -timeout=240s
```

It exited 1 as intended. Both shared-delete persistence models exposed the released
claim as ready, and the empty pinner marker reached Boxo's out-of-range panic. Raw
output: `modern/dist/media001/m2b-developer02/raw/01-focused-red.txt`, SHA-256
`085286e5e8640688fe5b4bbac2d3fa899c1525db25c29827fd752e9dc5c4d643`.
Its exit capture contains `1`, SHA-256
`4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865`.
Because the panic stopped the remaining case, the separately authorized command

```sh
go test ./attachment -run '^TestMEDIA001M2BRecordNullScalar$' -count=1 -timeout=240s
```

also exited 1 and showed that an otherwise-valid real empty-file record accepted
`byteLength:null`. Raw output: `02-null-scalar-red.txt`, SHA-256
`d932033ae26687f7d92050e0496db5198c41af6f19ec8e1260f51050e9645fb7`;
its exit capture has the same exit-1 hash above.

Commands ran from `modern` with the accepted cached Go 1.27.0 linux/amd64 executable,
SHA-256 `1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`,
and the prior controlled offline settings: `GOTOOLCHAIN=local`, `GOWORK=off`,
`GOENV=off`, `GOPROXY=off`, `GOSUMDB=off`, `GOFLAGS='-mod=readonly -p=2'`,
and `GOMAXPROCS=2`. The accepted developer01 module/build caches were reused.
`TMPDIR` was set to the newly created `modern/dist/media001/m2b-developer02/tmp`
directory. Its inspected workspace volume reported ext4 through `df` and `ext2/ext3`
through `statfs`, with 4096-byte blocks. Environment, version, executable-hash and
filesystem capture hashes are, respectively,
`7dfbe530e880562de5b8d9c6800668a45956d4f04f4a873adba9def2cfd87104`,
`76227025cc0bc2be7067aa45d11e09cacfd49c58f498f4c2e4f6a9872a607bf9`,
`21916595f2947bb24476ee74b7e2ba472af1f64719c219fb985004eb5b7f7ce5`, and
`a98f920a64c8aa794c96588e19c5ff035e90c07ef61a82268710e695bf773361`.

After the production fixes, the required commands produced:

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| focused correction | 0; all three named regressions passed; package 0.038s | `04-focused-green.txt`; `a7f1985d480198158ff7de1d12d15a6f64908e257057a13d8c882ee0d69c0c88` |
| targeted green | 0; all 14 top-level M2B tests passed; package 0.238s | `05-targeted-green.txt`; `0d93e05f7b7c8ba1fb9b15805e071cb6797fde78f014bdfd1436c0ca0a298e8e` |
| targeted race | 0; the same targeted suite passed under the race detector; package 1.718s | `06-targeted-race-green.txt`; `298909ffa75f25097bb2b5bc6abfaf2ffec17605b071040501bcd2dca19922e4` |
| record fuzz | 0; 238,048 executions, 306 total interesting inputs, no failure; package 30.122s | `07-record-fuzz.txt`; `cb48ffba78dda37a34dc1e6c725ec28758e51f34344cd0a7fd514a0ad6d129f5` |

Each green `.exit` capture contains `0` plus a newline, SHA-256
`9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.
The final three commands were the unchanged targeted, race and 30-second fuzz commands
listed in the original M2B results above.

Final correction identities are:

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/attachment/types.go` (frozen) | 96 | `58d94fd115f2fb35ab2f862cad9f359fae439bc41ca97273095ee0160c26604f` |
| `modern/attachment/codec.go` | 166 | `97dd8ef6a4e79b69617df6355301487a979a13bd79a4e1d843dc95007b99c482` |
| `modern/attachment/store.go` | 796 | `2898c1e48800fb860b69db720b9d04755c80191d23199ba6bb6f9dea6047daa2` |
| `modern/attachment/store_test.go` | 1,223 | `2bcf30f26a845f7beb67b3b25a5034799b75fd05d83e92814c43be8f9a58ac68` |
| `modern/attachment/store_fuzz_test.go` | 28 | `1c593c5f1e67b08d8f364cb284252d678f83da1722d68e03aed083f652cb521a` |

The module files remain frozen at SHA-256
`df7f1e5d4fa1d20083c1fcc872415a4203f6afefb4a7b2a980d45cdd286b3c38`
for `modern/go.mod` and
`58614c1e27170525cf205f0f03c54085c70e4b37e51ee0aebf73a75ff955e800`
for `modern/go.sum`. Formatting and scoped diff checks are clean. No broad tests,
scanner, dependency fetch, build, restart, Git operation or publication was performed.
The original M2B limitations remain unchanged.

## Review 09 — Hermes acceptance and publication

Date: 2026-09-18. Actor: Hermes, Jr Dev. M2B source accepted; executing acceptance
and publication per review 09 authorization.

### Source verification

All five M2B source pins verified. All seven M2A/module frozen inputs verified.

### Commands

| # | Command | Result |
|---|---|---|
| 1 | `go test ./... -count=1 -timeout=300s` | PASS (all 10 packages) |
| 2 | `go test -race ./... -count=1 -timeout=600s` | PASS |
| 3 | `go vet ./...` | PASS (clean) |
| 4 | `gosec -tests ./attachment/...` | 2 findings: G115 store.go:710 (int64→uint64 conversion in validated length check, non-blocking) |
| 5 | `go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$'` | PASS |
| 6 | `python3 scripts/govulncheck_policy.py source` | PASS (DHT GO-2024-3218 within scope) |
| 7 | `go build -o bitbookd ./cmd/bitbookd` | PASS |

### Build identity

- Path: `modern/bitbookd`
- SHA-256: `2570d121e903a2c5b032867aa841ff26a7c57ed5ef29267aaa2d17e86a0e6242`
- Size: 44,992,241 bytes
- VCS: fd17ae19..., modified=true

### Publication

Staged: 5 source files + EXECUTION-02.md.
Secret scan: 0 leaks.
Whitespace check: clean.
