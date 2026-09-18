# BBGO-MEDIA-001 execution 01

Source review and broader acceptance are pending. This section records the Sol High
M2A developer drop and targeted evidence only. It makes no acceptance, publication,
build, or daemon-restart claim.

## Sol High — M2A file primitive

Date: 2026-09-18. Actor: Principal Dev, Codex Sol High.

### Baseline and environment

The five ticket inputs matched before editing:

| Path | SHA-256 |
| --- | --- |
| `modern/go.mod` | `1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783` |
| `modern/go.sum` | `4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a` |
| `modern/network/node.go` | `ee15f7a120468679a7f52a8e0fa73aa38aa86813ea5a62a493bfd990daf13555` |
| `modern/network/open.go` | `96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b` |
| `modern/network/node_test.go` | `9abc12fc479252c390f78802c9df546e6c4a4e78d71b09acf9db20ba815ab000` |

HEAD at assignment was `68d73d55f00b9aef4f106122551dcc425d7a4ce2`.
`/tmp` was `tmpfs`; the repository filesystem was `ext4`. Generated evidence and
caches are retained under `modern/dist/media001/developer01/`; this directory is
ignored and is not a publication input. The Go build cache is `gocache`, the complete
offline module cache is `.gomodcache`, and the one-module network prerequisite cache
is `.gomodcache-net` beneath that retained directory.

The executable was the cached Go 1.27.0 toolchain at the standard module-toolchain
location (`golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go`), SHA-256
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
All tests used `GOTOOLCHAIN=local`, `GOWORK=off`, `GOENV=off`, `GOPROXY=off`,
`GOSUMDB=off`, `GOFLAGS='-mod=readonly -p=2'`, and `GOMAXPROCS=2`, plus the retained
disk-backed `GOCACHE` and `GOMODCACHE`. Loopback tests required the runner's approved
socket escalation; they remained isolated, offline, credential-free, and used no
public peers or user daemon/data.

### Exact prerequisite and module adjustment

The first attempt could not reach the intended red because the pinned Boxo graph
lacked the cached archive for
`github.com/crackcomm/go-gitignore@v0.0.0-20241020182519-7843d2ba8fdf`.
After the reviewer added the ticket's exact-cache prerequisite, only that version was
downloaded into `.gomodcache-net`. Go verified:

- module sum: `h1:dwGgBWn84wUS1pVikGiruW+x5XM4amhjaZO20vCjay4=`;
- `go.mod` sum: `h1:p1d6YEZWvFzEh4KLyvBcVSnrfNDDvK2zfK/4x2v/4pE=`.

Raw JSON: `raw/04-prerequisite-download-escalated.json`, SHA-256
`224b38937865bfc1e7dc6cb9e8f50846f1b2fb3320969a48cc672614ecca173a`.
No other network dependency retrieval occurred. A local-file proxy populated the
ticket's disk cache from the existing module cache and the verified prerequisite;
all subsequent commands used `GOPROXY=off`.

