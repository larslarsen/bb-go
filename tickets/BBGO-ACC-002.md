# BBGO-ACC-002 — durable account grants and revocations

Reviewer: Codex, 2026-09-17, High. **Active: Hermes green/falsification/security; review 03.**
Actor: Hermes on a free Nous Portal model, owner-relayed. This ticket is the sole
handoff. Read AGENTS.md, TESTING.md and [CURRENT_TASK](../docs/handoff/CURRENT_TASK.md).
Production source is accepted for execution, not final acceptance. All source is
frozen. Run only review 03's capture; no Git/publication or source correction yet.

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

**Historical test-authoring scope; now frozen:** `modern/accountstore/store_test.go`, `persistence_test.go`,
`fuzz_test.go`. Helpers stay in those files. Tests exercise the public contract;
do not copy the verifier or add production stubs. Synthetic reproducible keys only.
Gofmt of these three test files is allowed; no compiler/test/fuzz/scanner execution,
dependency changes, records, Git or actor launch. Stop with the ticket/source pointer.

**Production scope, authored under review 02; now frozen:** `modern/accountstore/types.go`, `store.go`,
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

Initial sequence (reviews below govern the current phase): reviewer reviews Sol's
three test-source files, then Hermes
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


## Review 01 — test source accepted for expected-red execution, 2026-09-17

Reviewed at HEAD `dd1abf272deb6ff1f5d8f31643094d8808b65704`. All nine frozen input
hashes above match. The drop contains exactly the three authorized test files;
production is absent. Source review only: no compiler, test, fuzz or scanner was
run by Codex. Sol had no execution/report authority, so no Sol execution report is
required. Eleven ordinary test functions and one fuzz target, 1,897 lines total:

| Input | SHA-256 |
| --- | --- |
| modern/accountstore/store_test.go | 6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32 |
| modern/accountstore/persistence_test.go | 25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a |
| modern/accountstore/fuzz_test.go | ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af |

Line counts respectively 1,008 / 725 / 164. Independent fixtures use the accepted
literal signature domains and byte offsets. Reviewed positive grant controls,
real LevelDB reopen and acknowledged-child-kill cases, 4095/4096/4097 boundaries,
forged/duplicate overflow witnesses, failed writes with old/new/corrupt outcomes,
disabled handles, concurrent methods, owned bytes and snapshot fuzzing. The failure
backend separates pending Put bytes from durable Sync bytes. No real daemon,
wallet, public peers or user database is used.

Review limits: this is permission to compile the tests, not evidence that they pass.
The Sync barrier's immediate nonblocking read check is only opportunistic evidence
about pre-Sync visibility; it is not a deterministic proof that the reader ran before
release. Review production lock/persistence order explicitly before green. Fuzz
replay checks the returned Records; the independent ordinary snapshot/restart tests
remain necessary to detect omitted records. No claim of exhaustive parser coverage.

### Historical Hermes work — expected-red capture, closed by review 02

Run the exact command below from the bb-go repository root. It checks the source
pins and disk backing, captures the installed Go identity, and runs only the package
compile/red command with downloads disabled. It automatically retains stdout/stderr,
exit status, UTC times, argv, explicit environment and before/after hashes. The
package currently lacks Store/Create/Open/constants/errors, so undefined-contract
compiler errors are expected. A syntax, fixture API or dependency error is a gap,
not an accepted red. Do not repair source or fabricate a successful result.

Writable scope: task-owned `modern/dist/acc002/` artifacts/caches and
`docs/testing/BBGO-ACC-002-EXECUTION-01.md` only. Source files, dependencies and all
other records are frozen. No Git mutation, scans, production, daemon build/restart,
network tests or actor launch. Return the report pointer; reviewer accepts the actual
red before activating Sol's three production files in this same ticket. If the
capture stops before writing its report, record that failure and actual output in
the designated report; never reconstruct missing raw command output.

