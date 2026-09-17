# BBGO-ACC-002 — durable account grants and revocations

Reviewer: Codex, 2026-09-17, High. **Active: Sol test-source authoring only.**
Actor: Codex Sol, `gpt-5.6-sol`, High, owner-relayed. This ticket is the sole handoff.
Read AGENTS.md, TESTING.md and [CURRENT_TASK](../docs/handoff/CURRENT_TASK.md).
Do not run tests or author production until their phases are activated here.

## Result and scope

Persist the accepted [ACC-001 verifier](BBGO-ACC-001.md) so an acknowledged device
revocation remains effective after restart. Persist its capacity-denial state too.
Deliver one isolated `modern/accountstore` package over the existing datastore
dependency, with real LevelDB restart/crash tests. No new dependency or changes to
the accepted verifier. This is the next prerequisite for portable accounts, not
activation of portable accounts in the daemon or UI.

The [accepted account direction](../../bb-desktop/docs/architecture/BB-ACCOUNT-RECIPIENT-PROPOSAL-01.md)
still governs: controller/device separation, permanent signed revocations, separate
capabilities, unchanged v1 meanings. Naming stays provisional and is not a gate.
Key custody, pairing, network discovery/synchronization, fresh-authority admission,
wallets and UI are outside this ticket. The next network contract must merge signed
records through this store; a peer must never replace its local snapshot wholesale.

### Storage choice and limits

Use one bounded, atomically replaced value per account, followed by datastore Sync.
Do not use a multi-key Batch as if it were a transaction. The pinned go-datastore
v0.9.2 interface explicitly gives Sync its crash-durability contract and says Batches
are not transactions. In pinned go-ds-leveldb v0.5.3, Put uses synchronous writes
and Sync is a no-op because Put already provides that guarantee. Reviewed local
module source: `datastore.go` in both modules, plus `modern/network/open.go`.
The attempted remote source fetch was unavailable; this finding uses the locally
cached versions pinned by go.mod/go.sum, not an unverified current upstream version.

The backend must atomically replace each value and honor Sync; production storage
must be disk-backed. The caller owns and closes the backend and must have exactly
one live Store owner per account key. Close its Store handles before closing the
backend. Do not copy Store values or operate separate
writable handles for the same account concurrently. Methods on one Store are safe
concurrently. No process-wide registry or background goroutines.

This guarantees acknowledged persistence under the backend contract. It cannot
detect replacement of the entire database with an older valid backup or discover
unseen remote revocations. The existing LevelDB adapter can call RecoverFile when
opening a corrupt database: later daemon integration must not silently recover or
recreate authority state after corruption. This isolated package receives an already
opened backend and never calls recovery, deletes evidence or repairs corrupt data.

## Baseline and exact paths

Daemon HEAD: `d24e68f68c71d18ff6dcf85ae7e1d36f438b3a3f`. All six target paths below
are absent. Preserve all unrelated dirty/untracked work, including cancelled DEV-001.
These inputs are frozen through every phase:

| Input | SHA-256 |
| --- | --- |
| modern/accountauth/types.go | ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb |
| modern/accountauth/records.go | cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640 |
| modern/accountauth/state.go | b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1 |
| modern/accountauth/records_test.go | 264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9 |
| modern/accountauth/state_test.go | 949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed |
| modern/accountauth/fuzz_test.go | 08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833 |
| modern/go.mod | 1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783 |
| modern/go.sum | 4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a |
| modern/network/open.go | 96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b |

**Sol writable now:** `modern/accountstore/store_test.go`, `persistence_test.go`,
`fuzz_test.go`. Helpers stay in those files. Tests exercise the public contract;
do not copy the verifier or add production stubs. Synthetic reproducible keys only.
Gofmt of these three test files is allowed; no compiler/test/fuzz/scanner execution,
dependency changes, records, Git or actor launch. Stop with the ticket/source pointer.

**Future production scope, inactive:** `modern/accountstore/types.go`, `store.go`,
`codec.go`. No imports of accountstore elsewhere, runtime wiring or existing source
edits. Sol authors those files only after reviewed tests and expected-red acceptance.

## Frozen public contract

Use the existing accountauth types; do not duplicate them:

