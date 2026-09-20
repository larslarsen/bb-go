# BBGO-MEDIA-001 M1P — Sol High execution record

Date: 2026-09-19. Actor: Principal Dev, Codex Sol High. Scope: the active M1P
bounded public rich-content codec and its test/vector evidence only. Baseline HEAD was
`89c5fd2fd69c45da46f7e16ccb2642bdb130eb20`.

## Baseline and test-first red

The assigned existing inputs matched before editing:

| Path | SHA-256 |
| --- | --- |
| `modern/social/types.go` | `2e4c4efbb5488812447a865e79c4f2602d9275bc14bbba59d61a2fdcff3de3e8` |
| `modern/social/store.go` | `3f85e99f398b998aa91414de996557163b019bd7dba29de6f4b9bd5249cf8be2` |
| `modern/social/store_test.go` | `8a37afaaf411fc2d7ed2160e3fa21b3d111027643d26f5090b15458f583987c4` |
| `modern/go.mod` | `df7f1e5d4fa1d20083c1fcc872415a4203f6afefb4a7b2a980d45cdd286b3c38` |
| `modern/go.sum` | `58614c1e27170525cf205f0f03c54085c70e4b37e51ee0aebf73a75ff955e800` |

The test files and literal vector fixture were created before `content.go`. From
`modern`, the exact intended-red command was:

```sh
go test ./publiccontent -run '^TestMEDIA001M1P' -count=1 -timeout=120s
```

Exit: 1. The package failed to compile only because `Parse`, `Content`, `PlainText`
and `Marshal` did not exist. Raw output is
`modern/dist/media001/m1p-developer01/raw/01-targeted-red.txt`, SHA-256
`f38c1f7a143d573a354f8396459357ecb43a77df73d2a771586251d4f0fa4c67`.
The direct exit capture contains `1` plus a newline, SHA-256
`4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865`.

## Implementation

The new `publiccontent` package exports the typed `Content`, paragraph, inline node
and attachment descriptor data, along with `Parse`, `Marshal`, `PlainText`,
`ErrInvalidContent` and `ErrUnsupportedSchema`. It has no runtime caller in this
slice.

Parsing first bounds raw bytes, validates UTF-8 and escaped UTF-16 surrogate pairs,
then performs a streaming recursive preflight for the eight-container depth limit and
duplicate keys at every object depth. A second streaming pass enforces exact object
fields and node variants, required non-null arrays and values, array limits during
decoding, digit-only unsigned integer tokens, and a single complete JSON value.

Shared typed normalization enforces paragraph/node/scalar limits, text controls,
marks, links, attachment identity/reference rules, CID profile, media claims,
aggregate sizes, dimensions, durations and filenames. It builds a deep normalized
copy, so caller slices, duration pointers and input bytes do not alias returned data.
The plain projection counts Unicode scalars after inserting break and paragraph LFs.

Canonical encoding uses explicit wire structures for the frozen key order and node
variants. It emits compact JSON, explicit empty arrays, decimal integers and standard
Go HTML/U+2028/U+2029 escaping, and rejects canonical output above 65,536 bytes.
Descriptor order is preserved and repeated CIDs under distinct IDs remain distinct.

## Test and vector coverage

The final suite has seven top-level `TestMEDIA001M1P...` tests, table-driven subtests,
and one `FuzzMEDIA001M1PContent` target. The literal fixture has ten vectors: six
accepted and four rejected, including an independently supplied raw CID. Independent
canonical strings cover text, emoji and combining characters, paragraph/break
projection, ordered marks and a safe link, mixed attachment content, and HTML-escaped
hostile text.

The tests cover raw input and canonical output at 65,535/65,536/65,537 boundaries;
paragraphs, total inline nodes and projected Unicode scalars; attachment count,
per-file and aggregate lengths; dimension/product, duration, filename and link
limits; valid TAB and rejected text controls; malformed JSON, UTF-8 and surrogate
escapes; missing/null/wrong/unknown fields; exact variants; unsigned integer syntax
and overflow; depth and trailing values; unsupported schema; attachment reference,
ID, CID, media and filename failures; direct typed-value validation; deterministic
canonicalization, accepted key-order equivalence, reparse stability, non-mutation and
ownership.

The fuzz target seeds accepted and malformed values, rejects vacuous acceptance,
checks independent top-level/node/reference/projection bounds for every accepted
input, and proves Marshal/Parse equivalence and deterministic canonical bytes.

