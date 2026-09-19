# BBGO-MEDIA-001 M2C — Sol High execution record

Date: 2026-09-18. Actor: Principal Dev, Codex Sol High. Scope: the active M2C
authenticated public upload and scoped byte-serving assignment only. Baseline HEAD
was `f9916348fca7820cb6e0a2bfb8a2357e5b79f8fb`.

## Baseline and test-first red

The assigned existing inputs matched before editing:

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/localclient/server.go` | 440 | `5b00cc6e10694a5fbaccda27637751b866cbc6bb1f0bfd98f05dc94479cc7a93` |
| `modern/localclient/server_test.go` | 749 | `655c7e18769fdd0f066e29636b11184c64b05ab83d29397b32399b018a6bdf6c` |
| `modern/cmd/bitbookd/main.go` | 287 | `fe2d3b2c07d3ddc0948889dc37158b90f5aecda79bd83d0974d1f61548f1caea` |
| `modern/cmd/bitbookd/localclient_test.go` | 195 | `221407662c049dd27e0a31c58952e194d52c8b13ea1e4015de2b129c7f619799` |

The three new test files were written before production. From `modern`, the exact
intended-red command was:

```sh
go test ./localclient ./cmd/bitbookd -run '^TestMEDIA001M2C' -count=1 -timeout=240s
```

Exit: 1. `localclient` failed only on the absent M2C constructor, media service,
authorization helper and route parser; the real daemon fixture reached the existing
listener and received 405 from the still payment-only route. Raw output is
`modern/dist/media001/m2c-developer01/raw/01-targeted-red.txt`, SHA-256
`d73924bd54b9892ff1dc3454e208ec2fa4d33456357c4b898584711cf6b213bc`.
Its exit capture contains `1` plus a newline, SHA-256
`4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865`.

## Implementation

`Start` retains its payment-only behavior. `StartWithMedia` validates its node, store
and peer identity before publication, then uses the same private Linux loopback
listener, run-scoped token, instance credential and connection descriptor. The common
handler authenticates loopback address, bound Host, absent Origin, one exact bearer
token and one exact instance value before media route parsing or state access.

The media service implements the frozen PUT, GET, DELETE, handle-creation, byte GET
and HEAD routes with canonical paths and stable JSON errors. Uploads require an exact
octet-stream content type and either a known length or chunked streaming, enforce the
100 MiB boundary through the real M2B importer, preserve first-write-wins retry
behavior, and map quota, not-ready, oversized and unavailable states without exposing
internal errors. Reference release invalidates its in-memory handles under the same
lock used for handle creation.

Handles are independent nonzero 128-bit random values, expire without renewal after
five minutes, and are capped at 128 live entries. Upload and byte-serving work share
two non-queuing slots and a ten-second operation context. Request cancellation and
server shutdown cancel work, close owned upload bodies to unblock readers, reject new
work and join admitted operations before closing the private `os.Root`. The server
does not close the caller-owned attachment store or network node.

Every byte response first copies the complete M2B descriptor through the real
`CopyPublicFile` verification path into an exclusive 0600 file below the private
root. The name is unlinked before any bytes are written. The copy has an independent
length bound and must pass exact written-length, sync, stat and seek checks before
headers. Whole files and one explicit closed range of at most 8 MiB are served with
attachment, no-store, sandbox, nosniff and same-origin headers. Invalid ranges and
conditional requests fail before staging.

The daemon opens M2B on its existing node with 4,096 references and a 4 GiB logical
limit before starting the media-enabled listener. Defer order stops the listener and
joins media work before closing the attachment store, then closes the node. Startup
recovery failures remain fatal; unsupported local access retains the existing
payment-only unavailability handling.

## Test coverage

The final source has nine top-level `TestMEDIA001M2C...` tests, extensive table-driven
subtests, and one fuzz target. Tests use real loopback listeners, real network nodes,
M2B stores and pins, plus the existing child-daemon fixture. Coverage includes empty,
chunk-boundary, multi-chunk, chunked and inert SVG round trips; same-listener payment
behavior; all trust-boundary denials on reads and writes; canonical-path attacks;
body, method, content and error contracts; known and chunked 100 MiB boundaries;
truncation, no-read retry and quota rejection; not-ready and missing references;
admission, cancellation and shutdown; 128/129 handles, expiry and random failures;
concurrent read/create/release behavior; staging creation, unlink, write and partial
copy failures; full, HEAD and bounded range responses; conditional-header rejection;
and late-block corruption before any file bytes are exposed.

The daemon test uploads, downloads, stops and restarts its isolated child, verifies
new credentials, rejects old credentials, a stale instance and the old handle,
reopens the durable reference, downloads it with a new handle, keeps payment records
usable, releases the reference, and checks bearer tokens and handles are absent from
captured logs. It does not invoke or restart the user's daemon.

## Corruption falsification

Against production SHA-256
`277608efd97255d3a38d0fe34a34efd81aa37acae207d333f19712ac610b7d23`,
the verified staging copy was temporarily replaced by a successful, length-matched
dummy-byte copy. The mutated SHA-256 was
`83462d98f1f24383eadb6c36ea38cab9ccac4924f5c75f334d8b72b65e3500de`.
The retained diff is
`modern/dist/media001/m2c-developer01/raw/02-corrupt-falsification.diff`, SHA-256
`151d5989f95f9bea0c43f5369e6472f485e462aae3b427cdc4099364358180df`.

The exact command was:

```sh
go test ./localclient -run '^TestMEDIA001M2CRejectsCorruptBeforeServing$' -count=1 -timeout=120s
```

The falsified source exited 1 as required: the regression observed HTTP 200 and dummy
bytes instead of the expected 503 JSON response. Raw output
`03-corrupt-falsification-red.txt` has SHA-256
`a25b2853674c2c006842a56e6bc014eee52fc3f9133d88e8ce32f8e2f9754a10`.
The exact production bytes were restored to their original SHA-256 and the same
command exited 0 in 0.022s. Raw output `04-corrupt-restored-green.txt` has SHA-256
`88eb97f2afe3d55e45daf0a0bb12c7027bf0f34e74c6240e54625d7b4c1f0738`.
The corresponding exit captures contain `1` and `0` plus newlines.

## Controlled environment and final results

All commands ran from `modern` on the workspace's ext4 disk-backed capture volume
(statfs `ext2/ext3`, 4,096-byte blocks). The selected cached Go executable reported
`go version go1.27.0 linux/amd64` and SHA-256
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
The environment set `GOTOOLCHAIN=local`, `GOWORK=off`, `GOENV=off`, `GOPROXY=off`,
`GOSUMDB=off`, `GOFLAGS='-mod=readonly -p=2'`, `GOMAXPROCS=2`, the accepted
disk-backed developer01 build/module caches, and
`modern/dist/media001/m2c-developer01/tmp` as `TMPDIR`. Exact context and commands are
retained in `raw/00-context.txt` and `raw/00-commands.txt`.

The exact final commands were:

```sh
go test ./localclient ./cmd/bitbookd -run '^TestMEDIA001M2C' -count=1 -timeout=240s
go test ./localclient ./cmd/bitbookd -count=1 -timeout=300s
go test -race ./localclient ./cmd/bitbookd -count=1 -timeout=600s
go test ./localclient -run '^$' -fuzz '^FuzzMEDIA001M2CRequest$' -fuzztime=30s -parallel=2
```

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| targeted M2C | 0; `localclient` 0.434s, daemon 0.088s | `05-targeted-green.txt`; `747fbbeccf9d17b9f10fddf4f8bd4ad229b36ff9c91b07025aa8ad7a27125319` |
| both affected packages | 0; `localclient` 1.242s, daemon 0.866s | `06-full-green.txt`; `ac4dce2b63feaeacc686126d2e17df65e90e36e54033872a5b7a0d3ae31fe31b` |
| race, both packages | 0; `localclient` 2.943s, daemon 13.708s | `07-race-green.txt`; `1b925452aa8546c6982bf364ce9014c00b26463e44b1dfd6983b52224bc665c9` |
| request fuzz | 0; 277,204 executions, 77 total interesting inputs; package 31.025s | `08-request-fuzz.txt`; `0b82add411508c8514067e2ca8390e79a5f4bdb92619474e26b760ac613362f4` |

Every green `.exit` capture contains `0` plus a newline, SHA-256
`9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.

