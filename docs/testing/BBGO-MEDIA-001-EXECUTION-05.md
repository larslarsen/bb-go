# BBGO-MEDIA-001 M1B execution 05

Date: 2026-09-19/20 UTC
Actor: Principal Dev — Codex Sol (`gpt-5.6-sol`, High)
Baseline: `b774fea588e7b7b5d5455d5d386917fd85b2543a`
Scope: signed public posts and post-owned attachment retention

## Result

M1B is implemented in the authorized paths. Rich posts use strict canonical signed
payloads, durable private claim journals, retry-safe creation, fail-closed recovery,
dedicated deletion, mixed legacy/rich publication, and daemon recovery before local
readiness. The legacy constructor refuses to hide existing rich state. Remote fetch
verifies rich payloads without creating attachment claims or fetching media blocks.

No Git index, commit, push, binary build, broad test, vet, or scanner action was
performed. Unrelated dirty and untracked paths were preserved.

## Source inventory

Hashes are SHA-256 over the final bytes; line counts are physical lines.

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/attachment/clone.go` | 93 | `999b4d5dc557fb211f503fddd56c4e5ee0839f2235d83b40fbaad0bdf1283352` |
| `modern/attachment/clone_test.go` | 142 | `71874af5f07293a03e729b8b8a4c514a44e4d3c6424a04e2df110dc16690bbbf` |
| `modern/social/store.go` | 791 | `14b19d0fa0ced39c386eff2dbdebead6328b32e446fed02601688e7a73f114eb` |
| `modern/social/richpost.go` | 425 | `562ca498345a7c15fa602d050551871947c77234f660c54cdfaf65f08d42b9e7` |
| `modern/social/richpost_store.go` | 794 | `0426d253829df79fa04579d830a34e704c8a9fb2dcac64dce7947b480c029043` |
| `modern/social/richpost_test.go` | 810 | `9f881bdf6e55b3766ffb7725c0f5d118b8c6ba373e93b19eddbcecd1fe2ad95c` |
| `modern/social/richpost_fuzz_test.go` | 68 | `131d81f59b45e17737c2b6c68edbb93fb419cefe9fb6e9886d45e3d1e39242ce` |
| `modern/social/testdata/public-post-v1-vectors.json` | 42 | `a7bb6b8cfcf2d9cb154a44bc2232b6d69ff9868cc99cb09d20ae5ee23e06aea9` |
| `modern/cmd/bitbookd/main.go` | 299 | `08b58f6eb0fc18d42efaeb4b4d062c0677ec3f7279f2b5684db6eedb8af49b39` |
| `modern/cmd/bitbookd/richpost_test.go` | 203 | `f8f43b9c035c13f4049ab19dbcbb07cb0b94046799984ace85cb3d8452668132` |

This report is omitted from its own self-referential hash inventory.
`modern/attachment/store.go` and all frozen inputs remained unchanged.

## Public encoding and verification

The signed rich payload is a compact JSON object in this order:
`schema`, `id`, `vendorID`, `timestamp`, `slug`, `postType`, `status`, `content`.
It binds schema `bitbook.public-post/1`, the 32-character operation ID, signing peer,
canonical UTC timestamp, `rich-<id>` slug, `POST`, the M1P plain-text projection, and
the embedded canonical M1P content. `VerifyPost` first preserves the existing
libp2p key/signature behavior, then applies strict semantics to every object with a
top-level `schema` or `content`. It checks canonical bytes, duplicate/unknown/missing
fields, Unicode, depth and size bounds, identity/projection/slug bindings, and a
supplied envelope CID. Schema-less legacy signatures retain their previous behavior.

Literal vectors cover a legacy control, text rich post, attachment rich post, and a
correctly signed invalid-status post. They were produced by the independent retained
generator `modern/dist/media001/m1b-developer01/_tools/vector_gen.go.txt`, SHA-256
`d56fd3405f95924eeed002ee1411e0bc43aec8fd4b046dc09cbe014e0cd70d05`.
Accepted vectors are also checked directly with the libp2p signature primitive.

## Private journal format

Each operation is stored at
`/bitbook/social/rich-post/v1/record/<32-lowercase-hex-id>`.
Non-deleted records use this exact canonical field order and field set:

```text
version, state, id, requestDigest, envelope, claims
```

`version` is `1`. `state` is `preparing`, `live`, `aborted`, or `deleting`.
`envelope` is the frozen `SignedPost` with fields `post`, `signature`, `publicKey`,
and `hash`. Each ordered claim contains `attachmentId`, `uploadId`, `targetId`, `cid`,
and `byteLength`. Upload and target IDs remain private to this record. A deleted
tombstone has exactly `version`, `state`, `id`, and `requestDigest`; it retains no
envelope, key, signature, or claims.

The request digest is SHA-256 over:

```text
"bitbook-rich-post-request-v1\x00"
frame(id)
frame(canonical M1P content)
uint32_be(claim count)
for each descriptor in content order:
    frame(document-local attachment ID)
    frame(raw 16-byte upload reference ID)