## Duplicate-key falsification

Against production SHA-256
`23e90da37d45658e35b34375d3670301188b5009cb24bc4e13a8af4d013aba1b`,
duplicate checks were temporarily removed from both the generic preflight and exact
semantic object readers. The compiled mutant SHA-256 was
`c8d8fd44de71ae154803f78140dd8302b157f416ebdd033dce3be3bee506d507`.
The retained diff is
`modern/dist/media001/m1p-developer01/raw/03-duplicate-mutant.diff`, SHA-256
`6c36ea8ef10bf62987ff1962fd48406f2c72cc098ddb6655e9870f6f3ebb90e3`.

The exact command was:

```sh
go test ./publiccontent -run '^TestMEDIA001M1PDuplicateKeys$' -count=1 -timeout=120s
```

The mutant built and exited 1: a duplicated schema received last-wins treatment and
returned `ErrUnsupportedSchema` rather than `ErrInvalidContent`. Raw output
`04-duplicate-falsification.txt` has SHA-256
`2ebba37a053b03454d44c1ce2d66fb2c127eca68333b815b934a78cd3432e931`.
The exact production bytes were restored and compared byte-for-byte; both pre- and
post-restoration hashes are the production hash above. The same test then exited 0.
Raw output `06-duplicate-restored-green.txt` has SHA-256
`05f2d5c5dd9ebed540bd9b33e351d9740efdb72795f1784e4fea7fe18a8b3657`.

## Controlled environment and final results

All commands ran from repository-relative cwd `modern` against the workspace's
ext4 disk-backed capture tree (statfs `ext2/ext3`, 4,096-byte blocks). The selected Go
binary reported `go version go1.27.0 linux/amd64` and SHA-256
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
The environment set `GOTOOLCHAIN=local`, `GOWORK=off`, `GOENV=off`, `GOPROXY=off`,
`GOSUMDB=off`, `GOFLAGS='-mod=readonly -p=2'`, `GOMAXPROCS=2`, the assigned
disk-backed developer01 caches, and
`modern/dist/media001/m1p-developer01/tmp` as `TMPDIR`. Exact commands, cwd,
timestamps, selected environment and direct exits are retained below
`modern/dist/media001/m1p-developer01/raw/`.

The exact final commands were:

```sh
go test ./publiccontent -run '^TestMEDIA001M1P' -count=1 -timeout=120s
go test ./publiccontent ./social -count=1 -timeout=120s
go test -race ./publiccontent -count=1 -timeout=120s
go test ./publiccontent -run '^$' -fuzz '^FuzzMEDIA001M1PContent$' -fuzztime=30s -parallel=2
```

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| targeted M1P | 0; package 0.012s | `12-final-targeted-green.txt`; `0e8b62a9d7f9c3e3714c366ac91b843da4681d74f5a8af2ec75e538dbf31fa67` |
| publiccontent and social | 0; packages 0.016s and 0.130s | `13-final-package-social.txt`; `7c3502cf78bab70c4aef7559d5bfc2571c07cd60b49c146940c3b7fd975890cd` |
| publiccontent race | 0; package 1.085s | `14-final-race.txt`; `6aa4953b9a41c1078b2ee212ee8561eba53bdd48ab5727e1707ed302538c434f` |
| content fuzz | 0; 762,543 executions, 126 total interesting inputs; package 31.038s | `15-final-fuzz.txt`; `0a69988cc30ed9aa94754fe3ef85994c5b2fadd301bd6ff6eb8e744093c0e796` |

The first combined package attempt was intentionally retained rather than overwritten.
`publiccontent` passed, while existing `social` tests could not bind
`127.0.0.1:0` in the restricted sandbox and exited 1. Its raw output is
`08-package-social.txt`, SHA-256
`77e70dd39837650830aea038cb88f5aa962b62bd4b3df111dd6ca41234ebdf89`.
The exact command was rerun outside that socket restriction with the same cwd,
arguments, toolchain, caches and offline environment, producing the final green result
above. Every final green `.exit` capture contains `0` plus a newline, SHA-256
`9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.

## Final identities

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/publiccontent/content.go` | 947 | `23e90da37d45658e35b34375d3670301188b5009cb24bc4e13a8af4d013aba1b` |
| `modern/publiccontent/content_test.go` | 603 | `b88b53256b632d009a34cc31830069fe0b1513b32e904e17f6ae4f454a675a97` |
| `modern/publiccontent/content_fuzz_test.go` | 88 | `93a071081995596ea33a1d9fea0942a2196543b82f5cfd13fc8a0cb7012f736c` |
| `modern/publiccontent/testdata/v1-vectors.json` | 72 | `8cd59f3b21e7674a38a9a6d98aa1f1500498aa5a18bea6da0534477204c0cdd8` |