## Final identities and frozen inputs

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/localclient/media.go` | 779 | `277608efd97255d3a38d0fe34a34efd81aa37acae207d333f19712ac610b7d23` |
| `modern/localclient/media_test.go` | 1,234 | `f2a9dcb0835ac0ef67c95d01878c1cbd4d3f0f1d4512c6993c85978e97d80b94` |
| `modern/localclient/media_fuzz_test.go` | 60 | `deb93d4a8ade5edab7319622787a0e222f3b108cbd7511d90aed261fad69dbe5` |
| `modern/localclient/server.go` | 501 | `64b7a5af02737fa66ae3f28fa216aa154e5be9af14e09fce9fbf61e88ebd4495` |
| `modern/cmd/bitbookd/main.go` | 296 | `5d8c643942be5153942a00aee0e862a6835970649cb080ec380ca31542275b74` |
| `modern/cmd/bitbookd/media_test.go` | 204 | `09cbd9bfa77fc07998ad34b201075166ecd0064c7ea7368408a56d25cc885743` |

All frozen M2B and M2A/module pins still match:

| Path | SHA-256 |
| --- | --- |
| `modern/attachment/types.go` | `58d94fd115f2fb35ab2f862cad9f359fae439bc41ca97273095ee0160c26604f` |
| `modern/attachment/codec.go` | `97dd8ef6a4e79b69617df6355301487a979a13bd79a4e1d843dc95007b99c482` |
| `modern/attachment/store.go` | `2898c1e48800fb860b69db720b9d04755c80191d23199ba6bb6f9dea6047daa2` |
| `modern/attachment/store_test.go` | `2bcf30f26a845f7beb67b3b25a5034799b75fd05d83e92814c43be8f9a58ac68` |
| `modern/attachment/store_fuzz_test.go` | `1c593c5f1e67b08d8f364cb284252d678f83da1722d68e03aed083f652cb521a` |
| `modern/network/files.go` | `5a2f7a8515fa4578edbedf3ec9899791c5f8145c570aa9d568c232a10a727492` |
| `modern/network/files_test.go` | `ec2df5adaebe5a0793e0020404158c876e9b2642b1a8a6cb0d56259bccee96c2` |
| `modern/network/files_fuzz_test.go` | `1951689375fb002eb81d34acf3e5c9e8c1cbf55411a679683ccc83bd2a37944a` |
| `modern/network/node.go` | `ee15f7a120468679a7f52a8e0fa73aa38aa86813ea5a62a493bfd990daf13555` |
| `modern/network/open.go` | `96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b` |
| `modern/go.mod` | `df7f1e5d4fa1d20083c1fcc872415a4203f6afefb4a7b2a980d45cdd286b3c38` |
| `modern/go.sum` | `58614c1e27170525cf205f0f03c54085c70e4b37e51ee0aebf73a75ff955e800` |

## Limits of this execution

This slice provides authenticated transport for explicitly public bytes. It does not
activate inline rendering, safe media decoding, metadata stripping, thumbnails,
transcoding, MIME acceptance, remote URL/CID import, private media, renderer token
exposure, social schemas, block GC or a public gateway. No broad acceptance, vet,
security scanner, dependency install/tidy, daemon build, user-daemon restart, Git
mutation or publication was performed. Those remain outside Sol's M2C authority.

## Review 12 correction — Sol High

Date: 2026-09-19. The correction started from the six source/test identities recorded
above and report SHA-256
`88805ab71f6be88b71607a39ec8cc49133b98ae1e03ef5bdede5070fe73e11bb`.
Tests were changed before production. The required focused command was:

```sh
go test ./localclient -run '^TestMEDIA001M2CRealHTTP(Shutdown|TruncatedUpload)$' -count=1 -timeout=120s
```

Against the submitted production hashes it exited 1. The real stalled-body test held
shutdown beyond its 250 ms context and 750 ms observation bound. A cancellation after
successful staging still committed HTTP 200 and file bytes, and a blocked response
write ignored request cancellation. Both a deterministic `io.ErrUnexpectedEOF`
reader and a raw loopback HTTP request that half-closed after two of three declared
bytes returned 503/UNAVAILABLE instead of 400/BAD_REQUEST. The client connection and
blocked writer were explicitly released after each expected failure, so the red left
no handler or goroutine blocked. Raw output `m2c-developer02/raw/01-correction-red.txt`
has SHA-256
`82c7fb19a91746be3161a7599048c7c7a633f2c78177516a7fbceb7d0cd545cf`;
the direct exit capture contains `1` plus a newline.

Upload bodies are now wrapped in concurrency-safe, idempotent ownership. Each admitted
upload installs a tracked context callback that first sets the real HTTP transport's
read deadline, then closes the owned body. The handler stops or joins that callback
before returning. Media shutdown only marks admission closed and cancels media work;
it no longer risks blocking before `http.Server.Shutdown` can honor its context and
reach the existing force-close path. All admitted handlers and their callbacks still
finish before the private root or caller-owned store can be torn down.

Byte delivery installs the corresponding tracked write-deadline callback. It checks
the operation context after verified staging and again immediately before success
headers, then uses a bounded, context-aware copy loop. Cancellation therefore prevents
post-stage success and interrupts a blocked response write. Positive upload and byte
delivery controls remain successful, while shutdown preserves a completed unrelated
reference and never makes the stalled partial reference ready.

The exact-length reader maps `io.ErrUnexpectedEOF` to the existing client-length
sentinel only while declared bytes remain. Exact completion is normalized to EOF;
timeouts, cancellation and unrelated storage or network errors retain their existing
503 or no-response behavior. Both the deterministic and raw half-close regressions now
return 400/BAD_REQUEST and leave no ready partial reference.

The fuzz target now calls the production authorization, route and range parsers. Its
independent oracles cover exact loopback/Host credentials, absent, empty-present and
duplicate bearer, instance and Origin headers; valid reference and handle routes;
canonical-ID and path rejection; no range, zero-length files, HEAD, duplicate headers,
closed and open ranges, suffix and multiple forms, numeric overflow, and the exact
8 MiB boundary. Valid ranges must also satisfy independent endpoint and length
invariants. Expected authorization denials pass through real dispatch with an
instrumented body and handle state so a read or state change fails the target.

The focused command then exited 0 in 0.314s. Raw output
`02-correction-green.txt` has SHA-256
`4c72b11cfea571cef2d76e14ceedf000b9f1e3c128e08f5c6d8c2d9089204724`.
The unchanged required final commands produced:

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| M2C targeted | 0; `localclient` 0.821s, daemon 0.092s | `03-targeted-green.txt`; `f6a3cb3a49c41666f5af34c269702d3d4cd7a419ed701db95278f8fa7cd99958` |
| both affected packages | 0; `localclient` 1.643s, daemon 0.967s | `04-full-green.txt`; `554979d0b4e003c52c9f5836f423b162c1314dfb3daffc40864fd8c119b5e19a` |
| race, both packages | 0; `localclient` 3.353s, daemon 13.680s | `05-race-green.txt`; `976c909dd7271d78a6d97a7ece343764c2ad594912428ea73c356bde8a51e947` |
| strengthened request fuzz | 0; 195,714 executions, 118 total interesting inputs; package 31.022s | `06-request-fuzz.txt`; `8e0da3493d64f9abda2b24fb471ac7f8eafda4935daa328192af3bcd7f0c63c7` |

Every correction green `.exit` capture contains `0` plus a newline, SHA-256
`9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.
Commands used the same pinned offline Go 1.27.0 executable and developer01 caches as
the initial execution, with the fresh disk-backed
`modern/dist/media001/m2c-developer02/tmp` directory as `TMPDIR`. The inspected volume
reported ext4 through `df`, `ext2/ext3` through statfs and 4,096-byte blocks. Exact
environment and source identities are retained in `raw/00-context.txt` and
`raw/07-final-source-identities.txt`, SHA-256
`306c52e0689c5bf7c1ae518b7fdaf7960068deb23788cba4fa20b47dc3cc15c3` and
`188beb758626f05e6d052a15c75ece7753768e16df5a4f396f95a0112e03a5d4`.
The exact command list is `raw/00-commands.txt`, SHA-256
`178ec5a327e223fe01ee9be59d836a61ce81c3ec4148c93901bc417c9c411aa9`.