```sh
python3 - <<'ACC002_CAPTURE'
import datetime, hashlib, json, os, pathlib, re, shutil, subprocess
root = pathlib.Path.cwd().resolve()
ticket = root / 'tickets/BBGO-ACC-002.md'
assert ticket.is_file(), 'run from bb-go root'
pins = dict(re.findall(r'^\| (modern/[^ |]+) \| ([0-9a-f]{64}) \|$', ticket.read_text(), re.M))
assert len(pins) == 12, 'expected nine frozen inputs and three test files'
def hashes():
    return {p: hashlib.sha256((root / p).read_bytes()).hexdigest() for p in pins}
before = hashes()
assert before == pins, 'source hash mismatch'
assert {p.name for p in (root / 'modern/accountstore').iterdir()} == {
    'store_test.go', 'persistence_test.go', 'fuzz_test.go'}, 'unexpected package files'
fs = subprocess.run(['findmnt', '-T', str(root / 'modern'), '-n', '-o', 'FSTYPE'],
                    capture_output=True, text=True, check=True).stdout.strip()
assert fs and fs not in ('tmpfs', 'ramfs'), 'disk-backed task directory required'
def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()
stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
base = root / 'modern/dist/acc002'
run = base / ('red-' + stamp)
run.mkdir(parents=True, exist_ok=False)
for name in ('tmp', 'gocache'):
    (base / name).mkdir(exist_ok=True)
go = shutil.which('go')
assert go, 'installed Go required'
env = {'HOME': os.environ['HOME'], 'PATH': os.environ['PATH'], 'LANG': 'C.UTF-8',
       'GOWORK': 'off', 'GOTOOLCHAIN': 'go1.27.0', 'GOPROXY': 'off', 'GOSUMDB': 'off',
       'GOENV': 'off', 'GOFLAGS': '-mod=readonly',
       'GOMODCACHE': str(pathlib.Path.home() / 'go/pkg/mod'),
       'GOCACHE': str(base / 'gocache'), 'TMPDIR': str(base / 'tmp')}
meta = {'started_utc': utc(), 'cwd': str(root / 'modern'), 'environment': env,
        'filesystem': fs, 'before': before, 'go_launcher': go,
        'go_launcher_sha256': hashlib.sha256(pathlib.Path(go).read_bytes()).hexdigest(),
        'commands': []}
commands = [('go-version', [go, 'version']), ('go-root', [go, 'env', 'GOROOT']),
            ('expected-red', [go, 'test', './accountstore', '-count=1'])]
for name, argv in commands:
    entry = {'name': name, 'argv': argv, 'started_utc': utc()}
    with (run / (name + '.log')).open('wb') as output:
        try:
            result = subprocess.run(argv, cwd=root / 'modern', env=env,
                                    stdout=output, stderr=subprocess.STDOUT, timeout=180)
            entry['returncode'] = result.returncode
        except subprocess.TimeoutExpired:
            entry['returncode'] = None
            entry['failure'] = 'capture timeout after 180 seconds'
    entry['finished_utc'] = utc()
    entry['log_sha256'] = hashlib.sha256((run / (name + '.log')).read_bytes()).hexdigest()
    meta['commands'].append(entry)
    if name == 'go-root' and entry['returncode'] == 0:
        compiler = pathlib.Path((run / 'go-root.log').read_text().strip()) / 'bin/go'
        meta['selected_go'] = str(compiler)
        meta['selected_go_sha256'] = hashlib.sha256(compiler.read_bytes()).hexdigest()
    if entry['returncode'] is None or (name != 'expected-red' and entry['returncode'] != 0):
        break
meta['after'] = hashes()
meta['source_unchanged'] = meta['after'] == before
meta['finished_utc'] = utc()
(run / 'metadata.json').write_text(json.dumps(meta, indent=2) + '\n')
report = root / 'docs/testing/BBGO-ACC-002-EXECUTION-01.md'
assert not report.exists(), 'retain existing report; do not overwrite'
lines = ['# BBGO-ACC-002 execution 01', '',
         'Expected-red capture only; reviewer acceptance pending.',
         'Local path prefixes are labeled in this report; raw logs/metadata retain exact values.', '',
         'Artifacts: `' + str(run.relative_to(root)) + '` (retained locally).',
         'Filesystem: `' + fs + '`. Source unchanged: `' + str(meta['source_unchanged']) + '`.', '',
         '## Captured commands', '']
for entry in meta['commands']:
    display = json.dumps(entry, indent=2)
    output = (run / (entry['name'] + '.log')).read_text(errors='replace').rstrip()
    for local, label in ((str(root), '<repo>'), (str(pathlib.Path.home()), '<home>')):
        display, output = display.replace(local, label), output.replace(local, label)
    display = display.replace(go, '<go-launcher>')
    lines += ['```json', display, '```', '', '```text', output, '```', '']
lines += ['## Metadata', '', 'Full environment/tool identities and before/after pins',
          'are retained in `metadata.json`. SHA-256: `' +
          hashlib.sha256((run / 'metadata.json').read_bytes()).hexdigest() + '`.', '']