`go mod tidy` used `GOFLAGS='-p=2'`. It preserved every dependency version. It promoted
the directly imported `go-ipld-format` and `go-multihash`, recorded the already-pinned
Boxo transitive modules needed by UnixFS, and added only the two verified
`go-gitignore` sums. The pre-apply diff is `raw/06-tidy-diff-local-cache.txt`, SHA-256
`fbf266a3d04939b43127abfef041e425591db32b8d1f4648488dafbde7329b1a`.
The actual tidy exited 0 with empty output (`raw/07-tidy.txt`, SHA-256
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`).

### Test-first red

Tests were authored before `modern/network/files.go`. From `modern`, the exact ticket
command was:

```sh
go test ./network -run '^TestMEDIA001' -count=1 -timeout=180s
```

Exit: 1, expected red. Compilation failed only on the absent M2A API, error values,
limits, and bounded block validator. Raw output:
`modern/dist/media001/developer01/raw/09-intended-red.txt`, SHA-256
`bdbb1af6c724661e0fc9302c577dbe4652697745a82a99de8f48ca967e726142`.

### Implementation and proof coverage

`ImportPublicFile` uses explicit unixfs-v1-2025 file parameters: balanced layout,
1 MiB fixed chunks, raw leaves, 1024 links, CIDv1, SHA2-256, and a 32-byte digest.
Its operation-bound DAG service replaces the importer's `context.TODO` on every store
and Bitswap notification. Invalid limits are rejected before reading; an extra byte,
source error, context cancellation, store error, or notification error returns a zero
descriptor. Failed imports may leave unreferenced partial blocks and never delete
possibly shared blocks.

`CopyPublicFile` performs synchronous, local-first, per-occurrence traversal without
prefetch workers. Every local or remote block is hash-verified before decode/output.
It accepts only the specified raw and DAG-PB CID profile and file types; verifies
inline data, block tables, child sizes, arithmetic, and exact declared EOF; and enforces
2 MiB blocks, 1024 links, depth 32, 4096 occurrences, 256 MiB processed encoding, and
100 MiB decoded/declared data. A failed copy reports the written prefix and never emits
beyond the descriptor length.

The suite has seven top-level `TestMEDIA001` functions, sixteen named subtests
(23 test events), and one fuzz target with three seed modes. It covers empty,
single-chunk, exact-chunk, and multichunk round trips; caller-limit boundaries;
determinism; the fixed external multichunk golden root
`bafybeiam4ft5qmcvb6nje45lvd55rb6ryr4fkd27h6llk22f7ybx3g4egq`; and an independent
Boxo importer/reader. The controlled recipient starts without every root/leaf fixture
block before Bitswap retrieval. Persistence reopens an isolated LevelDB datastore;
a private-namespace sentinel is not served as a public block.

Adversarial cases include in-flight caller cancellation and Node close while storage
is blocked, unavailable remote-child deadline, source/storage/output failure, invalid
and short I/O counts, nil and typed-nil inputs, concurrent copies, operation after
close, local hash corruption, unsupported node kinds/CIDs, inline parent data,
missing/mismatched/overflowing size tables, equal-total swapped child sizes, repeated
links, and below/at/above controls for links, depth, occurrences, and encoded bytes.
The fuzz target reaches both valid-CID raw/DAG-PB decoding and mismatched-CID rejection.

### Import-limit falsification

The temporary production fault was exactly:

```diff
- limited := &publicFileLimitReader{ctx: opctx, source: src, maximum: maxBytes}
+ limited := &publicFileLimitReader{ctx: opctx, source: src, maximum: maxPublicFileBytes}
```

With that fault, the focused import-limit test exited 1: the above-limit input returned
a usable CID and 1,048,577-byte descriptor. Raw output:
`raw/17-import-limit-falsification-red.txt`, SHA-256
`b98203bb6a6441614c85434e3504f9edb7593706bda72068650a8741d2820de9`.

The exact source was restored. Its pre/post SHA-256 was
`f088afa9f213a5b112b4443b49eb8bbb025b7436fc029d457b06e396bfab032d`.
The restored focused test exited 0 (`raw/19-import-limit-restored-green.txt`, SHA-256
`ea8ee473c0ee4824bb597335c532cb633d36f2e61b540bad8d66e9d82644e971`).

### Final targeted results

All commands ran from `modern` with the offline environment above.

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| `go test ./network -run '^TestMEDIA001' -count=1 -timeout=180s` | 0; 7 top-level tests / 16 named subtests passed; package 1.621s | `raw/20-final-targeted-green.txt`; `de7369d370ec02febc53c3ac211963acaa2e4d4eb675bb1816440b95053d34c7` |
| `go test -race ./network -run '^TestMEDIA001' -count=1 -timeout=300s` | 0; same 23 test events passed; package 4.619s | `raw/22-final-race.txt`; `ed236b30de0011c244d8b76471cf7e895e160a5597e0a7b279ca7389cb587193` |
| `go test ./network -run '^$' -fuzz '^FuzzMEDIA001FileNode$' -fuzztime=30s -parallel=2` | 0; 3 seeds, 207,216 executions, 156 new interesting inputs | `raw/21-final-fuzz.txt`; `4c0f824b9e6d86bd1308c1b4716cee18d0fcd17ae5e28ff970aeab36b23bdea3` |

Earlier setup and implementation iterations are retained rather than hidden:

| Artifact | SHA-256 | Exact action | Exit / disposition |
| --- | --- | --- | --- |
| `raw/01-red.txt` | `1b156f1db503ef77c618724e81bb4e63cea548831625992faef233e4ea0d0419` | ticket targeted test | 1; invalid infrastructure red, pre-red `go.mod` update requirement |
| `raw/02-tidy-diff.txt` | `5fd1c9b3beb2646af7807b57dfa99113dcd2c124bffff803d5fbe849d14f2805` | offline `go mod tidy -diff` | 1; identified the missing exact archive |
| `raw/03-prerequisite-download.json` | `2375dea3888e691281640f0921f021dc38b10e7bfbbc3ff4337e068cb6406145` | exact-version `go mod download -json` | 1; sandbox network denial |
| `raw/04-prerequisite-download-escalated.json` | `224b38937865bfc1e7dc6cb9e8f50846f1b2fb3320969a48cc672614ecca173a` | same exact download with approved network | 0; sums verified |
| `raw/05-tidy-diff-local-cache.txt` | `ac743c2577d4102a415a94b85002c9087b3a5144e194d70f79d19a9ca4957259` | local-proxy `go mod tidy -diff` | 1; non-hidden cache was visible to module package scanning, then moved beneath a dot directory |
| `raw/06-tidy-diff-local-cache.txt` | `fbf266a3d04939b43127abfef041e425591db32b8d1f4648488dafbde7329b1a` | corrected local-proxy `go mod tidy -diff` | 1 because the requested diff was non-empty; versions-preserving diff retained |
| `raw/07-tidy.txt` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | corrected local-proxy `go mod tidy` | 0 |
| `raw/08-intended-red.txt` | `0ec9eead3cf9c542e5f6bc0417cfaaee37b5183a952147294e1c58a05f9e18d7` | targeted red draft | 1; discarded because a test-helper name collided with an existing package helper |
| `raw/09-intended-red.txt` | `bdbb1af6c724661e0fc9302c577dbe4652697745a82a99de8f48ca967e726142` | corrected targeted red | 1; accepted intended missing-production red |
| `raw/10-first-green-attempt.txt` | `ccd892cdc2fd16a14c4acd5300005c9c0718d8a139438d51aaa45603d1e470f7` | targeted implementation compile | 1; corrected Boxo chunk package alias |
| `raw/11-green-attempt.txt` | `cf8e23826c5f2f17c5a2c35087b96e305e0ac9815db42eae7566550340c7b2b5` | targeted test | 1; sandbox denied loopback sockets |
| `raw/12-green-attempt-loopback.txt` | `51569b1197e49deebb00bd3124d610bc6cd929090b2962b24e9705f299ff377d` | targeted test with approved loopback | 1; captured the independently computed golden CID and replaced the deliberate placeholder |
| `raw/13-green-attempt.txt` | `7a5f8fd40f852b0ea1fd2fd5636227e108c34ac8193870e201325f5a6fcfbf02` | targeted test | 0 |
| `raw/14-expanded-green-attempt.txt` | `7babeca8f926879abad7c49dad195adf4eb20ff36eb07ff583030afd3ea4e620` | expanded targeted test | 0 |
| `raw/15-race-attempt.txt` | `b121abcce6c1d1c2f3a67690cac77bbb578264e33468602e8b364ced0c5aea39` | expanded targeted race test | 0 |

The final rows above supersede the successful iteration captures. No failed setup or
iteration result is presented as a product failure or as final green evidence.

### Changed publication inputs

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/go.mod` | 138 | `df7f1e5d4fa1d20083c1fcc872415a4203f6afefb4a7b2a980d45cdd286b3c38` |
| `modern/go.sum` | 376 | `58614c1e27170525cf205f0f03c54085c70e4b37e51ee0aebf73a75ff955e800` |
| `modern/network/files.go` | 470 | `f088afa9f213a5b112b4443b49eb8bbb025b7436fc029d457b06e396bfab032d` |
| `modern/network/files_test.go` | 827 | `ca2ebcf7e779f57b8b62baea1abeb081080bfed7a29e2a386923015285898a46` |
| `modern/network/files_fuzz_test.go` | 50 | `1951689375fb002eb81d34acf3e5c9e8c1cbf55411a679683ccc83bd2a37944a` |

This report is also a changed publication input; its final identity is intentionally
left for the independent reviewer/executor because embedding its own hash would be
self-referential.

### Remaining limitations and handoff

This primitive stores and retrieves public bytes. It provides no encryption,
recipient authentication, pin/retention policy, garbage collection, HTTP serving,
media decoding, upload job, or UI. Private plaintext must not be passed to it. Import
failure can leave unreferenced blocks. Copy failure can leave a destination prefix,
which callers must discard. Cancellation interrupts owned storage/network work, while
arbitrary caller-provided Reader/Writer calls still require cooperative I/O as the
ticket specifies.

No broad package acceptance, vet, scanners, binary rebuild, Git staging/commit/push,
or daemon restart was performed. Those remain in the ticket's post-review Hermes phase.
Unrelated dirty work and retained evidence were preserved.

## Sol High correction — review 01 source-read errors

Date: 2026-09-18. Actor: Principal Dev, Codex Sol High. This correction addresses
review 01 only. The module files, fuzz source, other production/tests and binaries
remained frozen.

### Regression red

`TestMEDIA001ImportPreservesSourceErrors` was added before the production repair. It
exercises partial reads returning direct and wrapped `io.ErrUnexpectedEOF`, a complete
1 MiB chunk returned with a custom source error followed by ordinary EOF, and valid
partial/full-chunk ordinary EOF controls. Every error case requires `errors.Is` to find
the original source error and requires a zero `PublicFile`; the controls import and copy
their exact bytes.

From `modern`, with the same offline environment, Go 1.27.0 executable and disk-backed
developer01 caches recorded above, the exact regression command was:

```sh
go test ./network -run '^TestMEDIA001ImportPreservesSourceErrors$' -count=1 -timeout=180s
```

Exit: 1, expected regression red. All three failure cases returned a usable descriptor
and no error: two 7-byte files and one 1,048,576-byte file. Raw output:
`modern/dist/media001/developer02/raw/01-regression-red.txt`, SHA-256
`546acd3b0059e3aa093ede30c91f076985b852e1906ba690fa9a6e09b11be731`.
The loopback command used the runner's approved socket escalation and remained offline.

### Repair

`publicFileLimitReader` now retains the first source error that is not ordinary EOF.
It observes the source result before count, limit and empty-read handling, including
the max-byte probe. After Boxo layout returns, `ImportPublicFile` rejects completion
when a retained error exists. If layout also failed for another reason, `errors.Join`
preserves both identities. Direct/wrapped `io.ErrUnexpectedEOF` and a full-buffer
custom error therefore cannot be converted into successful completion by Boxo's
splitter or Go's `io.ReadFull`. Ordinary EOF remains successful completion. Existing
limits, cancellation, partial-block behavior and error identities are preserved.

### Corrected results

All commands ran from `modern` with the same offline environment and retained caches.

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| `go test ./network -run '^TestMEDIA001ImportPreservesSourceErrors$' -count=1 -timeout=180s` | 0; three error regressions and two EOF controls passed; package 0.018s | `developer02/raw/02-regression-green.txt`; `ae9b0f0efeebbd29969d1cb516a7c590fc1f0b6ef83a31e153813dc0d0938951` |
| `go test ./network -run '^TestMEDIA001' -count=1 -timeout=180s` | 0; 8 top-level tests and 21 named subtests passed; package 1.631s | `developer02/raw/03-targeted-green.txt`; `f5be19131d5754a6c93bd7e3b7ffc6e032e083818df1f7e2e026447020bb0f2c` |
| `go test -race ./network -run '^TestMEDIA001' -count=1 -timeout=300s` | 0; same 29 test events passed; package 4.646s | `developer02/raw/04-race-green.txt`; `9ad6e724295ba344a48c4caa6d0770ad2cd582ad0962f66f6066d1c5b0f058ea` |

The non-verbose package output does not independently enumerate individual test events;
the counts above come from the eight top-level functions, the previously reviewed
sixteen named subtests and the correction's five named subtests. The block validator
and fuzz source did not change, so the retained 207,216-execution fuzz result remains
applicable as review 01 specifies. The prior import-limit falsification also remains
applicable.

### Current source identities

| Path | Lines | SHA-256 | Disposition |
| --- | ---: | --- | --- |
| `modern/network/files.go` | 485 | `5a2f7a8515fa4578edbedf3ec9899791c5f8145c570aa9d568c232a10a727492` | corrected |
| `modern/network/files_test.go` | 886 | `ec224bd2b3cd22b00fbf512028f5464327d5d7f06dcc19835989208945b9c323` | corrected |
| `modern/network/files_fuzz_test.go` | 50 | `1951689375fb002eb81d34acf3e5c9e8c1cbf55411a679683ccc83bd2a37944a` | frozen |
| `modern/go.mod` | 138 | `df7f1e5d4fa1d20083c1fcc872415a4203f6afefb4a7b2a980d45cdd286b3c38` | frozen |
| `modern/go.sum` | 376 | `58614c1e27170525cf205f0f03c54085c70e4b37e51ee0aebf73a75ff955e800` | frozen |

## Review 02 — Hermes acceptance and publication

Per review 02 authorization, executed from `modern/dist/media001/acceptance04/`:

### Commands

| # | Command | Result |
|---|---|---|
| 1 | `go test ./... -count=1 -timeout=300s` | PASS (all 9 packages) |
| 2 | `go test -race ./... -count=1 -timeout=600s` | PASS |
| 3 | `go vet ./...` | 1 false positive (conditional context cleanup, stop() called on both branches) |
| 4 | `gosec -tests ./network/...` | 1 (G115 findings, test-only) |
| 5 | `go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$'` | PASS |
| 6 | `python3 scripts/govulncheck_policy.py source` | PASS (DHT exception within scope) |
| 7 | `go build -o bitbookd ./cmd/bitbookd` | PASS |

### Build identity

- Path: `modern/bitbookd`
- SHA-256: `b885b1d23fe3da0b7ec7de2b1817a4f9827ac45a32be1386bca3dfaa3af3ef6e`
- Size: 44,992,241 bytes

### Publication

Staged: 5 source/module files + report.
Secret scan: 0 leaks.
Whitespace check: clean.

#### Feature commit

```
0951c837 feat(network): public IPFS bootstrap, BitBook peer discovery, local-first block reads
```

Push: `e445e13c..0951c837  HEAD -> master`

#### CI

GitHub Actions `Go 1.27` run 35320581742: **success** (completed 07:42:35 UTC).