Final correction identities are:

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/localclient/media.go` | 847 | `a5c4a77bf64af927883f4865aca09f623071c8b898afacca0e45c3473b352541` |
| `modern/localclient/media_test.go` | 1,503 | `458f7f6d9e5175ef4688d24d008d14f30dc89b5a6b2faa9198a65a0258a6a0c1` |
| `modern/localclient/media_fuzz_test.go` | 225 | `6ad0e2585aa1a433bb9d279a8aa3ac0e2b4e0c860fc18889e3c43231f927ecb8` |
| `modern/localclient/server.go` (unchanged in correction) | 501 | `64b7a5af02737fa66ae3f28fa216aa154e5be9af14e09fce9fbf61e88ebd4495` |
| `modern/cmd/bitbookd/main.go` (frozen) | 296 | `5d8c643942be5153942a00aee0e862a6835970649cb080ec380ca31542275b74` |
| `modern/cmd/bitbookd/media_test.go` (frozen) | 204 | `09cbd9bfa77fc07998ad34b201075166ecd0064c7ea7368408a56d25cc885743` |

All five M2B pins, seven M2A/module pins, frozen daemon files and the existing retained
corruption evidence remain unchanged. Formatting and scoped whitespace checks are
clean. No broad-module acceptance, scanner, dependency fetch, build, daemon restart,
Git operation or publication was performed.

## Review 13 test-fixture repair — Sol High

Date: 2026-09-19. This test-only repair started from
`modern/localclient/media_fuzz_test.go` SHA-256
`6ad0e2585aa1a433bb9d279a8aa3ac0e2b4e0c860fc18889e3c43231f927ecb8`
and report SHA-256
`4a3bf15685d799bed13c847493239cff82fb18bd75e69b07942a69c75a6374c1`.
All production, lifecycle tests, daemon tests, modules and binaries were frozen.

The prior correction's route and range fuzz results remain valid, but its credential
fixture stored `X-BitBook-Instance` by direct assignment under a noncanonical map key.
`http.Header.Values` canonicalized its lookup to `X-Bitbook-Instance`, so intended
valid fuzz candidates appeared to have no instance credential and returned 401. The
oracle observed the same malformed map and therefore did not reveal the construction
error. The earlier 195,714-execution fuzz result did not prove authenticated status
zero or distinguish empty and duplicate instance forms from a missing instance.

`TestMEDIA001M2CFuzzAuthFixture` was added before changing the helper. It constructs
all headers through that helper and uses fixed expected statuses and value counts for
a valid request; missing, empty-present and duplicate bearer credentials; missing,
empty-present and duplicate instance credentials; and empty-present and duplicate
Origin. The positive case requires literal status zero and exactly one retrievable
`instance` value rather than consulting the fuzz oracle.

The exact focused command was:

```sh
go test ./localclient -run '^TestMEDIA001M2CFuzzAuthFixture$' -count=1 -timeout=120s
```

Against the old helper it exited 1 as intended. The valid request returned 401,
`Header.Values` retrieved zero instance values, and every case expecting a present
instance reported the same missing value. Raw output
`modern/dist/media001/m2c-developer03/raw/01-auth-fixture-red.txt` has SHA-256
`56ae3e24e1caed124fa238fea1b13155da7fc460a601e3d4014cd9d5b68d9242`;
its direct exit capture contains `1` plus a newline.

The helper now canonicalizes the key in every present-header branch. Normal values use
`Header.Set`; empty-present and duplicate values are assigned below the canonical key.
Duplicate Authorization values are each `Bearer token`, and duplicate instance values
are each `instance`, so their 401 result proves duplicate cardinality rather than a
bad individual credential. The valid seed remains in the fuzz corpus and the fixed
positive regression permanently proves it authenticates.

The focused command then exited 0 in 0.006s. The authorized final commands produced:

| Command | Exit/result | Raw output and SHA-256 |
| --- | --- | --- |
| focused auth fixture | 0; package 0.006s | `02-auth-fixture-green.txt`; `78e01ce5abcfb7a139cd69643b0750fb2840cb49bb0aae200432a49309f3ee4d` |
| complete fuzz seed corpus | 0; package 0.006s | `03-fuzz-seeds-green.txt`; `78e01ce5abcfb7a139cd69643b0750fb2840cb49bb0aae200432a49309f3ee4d` |
| bounded request fuzz | 0; 186,420 executions, 156 total interesting inputs; package 30.078s | `04-request-fuzz.txt`; `fbe4570c5c3b59c938da7cb1ee305bed2442f45a800f10ce2790ca45ab5c8253` |

Every green exit capture contains `0` plus a newline, SHA-256
`9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.
Commands used the unchanged pinned offline Go 1.27.0 executable and developer01
caches, with the fresh disk-backed `modern/dist/media001/m2c-developer03/tmp`
directory as `TMPDIR`. The volume reported ext4 through `df`, `ext2/ext3` through
statfs and 4,096-byte blocks. Exact context, command and identity captures are
`raw/00-context.txt`, `raw/00-commands.txt` and `raw/05-final-identities.txt`, SHA-256
`3347fced5b63e45b326ec26ded42a2caae7b54a2881cbede2bca3cf90558bcda`,
`23db70eb8800675e137b57d077d64f93346e34228abd20db608274e72393554a` and
`c57c08844cb5fe0f0458f4b67f7857baa6813fcaf758838522297655381e4143`.

Final test-fixture identity:

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/localclient/media_fuzz_test.go` | 286 | `047b7b084d28db9aaa189ce329828d0bf7737a93706ae24223cf350843ddff30` |

The frozen production and lifecycle-test identities remain
`a5c4a77bf64af927883f4865aca09f623071c8b898afacca0e45c3473b352541`
for `media.go`,
`458f7f6d9e5175ef4688d24d008d14f30dc89b5a6b2faa9198a65a0258a6a0c1`
for `media_test.go`,
`64b7a5af02737fa66ae3f28fa216aa154e5be9af14e09fce9fbf61e88ebd4495`
for `server.go`,
`5d8c643942be5153942a00aee0e862a6835970649cb080ec380ca31542275b74`
for daemon `main.go`, and
`09cbd9bfa77fc07998ad34b201075166ecd0064c7ea7368408a56d25cc885743`
for daemon `media_test.go`. Prior package/race, real-socket and corruption evidence was
reused as directed. No production redesign, scanner, broad acceptance, dependency
operation, build, daemon restart, Git operation or publication was performed.