report.write_text('\n'.join(lines))
print(report.relative_to(root))
print(run.relative_to(root))
ACC002_CAPTURE
```

The runner's own successful exit means capture completed, not that the package
passed. Review the captured go-test returncode and output. Do not rerun merely to
change formatting. Cached-toolchain/dependency absence must be recorded as a gap;
this phase authorizes no dependency download or changes to go.mod/go.sum.

### Reviewer publication for review 01

Exact reviewer-only scope: this ticket, `docs/handoff/CURRENT_TASK.md` and
`tickets/BBGO-NET-001.md`, based on HEAD above. NET-001 records the concurrent owner
direction to use the public IPFS swarm; it is queued and does not alter this isolated
storage contract. Preserve the untracked Sol test drop and all unrelated work.

Publication checks: all 12 source pins match; 39 local document links resolve;
scoped whitespace checks pass. The embedded capture snippet parses as Python; it
was not executed. The staging scope contains only the three reviewer documents.


## Review 02 — expected red accepted; production source active, 2026-09-17

Baseline: `fb52a11d5e034d2c925717127db01424421d0af9`. The nine original input
pins and three test-source pins still match. Exactly the three test files exist in
accountstore; `types.go`, `store.go` and `codec.go` remain absent. No compiler,
test or scanner was executed by Codex during this review.

### Evidence and disposition

Read [execution report 01](../docs/testing/BBGO-ACC-002-EXECUTION-01.md), SHA-256
`3f816f8d919901de6607b6708a489cf957c107169021833bbfb4fa8b5d822be2`, and its
retained capture. The review-01 command stopped at `go version`, exit 1: the
reviewer-supplied `GOSUMDB=off` prevented launcher verification of Go 1.27.0.
That capture did not execute the package test. This was a capture-command defect;
it is not evidence of a source failure.

Hermes then ran, cwd `modern/`:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 GOPROXY=off GOFLAGS=-p=2 go test ./accountstore -count=1
```

**Actual exit: 1. Accepted missing-implementation red.** Output names undefined
`MaxStoredRecords`, `MaxSnapshotBytes`, `Open`, `ErrCorrupt`, `Store` and `Create`,
then the compiler's too-many-errors limit. It contains no reported syntax or
fixture/dependency error. It cannot prove the absence of errors hidden after that
limit; production compilation remains necessary. No test body ran.