```go
const MaxStoredRecords = accountauth.MaxKnownRecords + 1 // 4097
const MaxSnapshotBytes = 74 + MaxStoredRecords*(2+accountauth.MaxRecordBytes) // 966966

var ErrExists error      // Create found an existing value, even if corrupt
var ErrCorrupt error     // stored bytes cannot establish the pinned account state
var ErrUnavailable error // nil/zero/closed or disabled after a storage error
var ErrSaturated error   // persisted saturation; authorization denied

type Store struct { /* private fields; must not be copied */ }
func Create(ctx context.Context, backend datastore.Datastore, controller [32]byte) (*Store, error)
func Open(ctx context.Context, backend datastore.Datastore, controller [32]byte) (*Store, error)
func (s *Store) Apply(ctx context.Context, raw []byte) error
func (s *Store) AuthorizesKnown(grant accountauth.GrantID, device [32]byte, required accountauth.Capability) (bool, error)
func (s *Store) Saturated() (bool, error)
func (s *Store) Records() ([][]byte, error)
func (s *Store) Close() error
```

Sentinels must work with errors.Is. Additional errors may describe invalid input and
wrap backend/context errors; do not expose signing material or raw records in errors.
Reject a nil backend interface and zero controller before storage access. Callers
must supply a non-nil usable backend and context, not a typed-nil backend pointer.

- Create is an explicit local initialization operation. Get the derived key first;
  only datastore.ErrNotFound permits writing a valid empty snapshot. Existing data
  returns ErrExists without modification. Other Get errors abort. Put and Sync the
  empty snapshot before returning a usable handle. No create-on-open behavior.
- Open reads the derived key without writes. Missing data returns an error matching
  datastore.ErrNotFound. Corrupt, foreign, unsupported or truncated snapshots return
  ErrCorrupt and no handle. Other backend errors propagate. Reverify every signature
  and reconstruct KnownState; never trust cached booleans or caller claims.
- A nil/zero/closed/disabled handle rejects Apply and returns false/error or nil/error
  from queries, matching ErrUnavailable. Close is nil-safe/idempotent and disables
  this handle only; it does not close the shared backend or flush deferred writes.
- Reads use the committed in-memory state with no storage I/O. Records returns deep
  copies of original signed bytes in their stored order, including an overflow
  witness if saturated. Input or returned-slice mutation cannot alter authority.
- Saturated is a valid durable state: it returns true,nil; AuthorizesKnown returns
  false,nil for every request. It is different from unavailable storage.

## Frozen local snapshot format

Key: `/bitbook/accountauth/v1/` followed by the 64 lowercase hexadecimal characters
of accountauth.AccountIDFor(controller). There are no caller-chosen path fragments.
This private local format is not a new network protocol or a signed state claim.

| Bytes | Meaning |
| --- | --- |
| 0..7 | ASCII `BBACST01` (eight bytes) |
| 8..39 | pinned controller public key, 32 bytes |
| 40..41 | record count, unsigned 16-bit big endian |
| next | repeated: unsigned 16-bit big-endian length, then original signed record |
| last 32 | SHA-256 of all preceding snapshot bytes |

Empty snapshot: 74 bytes. Count range 0..4097. Each length must be exactly 130 or
234 and its record must pass ACC-001 verification for this exact controller. Reject
unknown magic, wrong controller, invalid lengths/counts, truncation, trailing bytes,
bad checksum, duplicate semantic facts or size above MaxSnapshotBytes. Bound total
length/count before allocating a count-sized collection or verifying signatures.
The checksum detects accidental corruption; it is not authentication or rollback
protection. Recomputed checksums cannot make invalid signatures/schema acceptable.

Fact identity is `(kind, grant ID)` for Grant/RevokeGrant (distinct kinds), or
`(RevokeDevice, device key)`. Signatures are not part of duplicate identity. Retain
first accepted bytes, all revoked grants and all tombstones; never compact/prune
them here. Stored order is arrival order, not a clock-based precedence rule.

Counts through 4096 reconstruct an unsaturated KnownState. At count 4097 the first
4096 facts must replay successfully; the last must be a novel verified same-account
fact whose Apply produces the verifier's permanent saturation. Retain that last
record as an **overflow witness**, not an authorized extra grant. This permits full
reverification of saturation without modifying accountauth or trusting a stored flag.
No larger count is valid and no reset/delete/snapshot-replacement API is exposed.