## Limits of this execution

This slice only defines and verifies the unsigned, pure public-content codec. It does
not bind content to signed posts, retention, upload handles or runtime APIs; fetch or
decode media; integrate desktop composer/feed behavior; or define private/provider
media. No broad acceptance, vet, security scanner, dependency operation, build,
daemon/app restart, Git mutation or publication was performed.

## Review 17 correction — typed parity and preflight proofs

Date: 2026-09-19. The correction started from the four source/test/vector identities
recorded above and this report at SHA-256
`acf6311870bd004afca3ad75df8d7867018306a6e20c9836e145dbaebe194b4b`.
Tests and vectors were changed before production.

`TestMEDIA001M1PTypedValidationParity` independently constructs canonical bytes at
65,535, 65,536 and 65,537 bytes. Marshal and PlainText accept the first two and reject
the third, along with the existing 256-long-link document. A complete raw document
below the input cap uses literal ampersands whose required Go JSON escaping pushes its
canonical form over the output cap; Parse, Marshal and PlainText all reject it. The
test also fixes schema sentinel parity: empty and invalid-UTF-8 schemas are invalid,
while a nonempty valid alternate schema is unsupported on wire and typed paths.

`TestMEDIA001M1PTypedRejectionBounds` supplies preallocated 4,096-child and 1 MiB text
fixtures to both typed entry points and measures bytes allocated during rejection.
Each call must return `ErrInvalidContent` below a fixed 64 KiB ceiling. Separate
256-node and 280-scalar documents remain positive controls.

The exact pre-correction command was:

```sh
go test ./publiccontent -run '^TestMEDIA001M1P(TypedValidationParity|TypedRejectionBounds)$' -count=1 -timeout=120s
```

It exited 1. PlainText accepted the oversized document; Marshal and PlainText each
allocated 361,008 bytes for the oversized child slice and 1,048,720 bytes for the
oversized text. Raw output `m1p-developer02/raw/01-correction-red.txt` has SHA-256
`8d4b3c89122a116001c7feb7b4e2a1128da40973dbc63fec7d7ab71abeb3e788`.
The direct exit capture contains `1` plus a newline.

The shared validator now classifies schemas consistently, sums all child counts before
allocating normalized slices, and enforces the projection scalar budget incrementally
before copying text. A bounded, allocation-free canonical-size counter mirrors the
frozen encoding profile, including Go HTML and U+2028/U+2029 escaping. All three entry
points therefore enforce the same 65,536-byte canonical limit; Marshal still emits the
unchanged canonical representation for accepted content.

`TestMEDIA001M1PPreflightGuards` replaces exactly the text field in otherwise complete
documents. It covers invalid UTF-8, lone high and low surrogates, a valid escaped pair,
and a literal replacement character. It calls the internal streaming preflight with
depths 7, 8 and 9, proving that 7 and 8 pass while 9 fails independently of root
content shape, and also checks public rejection of those malformed root documents.
The final suite has ten top-level `TestMEDIA001M1P...` tests. The reusable fixture
now has 14 literal vectors: eight accepted and six rejected. The fuzz target has a
separately asserted valid attachment seed, so descriptor and
reference invariants start from known-positive attachment content.

### Review 17 falsifications

Corrected production SHA-256 is
`d8dd584a3c98b26afb34b1782dad84057c10f567473ea9f409648df50d1f73d7`.
Removing both raw UTF-8 and escaped-surrogate guards produced compiled mutant SHA-256
`fec71b7f67491674ab5642f701cba306f67711266c80e2d39abbc93ec1953a18`.
The exact preflight command exited 1 because invalid UTF-8 and both lone surrogates
were accepted. The retained diff `04-unicode-mutant.diff` has SHA-256
`585a49e8d674f8967246192b9b03c73a9d8592640fc97de4a28cb8f936f0c8d7`;
raw output `05-unicode-falsification.txt` has SHA-256
`2802dacea07d4f6ca90f625f133c5288c3f1c10ad80bcd0148a642bd346eaa39`.
Exact corrected bytes were restored and hash-checked, and the same command exited 0.