The reviewer independently recovered the actual execution record read-only from
Hermes's local session `20260913_213737_aba8d9`: terminal call 86697 started the
command at 18:23:28 UTC; process result 86700 reports exit 1, observed at
18:24:39 UTC (not a claimed precise completion time); read result 86702 contains
the complete 802-byte output. The retained `acc002-expected-red.log` in the system
temporary directory matches those compiler diagnostics and has SHA-256
`b081520c686db9031026c8501fa3d2839cef0cfd36afb5c2cfb7ff44cefd235c`.
Earlier terminal results 86692 and 86696 identify installed Go 1.27.0 linux/amd64.
The selected cached toolchain binary currently hashes to
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`;
this is a review-time hash, not a retrospectively claimed execution-time capture.

Report qualifications: Hermes manually rewrote the report after the direct run.
Its `metadata.json` covers only the failed automatic capture, not the direct test;
its filesystem label `ext2/ext3` disagrees with the captured `ext4`. The direct run
also changed the authorized environment/capture procedure: inherited settings were
not fully captured, task-specific cache/temp paths were not pinned, and output went
to a small temporary log. Do not treat it as execution of the supplied script or as
fully captured offline acceptance. These deviations are recorded, not authorized
as precedent. The actual command, exit, compiler output and unchanged source are
sufficient for this narrowly scoped missing-implementation check. No rerun or
report-only handoff is required before production authoring.

Original capture directory: `modern/dist/acc002/red-20260917T181655552699Z`.
Metadata SHA-256: `fccacd7e02acb078eb92194b81fa92ee6bfc12d0aec8af7b8c3069384ba55b07`.
Retain it and the execution report. When green execution is activated, use the
already installed, verified Go 1.27.0 binary directly with `GOTOOLCHAIN=local` in
the automatic capture to avoid launcher verification, and retain the complete
explicit environment and task-owned disk-backed cache/temp paths. The later Hermes
phase will append to the same report and point to this qualification; do not rewrite
historical evidence or launch another actor now.

### Historical Sol production task — closed by review 03

Author exactly these new files under the frozen contract above:

- `modern/accountstore/types.go`: constants, error sentinels and private Store state.
- `modern/accountstore/store.go`: Create/Open, serialized operations, persistence
  before acknowledgment, permanent handle denial after storage failure, owned reads
  and caller-owned backend lifecycle.
- `modern/accountstore/codec.go`: bounded snapshot encoding/decoding, signature and
  exact-account verification, semantic duplicate rejection and overflow-witness replay.

All three test files and nine original inputs are frozen. No runtime imports,
network changes, dependencies, records or other source edits. Do not weaken the
contract to satisfy tests. In particular, hold the Store lock through Put/Sync and
committed-state advancement; fully verify before duplicate/capacity handling; never
shallow-copy KnownState or mutate it before persistence. Open never creates or repairs.
Keep this an isolated package; NET-001 remains queued.

Gofmt of the three new production files is allowed. No compiler/test/fuzz/scanner
execution, Git, report editing, daemon build/restart or actor launch. Stop with the
source/ticket pointer for reviewer source review. Hermes green, the reviewer-pinned
falsification, scans and source/evidence publication remain inactive until that review.

### Reviewer publication for review 02

Exactly `tickets/BBGO-ACC-002.md` and `docs/handoff/CURRENT_TASK.md`, based on the
baseline above. Preserve the untracked tests, execution report and all unrelated
work; none are part of this reviewer publication. Verify the 12 source pins, local
links and scoped whitespace before committing these two governance documents only.


## Review 03 — production source accepted for execution, 2026-09-17

Baseline: `9e51b7c111fe177b9ef6d4fb2fc5c044b2a10993`. The 12 original/test pins
still match. Exactly the six authorized files exist in accountstore. New production:

| Input | SHA-256 |
| --- | --- |
| modern/accountstore/types.go | d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f |
| modern/accountstore/store.go | 7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81 |
| modern/accountstore/codec.go | fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab |

Production line counts: 40 / 267 / 146, total 453. Tests remain 1,897 lines,
11 ordinary functions and one fuzz target. No blocking source finding. Create/Open
remain distinct; derived keys and complete signed snapshots are bounded. Decoder
checks lengths/count before collections, rejects duplicate facts and verifies every
record for the pinned controller, including the 4097th saturation witness. Apply
holds the same mutex used by all readers through verification, Put, Sync and state
advancement. Failed storage permanently disables the handle without rollback writes.
Records are copied, Close does not own the backend, and no runtime imports were added.
The lock/order review directly addresses review 01's opportunistic reader-test limit.
No compiler, test, fuzz or scanner was executed by Codex.

### Active Hermes execution

Run the exact capture below from bb-go root, unchanged. It uses the installed,
hash-pinned Go 1.27.0 executable directly with `GOTOOLCHAIN=local`, avoiding the
review-01 launcher failure. It checks all 15 source pins plus scanner/policy identities,
records the explicit environment and raw outputs automatically, and appends results
to the existing execution report. The report's original section remains historical
and is qualified by review 02. No manual replacement of logs, timestamps or metadata.

The batch runs package green, the accountauth/accountstore race suite, vet, 30-second
snapshot fuzzing, the pinned fault and restored regression, gosec v2.29.0, and the
existing govulncheck v1.7.0 source policy. Vet/scan results are evidence for review,
not permission to suppress findings. Govulncheck's existing DHT exception retains
its exact v0.42.2 scope and 2026-11-29 expiry; no exception is extended here.
Advisory access is allowed only for that scan. Tests use no public services, and
module downloads are disabled. Gitleaks/staging/publication remain a later phase.

The fault is fixed: wrap only Apply's `s.backend.Put` block in
`if record.Kind() != accountauth.RevokeDevice { ... }`; keep Sync, in-memory application
and acknowledgment intact. Use Go's build overlay so tracked/untracked source bytes
never change. The required failure is `TestRevocationSurvivesReopen` reporting
`device revocation after reopen = true, <nil>`. A compile/setup/timeout failure is
not successful falsification. The subsequent identical test without the overlay must
pass. Overlay behavior was checked in installed Go 1.27.0's `cmd/go/alldocs.go`;
this is a build-time source substitution, not a runtime file interception.

Writable paths: task-owned `modern/dist/acc002/` captures, caches and overlay files,
and append-only `docs/testing/BBGO-ACC-002-EXECUTION-01.md`. An unexpected Go fuzz
failure may retain its generated reproducer under `modern/accountstore/testdata/fuzz/`;
preserve and report it for review, without editing/promoting it into source. No other
source/test/dependency/record changes, actor launch, daemon build/restart, Git or
publication. The isolated package still has no UI/runtime effect. If the exact
capture cannot run, record the actual failure in the report and stop; do not substitute
an ad hoc execution environment. Review the already captured results before any rerun.

```sh
python3 - <<'ACC002_GREEN'
import datetime, hashlib, json, os, pathlib, re, signal, subprocess, sys
root = pathlib.Path.cwd().resolve()
ticket = root / 'tickets/BBGO-ACC-002.md'
assert ticket.is_file(), 'run from bb-go root'
pins = dict(re.findall(r'^\| (modern/[^ |]+) \| ([0-9a-f]{64}) \|$', ticket.read_text(), re.M))
assert len(pins) == 15
sha = lambda p: hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()
hashes = lambda: {p: sha(root / p) for p in pins}
before = hashes()
assert before == pins, 'source hash mismatch'
assert {p.name for p in (root / 'modern/accountstore').iterdir()} == {
    'types.go', 'store.go', 'codec.go', 'store_test.go', 'persistence_test.go', 'fuzz_test.go'}