## Apply, persistence and failure semantics

Serialize each Store's mutation and reads with one lock. Fully verify input and the
exact controller before any duplicate/capacity handling. Invalid or foreign input
returns an error, makes no storage calls that change data, and preserves prior
authority. If the context is already canceled before starting storage work, return
its error without changing state. Do not claim cancellation means rollback once a
write has started.

For an unsaturated handle, a verified duplicate is a successful no-op with no writes.
A novel fact appends to a new bounded snapshot. Put the entire value, then Sync that
exact key; only after both succeed may memory/authorization advance and Apply return
nil. Do not mutate KnownState optimistically or shallow-copy it: its maps are private
mutable state. Validate first, persist, then apply the verified record to live state.
An unexpected post-persistence invariant failure disables the handle.

The first novel valid fact beyond 4096 follows the same Put/Sync path, stores the
4097th witness, makes the handle saturated and returns ErrSaturated. Thus this error
means denial was durably recorded. A storage failure must instead return an error
matching ErrUnavailable, not ErrSaturated. Once saturated, validate input but reject
all valid further Apply calls with ErrSaturated and no write (including duplicates).

On Put or Sync error, return an error matching ErrUnavailable and permanently disable
the current handle: all queries must deny with ErrUnavailable, and no retry on that
handle can resurrect old authority. Do not write an older snapshot to roll back.
The failed operation has an uncertain durable outcome; it must not be acknowledged.
After explicitly reopening, storage may contain the complete old or new snapshot;
invalid bytes still cause ErrCorrupt. A failed, unacknowledged revocation cannot be
promised durable. No in-memory mechanism can recover a write the disk did not retain.
The later service must surface this failure and reconcile authority before admission.

Successful Sync is the acknowledgment boundary, including Create. Abrupt termination
after that boundary must retain the complete snapshot on the real supported backend.
Store.Close, process shutdown and a later successful write are not required to make
an earlier acknowledgment durable. Never repair an error by opening an empty account.

## Required test source

Use small independent signing/snapshot helpers with fixed literal domains and offsets;
reuse standard Ed25519/SHA-256, not a production encoder as the expected-byte oracle.
ACC-001 remains the cryptographic verifier; do not reproduce its full suite here.

- Create/Open distinction: fresh creation persisted before success; duplicate Create
  never overwrites; missing/corrupt Open never creates or repairs; wrong-account
  snapshot copied to a different account key fails. Unrelated datastore keys survive.
- Grant success before denial: `TestRevocationSurvivesReopen` uses a real disk-backed
  go-ds-leveldb store. Grant initially authorizes and survives reopen; device revoke
  then denies after closing/reopening both Store and database. A different device
  remains authorized. Cover grant-specific revocation and revoke-before-grant too.
- Durable bound: seed an independently encoded, fully signed 4095-fact snapshot to
  avoid thousands of full-value disk rewrites. Open/reverify, add the 4096th fact,
  reject invalid/foreign traffic without saturation, accept duplicate without write,
  persist the 4097th witness, reopen and prove permanent denial. Include all three
  kinds and keep revoked grants counted. A forged witness/duplicate at 4097 is corrupt.
- Storage failure/ordering: a deterministic instrumented backend separates volatile
  Put from durable Sync. Pause at the Sync barrier; no success or new authority may
  escape beforehand. Fail Put before and after retaining bytes, and fail Sync;
  assert disabled-handle denial, no rollback write, and old/new/corrupt outcomes on
  explicit reopen. Test an already-canceled context separately. Use channels, not sleeps.
- `TestAcknowledgedRevocationSurvivesProcessKill`: a test-binary child opens a fresh
  LevelDB directory, creates a grant, applies a revocation and signals only after
  acknowledgment. Parent then kills that child before deferred cleanup/database
  Close, waits, reopens and proves denial. Use only t.TempDir and that child process;
  no daemon, wallet, real data, shell-based process discovery or uncontrolled process.
  Bound readiness/wait and guarantee child cleanup on test failure. The regular child
  helper path is inert without its explicit test-only environment marker.