Separately relaxing only the maximum depth from 8 to 9 produced compiled mutant
SHA-256 `067464d06ded63ecc808c884424afce0be7fefd830e4150ba133767f381bad89`.
The same exact preflight command exited 1 because the internal boundary accepted depth
9. The retained diff `07-depth-mutant.diff` has SHA-256
`5d7713cfc32771704c006ff57bf5c9ec3382240d819f65b0760604b596888a2a`;
raw output `08-depth-falsification.txt` has SHA-256
`ee3b14bb2364b7aa91a836440140cdec6f3c23ee6ae42360907f0b8974f70540`.
Exact corrected bytes were again restored and hash-checked, and the command exited 0.
The Unicode and depth restored outputs are `06-unicode-restored-green.txt` and
`09-depth-restored-green.txt`, with SHA-256
`735bad1476bad9eef4ba2a0838a5c4c034bfee37cc40fed22e881942fca6c2f1` and
`05f2d5c5dd9ebed540bd9b33e351d9740efdb72795f1784e4fea7fe18a8b3657`.

### Review 17 final results

The correction reused the original pinned Go 1.27.0 binary, offline environment and
developer01 caches, with fresh disk-backed capture tree
`modern/dist/media001/m1p-developer02/` and its `tmp` child. Exact commands, cwd,
timestamps, outputs and direct exits are retained under its `raw` directory.

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| typed correction regression | 0; package 0.025s | `10-correction-green.txt`; `c64cac8e5985e10d9dd96e1ee1868aa681acd63dd4749bfea24d3b4ad40394b8` |
| `go test ./publiccontent -count=1 -timeout=120s` | 0; package 0.030s | `11-final-package.txt`; `cac2e0bfe955e1575efeea661188de774b6f864afb515addb770c0e428448564` |
| `go test -race ./publiccontent -count=1 -timeout=120s` | 0; package 1.116s | `12-final-race.txt`; `d455ae78e8116c2f524307113aa60d07bcbf785d3edd05f772e82c12338499ae` |
| 30-second content fuzz | 0; 79,026 executions, 144 total interesting inputs; package 31.033s | `13-final-fuzz.txt`; `37481a9cac83a685c488239d09ba11147c7bbab73e6d108a7fb757476ef845ac` |

All green exit captures contain `0` plus a newline. No social rerun, broad test,
scanner, dependency operation, build, runtime integration, restart, Git mutation or
publication was performed.

### Review 17 final identities

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/publiccontent/content.go` | 1,102 | `d8dd584a3c98b26afb34b1782dad84057c10f567473ea9f409648df50d1f73d7` |
| `modern/publiccontent/content_test.go` | 861 | `2b44a7bece5f0f38ffed04ac94273c57b454d6ad2c2c35eef3915de579115933` |
| `modern/publiccontent/content_fuzz_test.go` | 90 | `1cd41360ed7f2fdbb2fa0bab2995feed8908a2f739c1d9d352855e91049fa57b` |
| `modern/publiccontent/testdata/v1-vectors.json` | 100 | `c2e25ad2036cb795c9f4059d85c77ca3862e38eb7290607ac0a3a743be4d0636` |

## Review 18 — Hermes acceptance and publication

Date: 2026-09-19. Actor: Hermes, Jr Dev. M1P source accepted; executing acceptance
and publication per review 18 authorization.

### Source verification

All four M1P source pins verified. All social/module hashes from M1P match.

### Commands

| # | Command | Result |
|---|---|---|
| 1 | `go test ./... -count=1 -timeout=300s` | PASS |
| 2 | `go test -race ./... -count=1 -timeout=600s` | PASS |
| 3 | `go vet ./...` | PASS (clean) |
| 4 | `gosec -tests ./publiccontent/...` | PASS (0 issues) |
| 5 | `go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$'` | PASS |
| 6 | `python3 scripts/govulncheck_policy.py source` | PASS (DHT exception within scope) |

### Build

Not applicable — no executable consumes this isolated new package yet.

### Publication

Staged: 4 source files + EXECUTION-04.md.
Secret scan: 0 leaks.
Whitespace check: clean.