```

Each `frame` is `uint32_be(length) || bytes`. The target IDs are intentionally absent
from the request binding so an aborted retry can reuse its already frozen private
targets. Decoding rejects noncanonical bytes, duplicates, unknown/missing fields,
oversize values, invalid keys/CIDs/references, key/record ID disagreement, invalid
envelopes, descriptor/claim mismatch, digest mismatch, and duplicate target ownership.

Limits include every record state. Creation checks the prospective encoded record
before its first journal write, so count/byte quota rejection neither creates claims
nor poisons the store.

## Lifecycle and recovery invariants

- `ClonePublic` holds the attachment mutation lock across source validation and the
  complete target retention transition. A matching ready target is idempotent even
  after source release; new targets require a matching ready local source and local
  DAG verification.
- Creation durably writes and syncs `preparing` before cloning each descriptor into
  a distinct private claim. It stores and flushes the immutable envelope before the
  journal is written and synced as `live`. Only that final sync makes the post visible.
- Errors after a journal write attempt poison that store instance. Post mutation,
  local post reads, commit, publish, and direct root publication then require reopen.
- Startup validates the entire bounded journal and globally unique target ownership
  before releasing any claim. `preparing` rolls back to `aborted`; `deleting` resumes
  releases and becomes a compact `deleted` tombstone. `live` verifies every claim and
  reconstructs only a missing envelope block from verified journal bytes.
- Live retry uses the canonical request digest before consulting temporary uploads,
  so it returns the byte-identical frozen envelope after upload release. Changed
  content/mappings conflict and a tombstoned ID cannot be reused.
- Deletion syncs `deleting`, releases only recorded private targets, then syncs the
  tombstone. Shared roots and upload claims remain independently owned.
- Mixed local feeds validate live claims, then sort by descending parsed timestamp;
  legacy order is stable on ties, legacy precedes rich on equal timestamps, and rich
  ties use operation ID. Invalid legacy timestamps sort last.
- Daemon startup opens attachments and completes rich social recovery before creating
  the local-client descriptor, HTTP service, discovery, or republisher.

The durability test datastore exercises both permitted results of a failed live
sync: retaining the unsynced `live` write reopens as visible with its claims, while
discarding it reopens the durable `preparing` predecessor, releases only its targets,
and records `aborted`. Preparing and deleting sync interruptions, duplicate ownership,
restart retry, shared-CID deletion, and corrupted daemon startup are also covered.

## Tests and captured execution

The final source contains 19 `TestMEDIA001M1B...` functions across attachment,
social, and daemon packages, plus two fuzz targets. The daemon child function is
also the linked subprocess entry point. Captures are under the local ignored tree
`modern/dist/media001/m1b-developer01/raw/`; commands, cwd, environment, UTC start and
finish times, direct exit status, full output, and output hashes were retained.

The toolchain was Go 1.27.0; executable SHA-256 was
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
The required offline environment and disk-backed caches/TMPDIR were used.

| Capture | Exit | UTC interval | Output SHA-256 | Result |
| --- | ---: | --- | --- | --- |
| `01-targeted-red` | 1 | `2026-09-20T01:34:55Z`–`01:34:56Z` | `79366d434c1e568de5ca1d651fc71b94e26e410087e739e33d730cd901a6f5eb` | Expected missing M1B APIs/types only |
| `21-targeted-unique-claims` | 0 | `03:00:15.129649174Z`–`03:00:18.062301611Z` | `26134c1128e81a79ff11b5c0f8e5e13e84b0862f41b46bf6d38bca5b9c1bbfb9` | All three focused packages passed |
| `22-focused-unique-claims` | 0 | `03:00:36.120295994Z`–`03:00:41.647464923Z` | `b1adc0a678a3095cc7fc00a91f8b2938cf13a37f8ff055b727cf0d4e755ab2df` | All seven compatibility packages passed |
| `23-race-unique-claims` | 0 | `03:01:47.088479721Z`–`03:02:07.657921973Z` | `088e5f600cee9fddaa0bb9c405b2ea340fbd4997d32935b67aa3b5c701c3a99f` | Race suite passed |
| `16-fuzz-richpost` | 0 | `02:52:12.724534696Z`–`02:52:46.325028673Z` | `08b5938dc6a3f33a23193ffdeb5e881d9756d14483a8de926ad16addce54f87e` | 224,409 executions; 54 total interesting inputs |
| `17-fuzz-record` | 0 | `02:53:15.403320745Z`–`02:53:48.182473489Z` | `4953d8533b6c657c62aa52bb0c2d2c808a16bbca337b3a7e74c1057db939a414` | 155,888 executions; 90 total interesting inputs |

Intermediate retained failures were actionable:

- `02-targeted-compile`: two unused imports plus sandbox loopback denial. Imports were
  fixed and subsequent socket-requiring runs used the granted execution permission.
- `03-targeted-go1`: the unavailable-source test called a network-capable copy and
  timed out. It was corrected to inspect the local blockstore directly.
- `06-targeted-expanded`: the new two-node fixture used DHT auto mode and failed to
  populate its routing table. Both controlled nodes now use server mode; capture
  `07-two-node` and all final suites pass.

## Required falsification

Semantic validation mutant:

- Original/restored `modern/social/store.go` SHA-256:
  `14b19d0fa0ced39c386eff2dbdebead6328b32e446fed02601688e7a73f114eb`.
- Mutant SHA-256: `2a8a4222390ac7d63cbbb9fb33c7a7f3a1345f34f36fda8411be430a989891fd`.
- The mutant removed only the post-signature `validateRichEnvelope` call.
- `09-validation-mutant` compiled and failed the permanent test because a signed
  forged status returned `nil` (exit 1, output SHA-256
  `adefb6d8db565eabfe5135c59eb3994998d0e8d1929627b5db2ca3b7f42fc93b`).
- Exact restoration passed in `10-validation-restored` (exit 0, output SHA-256
  `4232e35236e17674fa3107b961e7027d42e185afae15df529573bffa203ee7c8`).

Live-sync mutant:

- Original/restored `modern/social/richpost_store.go` SHA-256 at falsification time:
  `dd73d0e3280688091adfa980aa66ad6321af8997cb4871077ed5a846b531f514`.
- Mutant SHA-256: `274cf013abb1058c9a3b77cff685516960215f8f167bcf8d400187ebf2ac0b32`.
- The mutant skipped only datastore Sync when writing `live`.
- `11-livesync-mutant` compiled and failed because the injected live-sync failure
  incorrectly returned success (exit 1, output SHA-256
  `77a2a505559fde7b6c8dabc6693a00c32a4d722c43f9de03b923bfbe51531994`).
- Exact restoration passed in `12-livesync-restored` (exit 0, output SHA-256
  `da5b5ad1811baed244b80ab0cddceb8c233ccf8fca577e55f8df7c0b4de8208b`).

The retained source copies use `.go.txt` suffixes. Restoration hashes match their
pre-mutation originals exactly. A later source audit changed the final file hash to
`0426d253829df79fa04579d830a34e704c8a9fb2dcac64dce7947b480c029043` by rejecting
duplicate target IDs within one record as well as across records; captures 21–23 are
the green targeted, compatibility, and race evidence for those final bytes.

## Remaining acceptance work

No known focused M1B failure remains. The handoff reserves broad `go test ./...`,
broad race, `go vet`, gosec, the DHT exception test, vulnerability policy, daemon
build, staged secret scan, commit, push, and CI verification for the later Hermes
acceptance assignment. Those gates remain intentionally unexecuted here.

## Review 21 correction

Review 21 was corrected in the original M1B allowlist. This section supersedes the
earlier source inventory, journal field list, test count, and final-command rows for
the corrected bytes. Historical captures and descriptions above remain unchanged.

The correction closes the six reported behaviors:

- Rich-candidate classification now has an explicit error result, uses `UseNumber`,
  checks complete framing, and skips nested values with an iterative depth-bounded
  scanner. A token, conversion, or framing failure cannot select legacy verification.
- Every non-deleted record has a signed local claim certificate. Startup validates
  all envelopes, request bindings, certificates, global target uniqueness,
  target/upload disjointness, and actual descriptors before recovery cleanup.
- Legacy deletion protects exact stored rich slugs and CIDs. Ordinary historical
  `rich-` legacy slugs remain deletable, while automatic legacy slug allocation
  excludes every live or tombstoned rich identity.
- Any error after attempting the first preparing write poisons that Store instance
  and leaves durable recovery to reopen. The gate covers both post APIs, local reads,
  commit, publish, and direct root publication.
- Clone failures and daemon attachment/social startup failures retain wrapped causes
  for `errors.Is` while their printable boundary exposes only a category.
- Tests now cover the expanded durability and retention matrix, including a cold
  close/reopen of the actual on-disk node and datastore.

### Corrected private record and ownership certificate

The canonical non-deleted field order is:

```text
version, state, id, requestDigest, envelope, claims, claimSignature
```

`claimSignature` is required, nonempty base64 in JSON, and at most 1 KiB. The
deleted tombstone remains exactly `version`, `state`, `id`, `requestDigest`.
The certificate preimage is exactly:

```text
ASCII("bitbook.local-post-claims/1") || 0x00 || compact canonical JSON
```

The compact JSON has ordered keys `id`, `postCID`, `requestDigest`, `claims`.
Each claim has ordered keys `attachmentId`, `uploadId`, `targetId`, `cid`, and
`byteLength`. The existing node private key signs the preimage before the preparing
write. State is excluded so one certificate survives preparing, live, aborted, and
deleting. The certificate is absent from public blocks, manifests, API results, and
printable diagnostics.

The independent retained generator is
`modern/dist/media001/m1b-developer02/_tools/claim_vector.go.txt`, SHA-256
`129dad6fac5f481fc391131ca2c9034ec59f12b6c2653b49fd94e63662a0ca02`.
Its literal fixture freezes:

```text
post CID: QmdXVo8TAGL8cVo52MBk4oQzPTPYLh2HEiFYAVMbwt2Dsh
claim CID: bafkreibzsacz6kfnmwxztclqftvham5xusz2mh5eoxctnzjxqawpmwlw2a
signature: Nz50skU2sy/Wct4njhoTAd7QppQl7qEIlsywkRY53GCVuPR1rA9OuOe3KMpw93XZ+G3SSqG9q90hzg3jw/Z5AQ==
public key: [REDACTED-PUBLIC-KEY]
```

Bare `decodeRichRecord` validates bounded canonical syntax, the exact state-specific
field set, IDs, CIDs, lengths, and record shape. It cannot establish node identity,
verify signatures, recompute the request against public content, or inspect
attachments. `NewStoreWithAttachments` performs those contextual checks while
loading. It closes the complete query, validates the entire loaded set, then checks
every represented attachment, and only then begins cleanup. The record fuzzer proves
decoder stability and bounds; constructor corruption and ownership tests prove the
key-aware and attachment-aware checks.

The earlier `TestMEDIA001M1BAttachmentClaimsAndRestart` reconstructs stores over one
still-open in-memory node. `TestMEDIA001M1BColdRestartSharedMediaRetention` instead
closes and reopens the disk datastore, persistent network node, attachment Store,
and social Store. After releasing the sole upload claim it verifies exact media
bytes and recursive retention for two posts sharing a file, including two distinct
same-file claims in one post. One deletion preserves the other post; deletion of the
last post leaves no private claim after another cold reopen. Cached readable blocks
are not used as the retention oracle.

### Correction regressions and failure coverage

The pre-fix six-test command compiled and failed for the intended behaviors in
`01-correction-red`: clone error ID disclosure; three `1e9999` classification
bypasses; changed-target recovery acceptance; ordinary legacy deletion rejection;
all poison-gated APIs remaining usable; and daemon startup output containing a
private reference. Exit was 1 and output SHA-256 was
`2bc12b5d5857460a39e51c95dcbecf87ae036daa9d25628eb032f0b8e313d2a0`.
The final identical command is `21-correction-final`, exit 0.

Permanent coverage includes:

- direct, nested, reordered, and escaped classifier failures around `1e9999`, rich
  and legacy positive controls, malformed/trailing input, and size/depth cases;
- changed preparing/deleting targets for different and identical files, a sibling
  upload target, missing/changed certificates, duplicate ownership, target/upload
  overlap, descriptor mismatch, and an empty envelope CID before another recoverable
  record; valid preparing/deleting recovery is also exercised;
- journal Put and Sync failure for preparing, live, deleting, and deleted, with both
  retained and discarded ambiguous writes; envelope block Put and block Sync;
- ClonePublic retaining Put/Sync, ready Put/Sync, a forced pin-disappearance/Pin
  failure, pin Flush failure, last-source Release interleavings, and Clone/Close
  interleavings. Every goroutine is awaited with a deadline;
- failure on the second claim during cleanup followed by successful later recovery,
  and cancellation before journaling, after journaling, during loading, and during
  recovery release;
- private-ID redaction for missing, conflicting, and datastore-write clone failures
  and an actual linked daemon child that fails before readiness;
- record-count and aggregate-byte quota, raw/map preflight bounds, record decoder
  size, payload/framing bounds, and exact/above signature and key ceilings. A valid
  M1B record cannot reach the independent 96 KiB record ceiling because lower
  content, payload, key, signature, claim-count, and envelope limits apply. Canonical
  positive controls and the independent over-limit guard cover both sides without
  inventing a supposed valid-at-limit record.

The public vectors remain byte-identical. The rich fuzzer now calls actual
`VerifyPost` classification and has a signed `1e9999` seed. The record fuzzer has a
non-deleted live record with a claim/certificate field. Bare-codec fuzzing is not
claimed as proof of contextual constructor validation.

### Ownership falsification and restoration

The claim-certificate mutant bypassed only
`publicKey.Verify(preimage, record.ClaimSignature)`; public envelope signature
verification stayed active. At mutation time the original/restored source SHA-256
was `0183542093bded893e84033f03690e99ad8ddcd6c486da6edb9f64a2f323cab0`
and the mutant was
`c8790290694780fbfec9f89c0acba94c5648f8289efa005e2339666d7222d0e6`.
Retained copies use `.go.txt` suffixes.

`05-claimcert-mutant` compiled and failed because an unrelated same-file target,
sibling upload target, and changed certificate opened successfully (exit 1, output
SHA-256 `f1eeb57dbca7413658041a2bb580bdce74684ccd06d5e6972b356b2c53704098`).
Exact restoration passed in `06-claimcert-restored` (exit 0, output SHA-256
`78ce01abc9c1b1f5865beacd7e6f64fc4220fef9e56b2cc77ea1fd342799d1bd`).
Later authorized validation and query-order edits account for the final production
hash below; original and restoration identities matched before those edits.

### Final corrected source inventory

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| `modern/attachment/clone.go` | 105 | `75019eaf38a056085042b984bc4708c411e8a489738b3e05d1f8d1112ca918b2` |
| `modern/attachment/clone_test.go` | 452 | `a1a66526739c22410667b39e15e7a9d8c1c2ef61bebaf3aa5e349801e736941d` |
| `modern/social/store.go` | 811 | `ce445cf9d6e7b7262db22e2e444c3d985194b5df7fb0d323929cbbb48ed391b7` |
| `modern/social/richpost.go` | 452 | `d40f8050b30a87fca5dd9e0b7095d6be02e88cf86e01985a9d130daba88bbceb` |
| `modern/social/richpost_store.go` | 884 | `e3d21b252083402584aaa00b677f1d80659390e143d52389c1c3a784e1892562` |
| `modern/social/richpost_test.go` | 1739 | `1706fe78bd1f0764b6e1cc075149d904534540f481ee20a9638403669369d497` |
| `modern/social/richpost_fuzz_test.go` | 89 | `04dfd73ff62a46b12725663532595d154a36e05564ee21f89e8c076633dc6ed8` |
| `modern/social/testdata/public-post-v1-vectors.json` | 42 | `a7bb6b8cfcf2d9cb154a44bc2232b6d69ff9868cc99cb09d20ae5ee23e06aea9` |
| `modern/cmd/bitbookd/main.go` | 307 | `d54bc50a06238747c544f0b04085d893f516496d1e3657fad9a4f470b795d3ab` |
| `modern/cmd/bitbookd/richpost_test.go` | 246 | `6601695862d26675ff1dc20aaf91043ea0d3f21ffc567a113da1aac4d109ae3b` |

The corrected source has 33 `TestMEDIA001M1B...` functions, including the linked
daemon child entry point, plus two fuzz targets. The report remains omitted from its
own self-referential inventory. `modern/attachment/store.go`, modules, frozen M1P
files, old public vectors, and both binaries remain unchanged.

### Final corrected captures

Captures are under `modern/dist/media001/m1b-developer02/raw/`, using the same Go
1.27.0 executable, offline module settings, disk caches, two-package concurrency,
and fresh TMPDIR specified above.

| Capture | Exit | UTC interval | Output SHA-256 | Result |
| --- | ---: | --- | --- | --- |
| `21-correction-final` | 0 | `2026-09-20T04:13:19.282411069Z`–`04:13:21.940517185Z` | `6b4eda7034a14557447d062fbec3c56fd51cec9a2dec4c2560f84e91fea37c09` | Exact six Review 21 regressions passed |
| `22-final-targeted` | 0 | `04:13:55.642823192Z`–`04:13:57.950667719Z` | `a4024578e1c758623f3d3590ff2e2a6e0009d72d8b77a4f8cf215c93283b651c` | All M1B attachment/social/daemon tests passed |
| `23-final-focused` | 0 | `04:14:40.156050361Z`–`04:14:46.629997587Z` | `8bd59f0b93d99cf1a04de06f7cdfd9049992bdbec2888da2512d9122ba39422e` | Seven compatibility packages passed |
| `24-final-race` | 0 | `04:16:32.475971421Z`–`04:16:53.247542209Z` | `0a3b7ea7db441bcfb34ae5bb12b203f8ee8feb0011e987b0aab16679b6e8cdef` | Three race-enabled packages passed |
| `25-final-richpost-fuzz` | 0 | `04:17:07.278599404Z`–`04:17:41.156652019Z` | `d6c1aef61a723168be13dcee9e0e30bb9c5a082cf96b1c615868a278c039e324` | 125,643 executions; 181 total interesting inputs |
| `26-final-record-fuzz` | 0 | `04:17:50.965789664Z`–`04:18:23.289824851Z` | `c89fdfa855b85fa68f9ca2abed5b6aae93abda27c2d8f9ec569b24fa0c6f5349` | 153,900 executions; 127 total interesting inputs |

An earlier final-gate attempt, `14-correction-final`, was blocked only by sandbox
loopback denial and is retained with exit 1. The identical permitted run and every
subsequent final command passed. Intermediate `03` and `10` captures retain test
fixture mistakes corrected before these final runs.

HEAD remains `b774fea588e7b7b5d5455d5d386917fd85b2543a`; the index remains empty.
No broad suite, vet, scanner, build, Git mutation, publication, daemon restart, or
desktop write was performed. Those actions remain assigned to later acceptance.