- Concurrent Apply/read calls on one Store retain both independently submitted facts
  and never lose a revocation; Close races are safe and reject later access. Nil/zero,
  deep-copy and capability/binding checks have meaningful authorized controls.
- Snapshot table cases independently assemble valid bytes and also recompute checksums
  for structural/foreign/duplicate/signature failures. Cover minimum, maximum,
  one-byte truncation/oversize, count 4096/4097/4098, bad magic and trailing bytes.
- `FuzzOpenSnapshot`: valid empty/grant/revocation seeds and meaningful corruptions;
  no panic, no writes on Open failure, no mutation of supplied bytes, and successful
  loads agree with replay through accountauth. In-memory fixture backend only for
  fuzzing; keep expensive 4095-fact setup in the ordinary boundary test, not every
  fuzz iteration. No separate file parser, network fuzz infrastructure or benchmark.

One later falsification: bypass only the storage Put for a RevokeDevice while keeping
its in-memory application/acknowledgment. TestRevocationSurvivesReopen must fail because
restart restores the revoked authority; restore source bytes and require it to pass.
Reviewer pins the exact fault after source review. Hermes must not invent a patch.

## Execution and acceptance sequence

Only Sol's three test-source files are active now. Reviewer reviews them, then Hermes
captures missing-implementation red. Sol next authors production after red acceptance;
reviewer reads it before Hermes green/falsification/security execution and publication.
Transitions stay in this ticket/current-task pointer; use one execution report,
`docs/testing/BBGO-ACC-002-EXECUTION-01.md`. No separate handoff per command. No actor
has been launched and no command below has been executed for ACC-002.

Future commands, cwd modern/, Go 1.27.0 and GOWORK=off:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountstore -count=1
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test -race ./accountauth ./accountstore -count=1
env GOWORK=off GOTOOLCHAIN=go1.27.0 go vet ./accountstore
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountstore -run '^$' -fuzz '^FuzzOpenSnapshot$' -fuzztime=30s -parallel=2
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountstore -run '^TestRevocationSurvivesReopen$' -count=1
gosec -tests ./accountstore/...
```

First command supplies both missing-contract compile red and later green. A setup,
syntax or dependency error is not the intended red. The targeted last test is used
once under the reviewer-pinned fault and once after byte-exact restoration. The
race suite includes real restart/crash tests; no whole-daemon test sweep is needed.
Use gosec v2.29.0 and govulncheck v1.7.0 with verified tool identities. From repo root:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 python3 scripts/govulncheck_policy.py source
gitleaks git --pre-commit --staged --redact=100 --no-banner .
```

Gitleaks v8.30.1 waits for separately activated, enumerated publication. Any new
finding, unknown scan failure or secret blocks acceptance pending reviewer disposition;
existing exceptions retain their exact scope/expiry. No new suppressions/baseline edits.
No binary rebuild/SBOM: this isolated package is not yet imported by a runtime binary.

Hermes execution activation must name task-owned disk-backed directories under
`modern/dist/acc002/`, with filesystem check and offline tests using installed/cached
dependencies. Network is limited to pinned tool acquisition/advisory access, never
test behavior. No user data or real services. Capture raw command output, direct
return codes, actual argv/environment/timestamps, tool identities and before/after
source hashes automatically at execution. Retain raw files before summarizing them;
do not hand-author output or metadata afterward. Bind the final staged scan to the
actual staged blob IDs and do not edit those bytes after scanning. Execution activation
will supply the exact capture command for Hermes to run unchanged, avoiding another
capture-design or report-only handoff cycle.

## Reviewer publication

Reviewer governance scope is exactly this new ticket and
`docs/handoff/CURRENT_TASK.md` in bb-go, based on the HEAD above. Desktop documents
are read-only references and its current routing already follows the daemon. Verify
the nine frozen source hashes, target-file absence, local links and scoped whitespace;
commit/push only these two documents. No production, tests, reports or unrelated work
are part of the reviewer publication.

Initial review checks: nine frozen hashes match, all six target files are absent,
snapshot size arithmetic is consistent, and all 38 local links across the two scoped
documents resolve. Whitespace checks passed. No implementation, compiler, test,
scanner or actor execution was performed.