report = root / 'docs/testing/BBGO-ACC-002-EXECUTION-01.md'
assert report.is_file() and '## Review 03 automated capture' not in report.read_text()
fs = subprocess.run(['findmnt', '-T', str(root / 'modern'), '-n', '-o', 'FSTYPE'],
                    capture_output=True, text=True, check=True).stdout.strip()
assert fs and fs not in ('tmpfs', 'ramfs'), 'disk-backed task directory required'
base = root / 'modern/dist/acc002'
base.mkdir(parents=True, exist_ok=True)
assert not base.is_symlink()
assert subprocess.run(['findmnt', '-T', str(base), '-n', '-o', 'FSTYPE'],
                     capture_output=True, text=True, check=True).stdout.strip() == fs
utc = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
run = base / ('green-' + stamp)
run.mkdir(exist_ok=False)
for name in ('tmp', 'gocache', 'xdg-cache'):
    (run / name).mkdir()
go = pathlib.Path.home() / 'go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go'
gosec = pathlib.Path.home() / 'go/bin/gosec'
govuln = pathlib.Path.home() / 'go/bin/govulncheck'
policy = root / 'scripts/govulncheck_policy.py'
tool_pins = {str(go): '1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8',
             str(gosec): 'eb00a1fb095b161a48c5bcadbe1e246bbafe270da497a122d2e63ade346954c2',
             str(govuln): '6c92f0536311f5e2083a839c75558e3fb986758a320a402aa8f524c85ffd7400',
             str(policy): '709cb00d44c62ef6e2d394f457407183d6fb90bc98958c80db0261607bc3c77c'}
assert all(sha(p) == h for p, h in tool_pins.items()), 'tool/policy identity mismatch'
env = {'HOME': str(pathlib.Path.home()), 'LANG': 'C.UTF-8',
       'PATH': str(go.parent) + ':' + str(govuln.parent) + ':/usr/bin:/bin',
       'GOWORK': 'off', 'GOTOOLCHAIN': 'local', 'GOENV': 'off',
       'GOPROXY': 'off', 'GOSUMDB': 'off', 'GOFLAGS': '-mod=readonly -p=2',
       'GOMAXPROCS': '2', 'CGO_ENABLED': '1', 'GOTELEMETRY': 'off',
       'GOMODCACHE': str(pathlib.Path.home() / 'go/pkg/mod'),
       'GOCACHE': str(run / 'gocache'), 'TMPDIR': str(run / 'tmp'),
       'XDG_CACHE_HOME': str(run / 'xdg-cache'), 'PYTHONDONTWRITEBYTECODE': '1'}
meta = {'started_utc': utc(), 'filesystem': fs, 'environment': env,
        'tools': tool_pins, 'before': before, 'commands': []}
def capture(name, argv, cwd=None):
    argv = list(map(str, argv))
    cwd = cwd or root / 'modern'
    entry = {'name': name, 'argv': argv, 'cwd': str(cwd), 'started_utc': utc()}
    print('Starting ' + name, flush=True)
    with (run / (name + '.stdout')).open('wb') as out, (run / (name + '.stderr')).open('wb') as err:
        proc = subprocess.Popen(argv, cwd=cwd, env=env, stdout=out, stderr=err, start_new_session=True)
        try:
            entry['returncode'] = proc.wait(timeout=900)
        except subprocess.TimeoutExpired:
            os.killpg(proc.pid, signal.SIGKILL)
            entry['returncode'] = proc.wait()
            entry['timed_out'] = True
    entry['finished_utc'] = utc()
    entry['stdout_sha256'] = sha(run / (name + '.stdout'))
    entry['stderr_sha256'] = sha(run / (name + '.stderr'))
    entry['after'] = hashes()
    meta['commands'].append(entry)
    (run / 'metadata.json').write_text(json.dumps(meta, indent=2) + '\n')
    assert entry['after'] == before, 'source changed during ' + name
    print(name + ': exit ' + str(entry['returncode']), flush=True)
    return entry
try:
    assert capture('go-version', [go, 'version'])['returncode'] == 0
    assert capture('scanner-identities', [go, 'version', '-m', gosec, govuln])['returncode'] == 0
    for name, args in [
        ('green', ['test', './accountstore', '-count=1', '-timeout=3m']),
        ('race', ['test', '-race', './accountauth', './accountstore', '-count=1', '-timeout=3m']),
        ('vet', ['vet', './accountstore']),
        ('fuzz', ['test', './accountstore', '-run', '^$', '-fuzz', '^FuzzOpenSnapshot$',
                  '-fuzztime=30s', '-parallel=2', '-timeout=3m'])]:
        result = capture(name, [go] + args)
        assert result['returncode'] == 0, name + ' did not pass'
    source = root / 'modern/accountstore/store.go'
    original = source.read_text()
    needle = '\tif err := s.backend.Put(ctx, s.key, snapshot); err != nil {\n\t\ts.disableLocked()\n\t\treturn storageError("put state", err)\n\t}\n'
    assert original.count(needle) == 1
    replacement = '\tif record.Kind() != accountauth.RevokeDevice {\n' + ''.join('\t' + line for line in needle.splitlines(True)) + '\t}\n'
    fault = run / 'store-fault.go'
    fault.write_text(original.replace(needle, replacement, 1))
    assert sha(fault) == 'f9f5b359b8f7414d28cc9bfcf45b35cd3236735a03e4767615fc8468480e2849'
    overlay = run / 'overlay.json'
    overlay.write_text(json.dumps({'Replace': {str(source): str(fault)}}))
    meta['fault_sha256'] = sha(fault)
    meta['overlay_sha256'] = sha(overlay)
    args = ['test', './accountstore', '-run', '^TestRevocationSurvivesReopen$', '-count=1', '-timeout=3m']
    result = capture('fault', [go, args[0], '-overlay=' + str(overlay)] + args[1:])
    output = (run / 'fault.stdout').read_text(errors='replace') + (run / 'fault.stderr').read_text(errors='replace')
    meta['fault_detected'] = result['returncode'] == 1 and not result.get('timed_out') and 'device revocation after reopen = true, <nil>' in output
    restored = capture('restored', [go] + args)
    assert meta['fault_detected'] and restored['returncode'] == 0, 'falsification/restoration not proven'
    capture('gosec', [gosec, '-tests', './accountstore/...'])
    policy_capture = "import datetime, json, pathlib, runpy, subprocess, sys\nrun = pathlib.Path(sys.argv[2])\nns = runpy.run_path(sys.argv[1], run_name='acc002_policy')\ndef execute(argv, **kwargs):\n    utc = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()\n    entry = {'argv': argv, 'cwd': kwargs.get('cwd'), 'started_utc': utc()}\n    result = subprocess.run(argv, **kwargs)\n    (run / 'govulncheck.sarif').write_text(result.stdout or '')\n    (run / 'govulncheck.stderr').write_text(result.stderr or '')\n    entry.update(returncode=result.returncode, finished_utc=utc())\n    (run / 'govulncheck-command.json').write_text(json.dumps(entry, indent=2))\n    return result\nraise SystemExit(ns['main'](['source'], execute=execute))\n"
    capture('govulncheck-policy', [sys.executable, '-c', policy_capture, policy, run], root)
except Exception as exc:
    meta['capture_failure'] = str(exc)
finally:
    meta['after'] = hashes()
    meta['source_unchanged'] = meta['after'] == before
    meta['finished_utc'] = utc()
    meta['artifacts'] = {p.name: sha(p) for p in run.iterdir() if p.is_file() and p.name != 'metadata.json'}
    (run / 'metadata.json').write_text(json.dumps(meta, indent=2) + '\n')
    def public(value):
        return value.replace(str(root), '<repo>').replace(str(pathlib.Path.home()), '<home>')
    lines = ['','## Review 03 automated capture', '',
             'Review 02 qualifies the earlier red capture. This section preserves this run only.',
             'Local paths are labeled here; raw artifacts retain exact values.', '',
             'Artifacts: `' + str(run.relative_to(root)) + '`.',
             'Metadata SHA-256: `' + sha(run / 'metadata.json') + '`.', '',
             '```json', public(json.dumps(meta, indent=2)), '```', '']
    for entry in meta['commands']:
        lines += ['### ' + entry['name'], '']
        for stream in ('stdout', 'stderr'):
            output = (run / (entry['name'] + '.' + stream)).read_text(errors='replace')
            lines += [stream + ':', '', '```text', public(output.rstrip()), '```', '']
    with report.open('a') as out:
        out.write('\n'.join(lines))
    print(report.relative_to(root), flush=True)
    print(run.relative_to(root), flush=True)
ACC002_GREEN
```

Capture completion does not mean acceptance: every returncode, fault diagnostic and
scanner finding requires review. No source repair, suppression, repeated run or Git
operation is authorized by a successful capture. The scanner stages are independent;
retain both results even if gosec reports findings. Stop and relay this ticket/report
pointer when complete.

### Reviewer publication for review 03

Exactly `tickets/BBGO-ACC-002.md` and `docs/handoff/CURRENT_TASK.md`, based on the
baseline above. Preserve all six untracked source/test files, the execution report
and unrelated work. None are part of this reviewer publication. Source acceptance
permits execution only; final behavior/security acceptance and publication remain open.

Publication checks: all 15 source pins and the four tool/policy pins match. Both
embedded Python snippets parse without execution. The fault substitution has one
exact match and its proposed bytes are hash-pinned; no source file was modified or
compiled to check it. Local document links and scoped whitespace checks pass.
