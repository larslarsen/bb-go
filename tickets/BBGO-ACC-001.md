# BBGO-ACC-001 — portable account grants and revocation verifier

Reviewer: Codex, 2026-09-17. **Active phase: publication evidence completion, review 07.**
Actor: Hermes, free Nous Portal model, owner-relayed. This ticket is the handoff;
do not create another handoff for each subsection. Read AGENTS.md, TESTING.md and
[CURRENT_TASK](../docs/handoff/CURRENT_TASK.md). The daemon's principal-dev role
routes cryptographic/protocol-core source to Sol. Source publication and passing CI
are verified in review 07. The secret scan lacks retained evidence; Hermes completes
the bounded committed-range scan and report-only closeout below. No source changes
or test reruns. Ticket closure awaits that evidence.
The reviewer has not launched an actor or executed the tests.

## Result and boundaries

Build an isolated `modern/accountauth` package that verifies a stable account
controller's device grants and revocations. Two independent device keys can belong
to one account; removing one must leave the other authorized. Messages, payment
requests and trust operations have separate permissions. Known revocations cannot
be undone by delivery order, a replay or another grant for the revoked device key.

Architecture decision and primary-source comparison:
[portable accounts](../../bb-desktop/docs/architecture/BB-ACCOUNT-RECIPIENT-PROPOSAL-01.md).
Use standard-library Ed25519 and SHA-256. No new dependency, OpenPGP/KERI integration,
custom signature algorithm, account registry or readable-name lookup.

This slice verifies public records and maintains bounded in-memory **known** authority.
It does not provide network freshness, durable storage, encryption, key generation or
custody, pairing UI, root backup, account migration, daemon/API integration or payment
approval. Do not import it into an existing package yet. Existing peer identities and
all v1 social/payment signatures stay unchanged. Device application keys are separate
from the account controller and existing libp2p transport keys; routing is a later
signed-device contract. No real node, wallet or user data is needed.

The controller is backed up separately from ordinary device keys. Device replacement
preserves account identity. Controller loss without backup, or controller compromise,
is not repaired by this profile; changing the controller requires explicit new-account
migration. No signature library establishes freshness of unseen revocations.

## Baseline and exact paths

Daemon source baseline: `cd497749a771b063300e2c3edf8748fe285b06c4`. Reviewer governance
commits may descend from it without changing source. Desktop decision baseline:
`58c854dbeda3031a57ac18ee601f1a588496d28d`, plus its linked authority-selection revision.
Before editing, verify these frozen source inputs. All six target files were absent
at initial authorization. Review 04 now pins all six completed package files.
Verify those identities plus the four frozen inputs before execution. Review 03's
restoration, production and formatting permissions are closed. Unexpected starting
identities require review.

| Frozen input | SHA-256 |
| --- | --- |
| modern/go.mod | 1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783 |
| modern/go.sum | 4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a |
| modern/network/identity.go | b43f6ad90149ce41526aa3b0f85329dbbf847feaaf0442529a204c637833b4fa |
| modern/payment/signature.go | 9573cecec02e0df951ccb08d70ed98ca112019f72ab94827846ffc2179ac9613 |

**Existing test-source paths, frozen at review 04 identities:**

- `modern/accountauth/records_test.go`
- `modern/accountauth/state_test.go`
- `modern/accountauth/fuzz_test.go`

The requested state-test restoration is accepted; all tests and fixtures are frozen.

**Production paths, fully restored and frozen:**

- `modern/accountauth/types.go`
- `modern/accountauth/records.go`
- `modern/accountauth/state.go`

Preserve unrelated dirty/untracked files, especially cancelled DEV-001 work. Hermes's
publication and evidence scope is enumerated in review 07; no source, dependency,
workflow or AGENTS edits. The source contract remains: author tests in
package `accountauth` against the public contract below; do not add production stubs
or a substitute verifier inside tests. Helpers may assemble and sign synthetic inputs.

## Frozen format, version 1

This is a BitBook application authorization format, not an implementation of a DID
method or OpenPGP. All lengths below count bytes. No JSON, length prefixes, optional
fields, text normalization, timestamps or trailing bytes. `||` means concatenation.
Domain strings below are ASCII followed by one NUL byte (`\x00`), not four literal
escape characters. All integers are unsigned; capabilities use eight-byte big endian.

Controller and device public keys are raw 32-byte Ed25519 keys. Signatures are 64-byte
**pure Ed25519** signatures over the specified full domain-plus-payload bytes, without
prehashing or Ed25519ctx/ph. Reject an all-zero controller, device or grant target;
reject a device equal to its controller. Signature validation uses `crypto/ed25519`;
do not implement curve arithmetic or broaden that verifier's acceptance behavior.

`AccountID = SHA256("bitbook/accountauth/v1/account\x00" || controller_public_key)`.
It is an opaque 32-byte value. Its derivation does not prove authority by itself.

| Kind | Payload (before signatures) | Complete record |
| --- | --- | --- |
| `Grant = 1` | version `0x01`, kind `0x01`, controller key (32), device key (32), capabilities (8), grant nonce (32) | 106-byte payload, controller signature (64), device signature (64): **234 bytes** |
| `RevokeGrant = 2` | version `0x01`, kind `0x02`, controller key (32), target GrantID (32) | 66-byte payload, controller signature (64): **130 bytes** |
| `RevokeDevice = 3` | version `0x01`, kind `0x03`, controller key (32), target device key (32) | 66-byte payload, controller signature (64): **130 bytes** |

Signatures:

- Grant controller: `"bitbook/accountauth/v1/grant/root\x00" || payload`.
- Grant device: `"bitbook/accountauth/v1/grant/device\x00" || payload`.
- RevokeGrant controller: `"bitbook/accountauth/v1/revoke-grant\x00" || payload`.
- RevokeDevice controller: `"bitbook/accountauth/v1/revoke-device\x00" || payload`.

Both grant signatures are mandatory and bind the same entire payload, including the
controller, device, permissions and nonce. The device signature confirms this specific
account/permission binding. It never authorizes another device or a revocation.

`GrantID = SHA256("bitbook/accountauth/v1/grant-id\x00" || grant_payload)`.
Signature bytes are deliberately excluded from grant identity. The nonce must not be
all zero; an eventual issuer must generate 32 cryptographically random bytes for each
new grant. The verifier cannot prove randomness. Repeating the same grant is idempotent;
changing permissions means a new grant and, when reducing authority, revoking every
older grant that would still authorize the removed operation.

Reject unknown version/kind, incorrect length (including extra bytes), zero/unknown
capabilities, zero nonce and either invalid/missing signature. Check framing before
copying attacker-sized input or calling signature verification. `MaxRecordBytes = 234`.
Every verification failure returns a zero `Record` plus a non-nil error; no partial
verified record escapes. Error text is not an API contract.

### Capabilities

`Capability` is a uint64 mask. Only these five bits are valid in v1:

| Constant | Value | Permission prerequisite |
| --- | --- | --- |
| `CapMessage` | 1 | Sign account messages |
| `CapPaymentRequest` | 2 | Sign account payment requests; never spend or approve |
| `CapPolicyWrite` | 4 | Write the account's private allow/block policy |
| `CapAssessment` | 8 | Publish account trust assessments |
| `CapProfileUpdate` | 16 | Sign a community-profile update contribution |

A grant needs a nonzero subset of 31. No implicit permission hierarchy or delegation
by a device. Enrollment/revocation belongs to the controller; wallet spending is a
separate native authority. Assessment/profile permission does not appoint a source,
establish independence or satisfy the trust profile's quorum. Future call, history-sync
and social-publication contracts must define their own mapping; do not infer it here.

## Public Go contract

Export these names and signatures; implementation helpers remain private:

```go
type AccountID [32]byte
type GrantID [32]byte
type Capability uint64
type Kind byte
// Kind constants: Grant=1, RevokeGrant=2, RevokeDevice=3.
// Capability constants and MaxRecordBytes are specified above.
// MaxKnownRecords = 4096.
type Record struct { /* unexported fields only */ }
type KnownState struct { /* unexported fields only */ }

func AccountIDFor(controller [32]byte) (AccountID, error)
func VerifyRecord(raw []byte) (Record, error)
func (r Record) Kind() Kind
func (r Record) AccountID() AccountID
func (r Record) GrantID() GrantID
func (r Record) DeviceKey() [32]byte
func (r Record) Capabilities() Capability
func (r Record) Bytes() []byte
func NewKnownState(controller [32]byte) (*KnownState, error)
func (s *KnownState) Apply(raw []byte) error
func (s *KnownState) AuthorizesKnown(grant GrantID, device [32]byte, required Capability) bool
func (s *KnownState) Saturated() bool
```

`AccountIDFor` and `NewKnownState` reject the all-zero controller. A valid Record's
AccountID derives from its included controller. GrantID returns the derived grant ID
for Grant, the target for RevokeGrant, and zero for RevokeDevice. DeviceKey returns
the bound/target device for Grant/RevokeDevice and zero for RevokeGrant. Capabilities
is zero for revocations. A zero Record has zero-valued accessors and nil Bytes.
Bytes returns an owned copy; input mutation or mutation of a returned copy cannot
change a Record, its getters or a state's decisions.

### Known-state rules

1. State is pinned to one controller. Apply performs complete VerifyRecord verification
   and then checks that exact controller; a valid record from another account is an
   error. Invalid/foreign input never changes state or consumes capacity.
2. Keep grants by GrantID, grant-revocation tombstones by target GrantID, and
   device-revocation tombstones by target device key. Record identity for counting is
   `(kind, grant ID or target)`, independent of signatures. Exact duplicate/replayed
   facts are successful no-ops after full verification. Keep revoked grants/tombstones;
   do not erase facts to make room or because the grant has not arrived yet.
3. AuthorizesKnown is true only for a stored grant bound to the exact supplied device,
   when all nonzero requested bits are included in that **one** grant and neither
   the grant nor device is revoked. Zero/unknown requested bits, unknown grants,
   wrong devices, nil/zero states and saturated states return false. Never union
   different grants to satisfy a compound request. A zero/nil state rejects Apply.
4. A device tombstone wins over every grant for that key, including newly issued
   grants and grants arriving after revocation. A grant tombstone affects that grant
   only. No un-revoke operation or clock-based winner. A new grant for an otherwise
   unrevoked device can authorize it; after device revocation use a new device key.
5. Store at most **4096 distinct verified records total**, across the three kinds.
   Duplicates remain no-ops at the limit. On the first additional distinct, valid,
   same-account record, set a permanent saturated flag, retain the existing facts,
   and return an error. All authorization then fails closed. Saturated state keeps
   rejecting new facts and has no reset API; replaying stored facts may return nil
   but cannot clear saturation. Invalid/foreign input must not trigger saturation.
   This bounds a local verifier, not the future network intake/rate policy. The
   integration must persist saturation and provide explicit recovery/version handling;
   it cannot silently start an empty state to regain access.
6. Apply is an explicitly single-owner mutation API; no internal goroutines or I/O.
   It is not safe concurrently with any access; callers later serialize it. After
   mutation stops, concurrent reads do not modify state. Do not add a cache, global
   registry, background refresh, snapshot replacement or wall clock.

All permutations of the same valid record set below the bound give the same final
authorization decisions. More than the bound gives denial in every order. Intermediate
decisions can differ before a revocation is received. This is **known-state authority**,
not proof of latest global state or acceptance of a signed message/payment. Grants
do not expire in this first profile. Durable replay, revocation discovery and freshness
are mandatory follow-on integration work; do not describe this package as solving them.

## Required test source

Use synthetic, reproducible keys only. Test helpers independently assemble the fixed
byte offsets and call standard Ed25519 signing; do not ask a production encoder or
production domain constant to tell the tests what the expected bytes are. Include a
literal payload fixture and independently specified account/grant hash preimages;
compute expected digests with standard SHA-256 directly over those literal preimages,
not production encoding helpers. Include a cited public RFC 8032 vector to check
the signing fixture's setup.
Keep fixtures inside the three authorized test files. Sol must not execute a generator.

Cover these invariants with non-vacuous positives and focused negative mutations:

- Valid grants for two devices under one account; all three record kinds verify.
  Wrong controller/device signature, substituted account/device/permissions/nonce,
  swapped signatures, missing proof, cross-kind/domain and signed-digest substitution
  fail. Use correctly re-signed malformed payloads too: invalid schema must not pass
  simply because signatures are correct.
- Lengths just below/at/above 130 and 234; unknown kind/version; nil/empty/oversized
  input; all-zero reserved values; root=device; capability zero, 31 and unknown bits.
  No panic and no partially populated result. Do not test only random invalid bytes.
- `TestKnownStateRevocation`: a grant first authorizes; grant revocation denies only
  it; device revocation denies all its grants before/after replay and re-grant;
  a second device remains authorized. Revocation before grant has the same result.
  A newly keyed replacement under the same account works.
- `TestKnownStateCapabilities`: each permission separately, combined-bit requests,
  wrong-device binding, no union across grants and no authority from unknown grants.
  No automatic elevation from message permission to the other four permissions.
- Idempotence and permutations, cross-account rejection without mutation, zero/nil
  state, input/output aliasing and non-mutating reads. Capability reduction explicitly
  proves old grants remain active until separately revoked.
- Boundaries at 4095/4096/4097 distinct facts, duplicate at capacity, invalid/foreign
  extra input, valid overflowing revocation and permanent saturation. Exercise a mix
  of all record kinds so a per-map count bug cannot pass. No fixture mutation that
  bypasses signature checks just to reach the limit.
- Native fuzz targets `FuzzVerifyRecord` and `FuzzKnownState`: seed all valid kinds and
  meaningful corruptions; check no panic, owned copies, deterministic verification
  and no invalid-input state mutation. State fuzzing limits each input to a small,
  fixed number of attempted records; do not generate thousands of signatures per
  iteration. Keep discovered regressions inline in the authorized test files.

No filesystem, crash, multi-node or live transport test is relevant to this isolated
package. Do not build that infrastructure here. Do not substitute tests for the future
durability/freshness integration requirements.

## Phase sequence and execution contract

**Now:** review 07 verifies publication and bounds the missing secret-scan evidence.
The source remains frozen at review 04 identities.
No owner transcription or invented execution result. Sol's role excludes repository-
record ownership; Hermes owns the single execution report
`docs/testing/BBGO-ACC-001-EXECUTION-01.md`.

Hermes completes review 07's scan evidence and report closeout, using the same ticket
and report. Review transitions update this ticket and CURRENT_TASK; do not create
separate documents for every command. Only review 07's completion phase is active.
Its exact path set and staged secret gate govern publication.

Hermes first records tool identities, clean/dirty inventory and all ten authorized
source hashes. Use Go 1.27.0,
`GOWORK=off`, and a task-owned disk-backed cache/artifact directory
under `modern/dist/acc001/`, after checking that repository filesystem type with
`stat -f -c %T .`. Do not put substantial caches in RAM-backed temporary storage.
Record actual expanded paths locally in the evidence, without publishing local
absolute paths. Capture exact cwd/argv/environment, stdout, stderr and exit status.
Offline tests may not acquire credentials or depend on public services.

From `modern/`, red and green use the **same** command:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountauth -count=1
```

Initial red is the missing public implementation in the new package (undefined
contract names), not a malformed test/helper/import. This compile-time red cannot
prove behavioral sensitivity; the later falsification below supplies that evidence.
After reviewed production, the command must pass without skips or replacing tests.

Green acceptance from `modern/`:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test -race ./accountauth -count=1
env GOWORK=off GOTOOLCHAIN=go1.27.0 go vet ./accountauth
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountauth -run '^$' -fuzz '^FuzzVerifyRecord$' -fuzztime=30s -parallel=2
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountauth -run '^$' -fuzz '^FuzzKnownState$' -fuzztime=30s -parallel=2
```

One high-value falsification: temporarily bypass only the device-tombstone check in
AuthorizesKnown; `go test ./accountauth -run '^TestKnownStateRevocation$' -count=1`
must fail on a formerly authorized device remaining active after revocation. Reviewer
will pin the exact expression to change after source review, before executor authority.
Hermes does not design a fault patch. Restore the original production bytes, verify
the restoration hash, and rerun the targeted test. Never commit the fault. No second
falsification project or unrelated full-daemon test sweep is required for an unimported
standard-library-only package.

Security gates after reviewed source (not authorized for Sol): verify scanner module
provenance/version; gosec v2.29.0, govulncheck v1.7.0, Gitleaks v8.30.1. Network use is
limited to obtaining these exact tools and official vulnerability data; tool installs
must use the task-owned disk-backed tools directory and not modify go.mod/go.sum.
From `modern/`, after setting the same Go environment:

```sh
gosec -tests ./accountauth/...
```

From the repository root:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 python3 scripts/govulncheck_policy.py source
gitleaks git --pre-commit --staged --redact=100 --no-banner .
```

The staged scan waits for the separately activated final publication phase. No new
unreviewed finding, secret, unknown scanner failure or bypass is acceptable. Existing
findings need their retained baseline/rationale; severity alone does not dismiss a
plausibly exploitable issue. Do not add suppressions or baseline entries here. Record
failures and stop dependent work for reviewer adjudication, without silently widening
scope. No release/SBOM or binary refresh is needed: the new package is deliberately
not imported by bitbookd, and the running executable gains no behavior from this slice.

## Phase record

Initial authorization, 2026-09-16, Codex: authority choice and first contract frozen
after source/specification review. The six target source files are absent. Only Sol's
test-source phase is active.
No tests, security scans, builds, actor launches or runtime integration performed.
Future source identities, execution evidence and acceptance must be recorded from
actual files/results here and in Hermes's named report; this entry claims none.

### Test-source review 01 — historical correction, source phase closed

2026-09-16, Codex. Owner confirms reviewer effort High and directs continuation.
Reviewed the complete three-file drop against this contract and TESTING.md at daemon
HEAD `27571cd65b18ff59598883f79f2555d7027fdb0b`. No production files are present and
all four frozen source-input hashes still match. The index was empty. Source inventory:

| Test path under modern/accountauth/ | Lines | Top-level entry points | SHA-256 |
| --- | ---: | --- | --- |
| records_test.go | 472 | 9 tests | 18c6dcaa57cc3d772bf232428e5f66f01d4f324d225bd8dda57b1f45c93e95ce |
| state_test.go | 429 | 8 tests | 82ca2eb49baaa72d468784e1352a3d92b9ae77a55f3f30180f0a847534a98473 |
| fuzz_test.go | 265 | 2 fuzz targets | 02678cea8f776d8f4d0b6894c179afc5470e8cbb1364ae59de35e4d956e09a7f |

Total: 1166 source lines, 17 top-level tests and two fuzz targets, not an executed
test count. Two hash reads matched during review. The drop includes independent
fixture bytes/hash preimages, valid record cases, device replacement/revocation,
permission separation, 120 record-order permutations, capacity boundaries, owned-byte
checks and bounded fuzz loops. Retain this useful coverage.

**Disposition: not yet accepted for execution.** These are test-source coverage
findings, not reproduced defects in a production implementation (none exists yet).
Complete this single bounded correction in the same three authorized test files:

1. **R1 — authenticate both revocation kinds explicitly.** The signature/binding
   mutation table in `records_test.go:300` covers grants only. The revocation negative
   cases at lines 356–375 cover zero targets and one RevokeGrant domain mismatch;
   no direct case rejects a forged signature on an otherwise well-formed RevokeDevice.
   For each revocation kind, start with a positively verified record, then reject a
   corrupted/zero signature, a signature made by the device instead of the declared
   controller, a changed nonzero valid target retaining the old signature, and the
   other revocation domain. Assert zero Record on each failure. Keep keys/targets,
   kind, version and length valid so schema rejection cannot mask the crypto check.
   Add forged-revocation seeds for both kinds to the existing fuzz coverage. Fuzz
   determinism and calling VerifyRecord as the state fuzzer's oracle do not replace
   these independent known-invalid assertions.
2. **R2 — reject another account's revocations without changing local authority.**
   `state_test.go:248` and the state fuzz seeds currently use foreign grants only.
   Add validly signed foreign-controller RevokeGrant and RevokeDevice records whose
   targets are a live local grant/device. Prove VerifyRecord accepts their signatures,
   then Apply rejects their account and preserves local authorization. Also reject
   the forged local-controller revocations from R1 without changing state. Retain a
   positive local-controller revocation path that actually removes authority. Seed
   the state fuzzer with the valid foreign revocations as well.
3. **R3 — test a valid permission change and pin the wire bits.** The capability
   mutation at `records_test.go:329` XORs 0x80 into the mask, setting an unknown bit.
   It can fail schema validation without proving signature binding. Add a change
   between two nonzero allowed masks while retaining old signatures; also exercise
   the changed payload with only the controller signature renewed and with only the
   device signature renewed. Both incomplete re-signings must fail; renewing both
   for that allowed mask must succeed. Assert the five named capability constants
   equal 1, 2, 4, 8 and 16 respectively; tests using only the same symbolic constants
   on both sides cannot detect a wire-level permission swap.
4. **R4 — count retained and repeated revocation facts at the existing limit.**
   `TestKnownStateRecordLimit` currently uses an absent grant target and repeats only
   a grant at capacity. Adjust this existing fixture to revoke an already stored,
   non-probe grant; the original grant and its tombstone must consume two facts.
   Keep the unaffected first-grant probe and all three record kinds. Replay both
   tombstone kinds at capacity without saturation. Attempt invalid and foreign
   records just below capacity, then prove the last distinct valid fact still fits.
   Preserve the valid overflowing-revocation and permanent-denial checks. No new
   large fixture or separate saturation test project is needed.

Sol High may amend only the existing three test files from the hashes above. No
production stubs, test execution, formatter execution, dependencies, records or Git.
Do not replace the existing suite, expand the protocol or create another handoff.
Return a pointer to this ticket and the corrected files; Codex reads them directly
and records the new inventory before activating Hermes. No owner log transcription.
The previously specified red/green/falsification/scans remain future phases, not
execution authority. Reviewer work here was read-only source inspection and hashing;
no test, compiler, fuzz, formatter, security scan or runtime command was executed.
Review-document checks: inspected the two-file diff; scoped `git diff --check`
returned exit 0, all 37 local document links resolved, and the final source inventory
still matched all three test hashes with the three production targets absent.

Reviewer governance publication scope for this authorization is exactly this ticket
and `docs/handoff/CURRENT_TASK.md` in bb-go, plus the six desktop documents enumerated
in the [research scope](../../bb-desktop/docs/architecture/BB-TRUST-RESEARCH-SCOPE-01.md).

### Test-source review 02 — accepted for bounded expected-red execution

2026-09-16, Codex. Reviewed all three corrected files at daemon HEAD
`cd8bd1e856f41c3857bd0dc4a2de92078a824817`. Frozen source inputs still match the four
hashes above; production remains absent. This is source acceptance for the missing-
implementation red, not a test pass or production acceptance.

| Test path under modern/accountauth/ | Lines | Top-level entry points | SHA-256 |
| --- | ---: | --- | --- |
| records_test.go | 564 | 10 tests | 11d8cbafd031454d176ef743480f071f4e674920d445a49e25ae3e7a459488ad |
| state_test.go | 525 | 8 tests | 6145fe6183ed258b86482da8ef6d4338720fe3911c51032a7d9aed2eca08842d |
| fuzz_test.go | 277 | 2 fuzz targets | 08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833 |

Total: 1366 lines, 18 top-level tests and two fuzz targets. Counts are static source
inventory, not executed cases. Findings disposition:

- R1: both revocation kinds have valid controls followed by corrupted/zero signatures,
  device-key forgeries, changed valid targets and wrong domains. Failures require
  zero Records. Both forged kinds are fuzz seeds.
- R2: state tests positively verify foreign-signed revocations, then require Apply
  rejection while two local grants remain authorized. Forged local revocations are
  rejected too. Valid local revocations subsequently remove the intended authority.
  Foreign revocation fuzz seeds include sequences with a live local grant.
- R3: changed permissions stay within the valid mask; stale proofs and each single-
  signature renewal fail, while renewing both succeeds. All five wire bits are pinned.
- R4: the bound fixture retains an existing grant and its separate tombstone, repeats
  both revocation kinds at capacity, rejects invalid/foreign input just below capacity,
  then fits the last valid fact and denies permanently on the next valid revocation.

One small remaining source obligation: the R4 edit moved the earlier invalid/foreign
checks below capacity. Restore those same rejection/non-saturation checks **also at
exactly 4096**, after the duplicate checks and before the valid overflow, during the
next reviewer-authorized source phase, before production authoring. Reuse the current
invalid/foreign fixtures; no new suite or fixture is needed. This catches an early
capacity guard that saturates on unverified traffic. It does not prevent capturing
the existing missing-implementation red; it must be present before green acceptance.
Sol has no present authority to make this change. The production-phase activation
will include it, preserving the normal test-first/source-review sequence.

#### Hermes execution authority — expected red only (closed by review 03)

Use this section directly; no new handoff document. Record the actual Hermes version,
provider and model. Verify the seven input hashes (three corrected tests plus four
frozen source inputs), the absence of `types.go`, `records.go`, `state.go`, and current
Git status/HEAD. The three tests already exist in this working tree; no copy, patch,
stub, formatting, staging or commit is needed. Unexpected inputs stop execution for
review. Earlier Sol correction permissions are closed.

Writable scope is exactly:

- `docs/testing/BBGO-ACC-001-EXECUTION-01.md`, created or appended without overwriting
  any existing evidence;
- task-owned captures under `modern/dist/acc001/red01/`;
- task-owned compiler cache and temporary files under `modern/dist/acc001/go-cache/`
  and `modern/dist/acc001/go-tmp/`.

First inspect the repository filesystem with `stat -f -c %T .`. Use these disk-backed
directories only; do not move large work to RAM-backed temporary storage. Configure
the named directories as needed without altering other task artifacts. Set
GOCACHE and GOTMPDIR to their resolved paths in the child process environment, and
GOPROXY=off/GOSUMDB=off for this offline phase. Do not change persistent Go settings.
Use the already installed/cached Go 1.27.0; unavailable tooling is a setup failure,
not permission to install tools, download dependencies or claim the expected red.

From `modern/`, capture the following exact commands with the above environment:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go version
env GOWORK=off GOTOOLCHAIN=go1.27.0 go env GOVERSION GOWORK GOCACHE GOTMPDIR GOPROXY GOSUMDB
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountauth -count=1
```

Run the test command once, bounded by 300 seconds in the capture runner. Expected:
nonzero compile failure reporting the absent public contract names, such as Kind,
AccountID, GrantID, Capability, Record and KnownState. A parser/import/toolchain/cache
error or timeout is not the intended red. Do not repair failures, add temporary
production declarations, run a copied substitute package or proceed to green.
Compiler diagnostic truncation does not prove absence of other compile issues;
record the actual output. No test bodies are expected to run yet, and the report
must not describe this as behavioral regression evidence.

Retain separate stdout/stderr captures, exact argv/cwd/environment, exit status and
before/after hashes, with source line/top-level test counts. The repository report
must link the relative capture paths and distinguish setup, expected compile failure,
unexpected failures and unexecuted future gates. Keep local absolute paths out of
published text. Finish by checking all seven hashes and production-file absence
again. No source mutation, scan, fuzz campaign, live daemon/wallet, Git publication
or actor launch is authorized. Hermes stops with the report pointer; Codex reviews
the retained evidence and activates the next source phase in this same ticket.

Reviewer publication for review 02 is limited to this ticket and
`docs/handoff/CURRENT_TASK.md` in bb-go, and `docs/handoff/CURRENT_TASK.md` in
bb-desktop. Baselines: daemon `cd8bd1e856f41c3857bd0dc4a2de92078a824817`, desktop
`9e6834bbf7507d07d90ba31c2472aa854b6b5ec2`. Validate scoped diffs/links/whitespace and
commit each repository separately. The reviewer does not integrate the test files,
create execution evidence or execute the authorized Hermes commands.

Review 02 documentation checks: 64 local links resolved across the three scoped
documents; scoped `git diff --check` exited 0 in both repositories. A second source
inventory matched all three corrected test hashes and counts; production files and
the execution report remain absent. No implementation, compiler or test execution
is claimed by this review.

### Red review 03 — narrow red accepted; historical source authorization

2026-09-16, Codex. Baseline HEAD `0cda9ae54ea26332143bb9f1cc9d6a350ffb9101`.
Read the retained Hermes report and both raw captures. The captures show package
compilation failing on the missing public contract names. Diagnostic locations,
including `fuzz_test.go:120`, `:122`, `:130` and `records_test.go:84`, `:102`, match
the current pinned test source. All seven source hashes match the pre-execution
contract; the three production files remain absent. The index is empty. No test
bodies ran and no behavioral pass/fail or security acceptance is established.

Evidence inspected (paths relative to repository root):

| Artifact | SHA-256 |
| --- | --- |
| docs/testing/BBGO-ACC-001-EXECUTION-01.md | 5cfb361d072f13fcb8290b35397a5dcfedbb75b3b7ada7a22e86128aa828be73 |
| modern/dist/acc001/red01/test.stdout.log | ec4aef79f721e72766855bbde69d5b2231e08f493b697bb6fe17c54060b8fd18 |
| modern/dist/acc001/red01/test.stderr.log | 34746378cbaf7e50f36167040549db87d8edb23a65000e1a56d341fc490e37df |

**Evidence limits and required report repair at the next Hermes phase:**

- The report gives 228 lines for `modern/network/identity.go` and 443 for
  `modern/payment/signature.go`; their actual unchanged files have **95 and 156**
  lines. The other five source line counts match. Test inventory remains 18 top-level
  tests and two fuzz targets, not executed-case counts.
- Only the test stdout/stderr files are retained in red01. There are no retained
  go-version/go-env captures, per-command exit/environment metadata or separate
  before/after source manifests. Exit 1, tool/provider/version, filesystem and
  execution-environment details are actor-reported, not independently established
  by those two logs. The report omits GOTOOLCHAIN/GOSUMDB and abbreviates cache paths;
  it also includes a local absolute cwd despite the publication rule.
- Hermes must correct the counts, use repository-relative presentation, link the
  captures and distinguish its reported claims from retained evidence in the same
  report before final acceptance/publication. Recover actual original metadata if
  available; otherwise label it unavailable. A later fresh environment capture must
  not be represented as evidence of the earlier run. Never reconstruct missing
  execution records. The next green phase must retain full command metadata and
  before/after identities as already required by the contract.

Reviewer disposition: accept **only the intended missing-implementation compile
failure**, supported by the retained diagnostics and matching source. The reporting
defects do not require a second red run or a record-only handoff before authoring this
isolated implementation. They remain open evidence corrections for Hermes's next
phase. This is not full execution-provenance acceptance. The reviewer neither edits
Hermes's implementation evidence nor reruns the command. Red execution is closed.

#### Sol production-source authority (closed by review 04)

Use `gpt-5.6-sol`, High. Verify the four frozen source inputs and three review-02
test hashes above before edits; all three production files must initially be absent.
This is one source drop, in this order:

1. In `modern/accountauth/state_test.go`, restore the invalid/foreign-input checks
   at exactly 4096 records in `TestKnownStateRecordLimit`, after duplicate replay
   and before the valid overflow. Reuse the existing `invalid` and `foreign` inputs.
   Each Apply must return an error without saturation or loss of the first/last
   valid grant's authority. Retain the below-capacity checks and every existing
   regression. Do this before production source. No extra red run is authorized;
   the absent implementation and its compile failure have already been established.
2. Author `modern/accountauth/types.go`, `records.go` and `state.go` to implement
   this ticket's frozen format, exported API and known-state semantics. Keep
   implementation helpers private and dependencies limited to the standard library.
   Use the exact signature domains/full payloads, independent controller/device
   verification and owned bytes. Keep invalid/foreign validation ahead of duplicate
   and capacity handling; only a distinct valid same-account overflow saturates.
   Preserve revocation facts, one-grant permission checks and nil/zero failure rules.
   No clock, persistence, signing/key custody, transport, wallet or existing-package
   integration. Do not import this package elsewhere or modify dependency files.
3. Mechanical formatting of all six package files is authorized, including the
   following command from `modern/`:

   ```sh
   gofmt -w accountauth/types.go accountauth/records.go accountauth/state.go accountauth/records_test.go accountauth/state_test.go accountauth/fuzz_test.go
   ```

   This is a source-editing operation, not test/compile execution. `records_test.go`
   and `fuzz_test.go` permit formatting-only changes; their semantics and fixtures
   remain frozen. `state_test.go` permits only the restoration above plus formatting.

No other writable paths. No compiler/test/fuzz/scan/build commands, tooling installs,
Git mutations, repository-record edits or actor launches for Sol. Preserve Hermes's
report/captures and unrelated work. Do not weaken tests to fit production, add test
stubs, alter capabilities/domains/bounds, or expand the API. Report a discovered
contract conflict before changing semantics.

Stop with a pointer to this ticket and the six source files. The reviewer reads the
drop directly, records actual source identities and checks the test restoration and
formatting-only restrictions. Hermes receives green/falsification/security authority
only after that source review, including correction of its retained report. Source
completion is not an execution result. No owner log transcription is requested.

Reviewer publication for review 03 is exactly this ticket and
`docs/handoff/CURRENT_TASK.md` in bb-go. No source/test or Hermes report integration
is included. Preserve the unchanged desktop routing, which already follows this
ticket's current phase. Verify scoped document diffs, local links and whitespace;
commit only these two governance paths from the baseline above.

Review 03 document checks: scoped `git diff --check` exited 0; all 37 local links
resolved. The final inventory matched seven source hashes and three evidence hashes,
with all production targets still absent. These were read-only review/document
checks, not compiler, test or scanner execution.

### Source review 04 — accepted for bounded Hermes execution

2026-09-16, Codex, High. Reviewed the complete production drop at daemon HEAD
`3b809c563067ee8e1e0fc4326bd31abbf4bc9b66`, against the frozen format and state rules.
The four original source inputs still match their hashes above. Source inventory:

| Path under modern/accountauth/ | Lines | SHA-256 |
| --- | ---: | --- |
| types.go | 78 | ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb |
| records.go | 177 | cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640 |
| state.go | 122 | b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1 |
| records_test.go | 564 | 264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9 |
| state_test.go | 545 | 949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed |
| fuzz_test.go | 277 | 08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833 |

Total 1763 lines: 377 production and 1386 test lines. Static entry-point inventory
remains 18 top-level tests and two fuzz targets; no executed test count is claimed.

Source findings:

- Exact framing precedes indexing. The domains, byte offsets, full-payload Ed25519
  proofs, capability mask and reserved-value checks match the contract. Failure
  paths return a zero Record; successful Records own their bytes.
- State verifies signatures and account before duplicate/capacity handling, retains
  all three fact kinds, and permanently denies after valid novel overflow. Grant
  and device tombstones, exact device binding and single-grant capability checks
  match the contract. Nil/zero states fail closed.
- Compared tests with review 02: records_test.go differs only in formatting;
  fuzz_test.go is byte-identical. state_test.go adds exactly the requested 20 lines
  checking invalid/foreign input at full capacity, preserving below-capacity checks,
  non-saturation, and the first/last grants' authority. Existing regressions remain.

Disposition: source accepted for execution, not runtime or security acceptance.
The package remains isolated and unimported. Sol's source-editing phase is closed.

#### Hermes execution authority — green, one fault, fuzz and security (closed by review 05)

Use this section directly and the existing report
`docs/testing/BBGO-ACC-001-EXECUTION-01.md`. Verify the six review-04 package hashes
and four original input hashes before any execution. Record current HEAD/status;
preserve all unrelated work and the red01 captures. Unexpected source identities
stop execution for reviewer disposition.

Writable scope:

- The existing execution report, with review 03's reporting corrections and a new
  green section; preserve the historical results and disclose corrections.
- Captures and an optional bounded capture runner under `modern/dist/acc001/green01/`;
  compiler cache/temp under `modern/dist/acc001/go-cache/` and `go-tmp/`; pinned
  scanner tools under `modern/dist/acc001/tools/`. Native fuzz cache artifacts may
  reside in the authorized cache; preserve any failure input under green01 for Sol.
- Only the exact temporary state.go predicate change below, followed by byte-exact
  restoration. No other production/test edits, formatting or new regression source.

First check filesystem type as specified above. Retain Go version and go-env output,
actual Hermes version/provider/model, and before/after manifests for all ten inputs
(hashes, line counts, static entry-point counts). For every command retain separate
stdout/stderr and metadata containing exact argv, cwd, effective Go/tool environment,
start/end timestamps, exit status and timeout status. Record only relevant environment
variables, never a wholesale environment dump that might contain credentials. Raw
ignored captures may retain actual local paths; the published report uses relative
paths and links. Capture-file hashes belong in the report. Use a capture runner with
a 600-second bound per test/vet/fuzz command and 1200 seconds per scanner/install;
timeout or setup failure is not a test result.

Use Go 1.27.0, GOWORK=off, process-local GOCACHE/GOTMPDIR resolving to the named
directories, and GOPROXY=off/GOSUMDB=off for tests. Capture these first from modern/:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go version
env GOWORK=off GOTOOLCHAIN=go1.27.0 go env GOVERSION GOWORK GOCACHE GOTMPDIR GOPROXY GOSUMDB GOFLAGS
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountauth -count=1
```

After that green succeeds, perform the one pinned falsification. Back up the exact
state.go bytes under green01 and verify its review-04 hash. In AuthorizesKnown,
replace exactly one occurrence of:

```go
if _, revoked := s.revokedDevices[device]; revoked {
```

with:

```go
if _, revoked := s.revokedDevices[device]; revoked && false {
```

Save the one-line diff and fault-source hash. Run from modern/, in the same environment:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountauth -run '^TestKnownStateRevocation$' -count=1
```

It must fail because a device's grant remains authorized after device revocation;
compile/setup errors do not count. Restore original bytes in guaranteed cleanup even
on failure, interruption or timeout, verify the original hash, then rerun that exact
target expecting success. If the fault is insensitive or restoration fails, stop
dependent work and report it. Never run broader checks on the faulted source.

After restoration, execute the four previously enumerated green acceptance commands
(race, vet, FuzzVerifyRecord and FuzzKnownState), once each. Then run the previously
enumerated gosec v2.29.0 package scan and govulncheck v1.7.0 source-policy command.
Verify scanner binary hashes and module provenance using `go version -m`; use the
verified binaries via process-local PATH. Reuse installed pinned tools when available.
If absent, obtaining exactly these versions is authorized, with GOBIN set to the
resolved task tools directory:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go install github.com/securego/gosec/v2/cmd/gosec@v2.29.0
env GOWORK=off GOTOOLCHAIN=go1.27.0 go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
```

Only tool acquisition and official advisory access may use the network, under the
earlier security contract. Do not alter persistent Go settings, dependency files or
scanner policy. The existing govulncheck policy covers modern's resolved source,
including its reviewed exception; retain the actual result and findings. New findings
or unknown scanner failures require review. Gitleaks remains deferred to the later
enumerated staged-publication phase; do not stage files to run it now.

In the same execution report, complete review 03's corrections: identity.go is 95
lines and payment/signature.go is 156; distinguish retained red diagnostics from
actor-reported metadata, mark unavailable historical records explicitly, and remove
local absolute paths from published prose. Fresh captures establish this run only.
No reconstructed historical metadata or repeat red is requested.

Finish with ten matching restored source hashes, captured outcomes and remaining
gaps in the report. No live daemon/wallet, full-daemon test sweep, binary rebuild,
SBOM, source integration/staging/commit/push or additional actor is authorized.
Codex reviews that evidence before final acceptance and scoped publication. Hermes's
completion message is a pointer to the report; the owner need not transcribe results.

Reviewer governance publication for review 04 is exactly this ticket and
`docs/handoff/CURRENT_TASK.md` in bb-go, from the HEAD above. No source or execution
report is included; desktop routing already follows this ticket. Validate scoped
diffs, links, whitespace and source identities before publishing those two documents.

Review 04 document checks: inspected the two-file diff; scoped `git diff --check`
exited 0 and all 37 local links resolved. All ten source hashes and three retained
red-evidence hashes matched; the index was empty before governance staging. These
were read-only source/document checks, not test, compiler or scanner execution.

### Execution review 05 — behavioral evidence accepted; security gate incomplete

2026-09-16, Codex, High. Baseline HEAD
`7b3cbf61363b7fb580ced65028143c45231398d4`. Read the execution report and every
green01 log. All ten source hashes still match; state.go and its saved backup match
byte-for-byte. The index is empty. No production defect was found in this review.

Retained evidence, paths relative to `modern/dist/acc001/green01/` except the report:

| Artifact | SHA-256 |
| --- | --- |
| docs/testing/BBGO-ACC-001-EXECUTION-01.md | c9e2e81230998435e19008bd9871bc6be883280857fc2b7f5dd611ec839879ef |
| test.stdout.log | d7e4b2a499cbb810c3a1f5dbcfcab71a1118a1bd7c6b366224e4dc4d166214ac |
| race.stdout.log | dcae0d39ed1800d85c5b8658f5a85a94672dcd0af4ad9f63061eb1f9252216f5 |
| fuzz-verify.stdout.log | f951892e97dd5457a6fb83ce289fb884c118a2d29657e76809062f359a607fc6 |
| fuzz-state.stdout.log | 91b031d2bd243069c43623db1b2735fd506d35ea455541cf55e06efc05e80dc0 |
| fault.stdout.log | 373ed48017f716d035193e5e8756b82ffe492194c4a83621614b0dd42eb058cb |
| restored.stdout.log | df8e8a21e6ed7d5e53e653beeea948786ccf98a89e3ec818d9bf271ee5933ada |
| gosec.log | bcaed611362d3d87d142e29fb44cd916757b70474a4e3fc2d315c89588378564 |
| gosec2.log | 0e400f6131121bf3e316215935f5b9ab06888be1da827e6165760524ebf3c863 |
| govulncheck.log | 3016e51e4eac0d421674d2128bbbdefb2924b4646e0c14a1ab034977ad73fae5 |

The logs show ordinary and race package passes, successful 30-second fuzz campaigns
(1,937,477 record-verifier executions; 426,538 state executions), the intended fault
failure at state_test.go:66, and a restored targeted pass. Accept these qualitative
behavioral results with the provenance limits below. No repeat race, fuzz or fault
is justified for unchanged source. Vet's two empty logs alone do not establish its
exit status; capture it properly with the remaining scan.

**Outstanding execution/report defects:**

1. Hermes reports `govulncheck -mode source ./accountauth/...`, not the authorized
   repository policy command. The one-line clean result is not evidence of the
   required whole-modern source/test scan or its exception adjudication. This gate
   remains unexecuted as specified, regardless of the report's all-passed verdict.
2. green01 again lacks command metadata, captured Go environment/version, separate
   before/after manifests, and the retained fault diff/hash. Source identity now and
   the matching backup do not prove the exact historical environment or ordering.
   Exit statuses and actor/environment details remain actor-reported. Do not invent
   missing metadata or represent fresh captures as historical proof.
3. gosec2.log records an additional failed `-include-tests` invocation, omitted from
   the report. The report also removes the earlier red command/result narrative,
   still contains an absolute tool path despite claiming none, and incorrectly says
   no source modification occurred during execution despite the temporary fault.
   Restore the historical red summary with review 03's limitations; disclose the
   extra failed scanner attempt and say no *lasting* source modification instead.

**Gosec disposition — G115 at fuzz_test.go:157:** reviewed, non-blocking for these
exact source bytes. packFuzzRecords is called only while constructing fixed seeds;
its inputs are 130-byte revocations, 234-byte grants, or one 235-byte malformed grant.
All fit uint16. The fuzz callback calls unpackFuzzRecords, never the packing helper,
so arbitrary fuzz bytes cannot reach the narrowing conversion. This call-site bound,
not production MaxRecordBytes, is the rationale. The helper is not production code.
No inline suppression or source change is needed. Owner: Codex reviewer. This
disposition expires if the helper, callers or fixture lengths change and must then
be re-reviewed; it does not authorize new findings or general test-file exclusions.

Reviewer read current scanner build metadata with `go version -m`, without executing
scans. gosec's embedded module is v2.29.0 despite its display label `dev`; its binary
hash matches the report. govulncheck's embedded module is v1.7.0. Current binary pins:

| Tool/input | SHA-256 |
| --- | --- |
| gosec v2.29.0 | eb00a1fb095b161a48c5bcadbe1e246bbafe270da497a122d2e63ade346954c2 |
| govulncheck v1.7.0 | 6c92f0536311f5e2083a839c75558e3fb986758a320a402aa8f524c85ffd7400 |
| scripts/govulncheck_policy.py | 709cb00d44c62ef6e2d394f457407183d6fb90bc98958c80db0261607bc3c77c |

These are current inspections, not retained historical scanner identities. The
gosec result plus matching reported binary and source is accepted with that limit;
no gosec repeat is needed. Final security acceptance/publication remains pending the
required policy result and later staged secret scan.

#### Hermes completion authority (closed by review 06)

Keep the six package files, four original inputs and policy script frozen at their
pins. No further fault injection or source edits. Write only the existing execution
report, new captures/optional capture runner in `modern/dist/acc001/finish01/`, and
the task cache/temp directories already authorized in review 04. Preserve red01 and
green01. Verify current source/tool pins before running. Use installed pinned tools;
no install is necessary. Earlier repeated-command authority is closed.

Run these five commands **once**, using a capture runner; cwd is modern/ except
the last command, whose cwd is the repository root:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go version
env GOWORK=off GOTOOLCHAIN=go1.27.0 go env GOVERSION GOWORK GOCACHE GOTMPDIR GOPROXY GOSUMDB GOFLAGS
env GOWORK=off GOTOOLCHAIN=go1.27.0 go test ./accountauth -count=1
env GOWORK=off GOTOOLCHAIN=go1.27.0 go vet ./accountauth
env GOWORK=off GOTOOLCHAIN=go1.27.0 python3 scripts/govulncheck_policy.py source
```

The short test/vet repeat establishes fresh captured execution; it does not replace
the existing race/fuzz/fault evidence. The policy script internally runs govulncheck
with `-format sarif -db https://vuln.go.dev -test ./...` in modern/. Do not substitute
a package-only invocation. Its output must be clean or exactly its already-reviewed
exception; any new finding/failure stops dependent work for review. Keep GOPROXY=off,
GOSUMDB=off, GOFLAGS empty, and resolved task GOCACHE/GOTMPDIR in the child environment.
Put the verified govulncheck binary's directory first in process-local PATH. Official
advisory access is authorized for the policy scan; tests remain offline. No persistent
settings or policy edits. Bound test/vet by 600 seconds and the policy scan by 1200.

**Required capture files, created by the runner during this execution:**

- `before.json` and `after.json`: HEAD, Git status, ten source hashes/counts, policy
  hash and two scanner binary hashes. Check source restoration/identity in finally.
- `01-version`, `02-env`, `03-test`, `04-vet`, `05-policy`: each gets `.stdout.log`,
  `.stderr.log` and `.meta.json`. Metadata records the actual argv array, resolved
  cwd, relevant child environment, UTC start/end, subprocess return code and timeout
  flag. Store the subprocess return code immediately; do not infer it from log text
  or a later shell `$?`. Write metadata even on errors/timeouts. Use no pipeline that
  hides the command's status. Stop on unexpected nonzero exit or timeout.
- `tools.log`: captured `go version -m` for both verified scanners. Record the actual
  actor/provider/model in the report; label any unretained historical claims as such.

A simple subprocess-based runner under finish01 is allowed. It must write the named
files itself, not rely on chat transcript retention. Inspect that all five metadata
files exist and contain real return codes before declaring this phase complete.
The report links the relative captures, gives their hashes and actual results, and
applies the three reporting corrections above plus the G115 disposition. Keep local
absolute paths only in ignored raw captures. Preserve unavailable old metadata as
an explicit limitation; do not recreate it. If original metadata can be recovered
from actual records, identify that origin.

No further test suite, race/fuzz campaign, gosec rerun, temporary fault, rebuild,
runtime service, Git stage/commit/push or actor launch. Return the updated report
pointer for reviewer acceptance and one scoped publication phase. No new handoff
document or owner-transcribed evidence is needed.

Reviewer governance publication for review 05 is exactly this ticket and
`docs/handoff/CURRENT_TASK.md`, from the HEAD recorded above. Preserve all untracked
source/evidence and unrelated dirty work. Validate document diffs/links/whitespace
and the source inventory before publishing these two documents.

Review 05 document checks: scoped `git diff --check` exited 0; all 37 local links
resolved. Ten source pins, the policy-script pin and reviewed evidence hashes match;
the saved backup equals current state.go. The reviewer inspected scanner build
metadata only and executed no tests, compiler, fuzz campaign or security scanner.

### Acceptance review 06 — execution accepted; scoped publication authorized

2026-09-16 local date (finish01 captures are 2026-09-17 UTC), Codex, High. Baseline
HEAD `d5fca0b693f64f9b316420a6daf3a57b16c8c5ad`. Read the report, capture runner,
five command metadata files, output/error logs, tool identities and Git snapshots.
All reported finish01 capture hashes match. All ten source inputs and the policy
script match their reviewed hashes; the index is empty.

Accepted results: Go 1.27.0, focused package tests, vet, and the required source-policy
command each have return code 0, no timeout and empty stderr. The policy reports only
the existing reachable exception GO-2024-3218 on DHT v0.42.2, expiring 2026-11-29,
and four non-reachable x/crypto notes: GO-2026-5932, GO-2026-6303, GO-2026-6354,
GO-2026-6355. The notes stay visible; they are not additional reachable findings
under the [existing policy](BBGO-SEC-001.md). This review does not create or extend
an exception. Prior race/fuzz/fault results and the G115 disposition remain as
accepted in review 05. No additional execution is needed before publication other
than the staged secret scan below.

Evidence identities (finish01 paths relative to `modern/dist/acc001/finish01/`):

| Artifact | SHA-256 |
| --- | --- |
| docs/testing/BBGO-ACC-001-EXECUTION-01.md | b36dd7939277df2792e64969b451cc3e8bcd07b926053d99870fafd8ac6f8018 |
| runner.py | 226487a8fc1038c75c154dbb78be197168d09d3278d71e0bf9e62d336c212c71 |
| before.json and after.json (identical) | f4a38195be5f9409e98fd6651cf90504e2c5cf701b4a440bdb38044ca0307b02 |
| tools.log | f5d8be6bd27ab19def6a3aa762fd66de9d69476ce86ab07aca5c2a59faaf2399 |
| 03-test.meta.json | a511c1fd8094e9a70c5294e6db0e9463216a0c4003c2bea3b8a4b7b2f7ec3904 |
| 04-vet.meta.json | 4de26344d28cf1275e98fbff952b62b8078d5c7068f37ef1942c60a4bf2272ee |
| 05-policy.meta.json | fbac18a5ad0451ccb96c49dc07277749efe85222d090ff6fc21e57496b4bbd01 |
| 05-policy.stdout.log | 05234cec3775b7be8a52ab2326ce83935c9f60cc4e78ae00d016dafd5e6bbf7d |

**Evidence limits accepted explicitly; correct the report during publication:**

- before/after.json contain HEAD/status only, not the requested source/tool hash
  manifests. Current hashes match the earlier reviewer inventory; do not claim
  those snapshots independently prove unchanged bytes throughout execution.
- The runner omitted GOSUMDB from both the environment override and go-env query,
  contrary to review 05. Its effective value is unrecorded. GOPROXY=off, Go 1.27.0,
  GOWORK=off and empty GOFLAGS are captured. There is no retained evidence that
  setting GOSUMDB=off caused a toolchain error; the runner's comment is not proof.
  This deviation does not invalidate the observed results on the unchanged isolated
  package, and does not warrant another run. Do not claim a fully verified offline
  environment or reconstruct the missing setting.
- PATH is not retained in command metadata, though the runner prepends the tool
  directory and tools.log records the pinned module versions. The runner also lacks
  fail-fast handling and final manifests on interruption and gives the policy a
  600-second bound. All five commands completed successfully; those unused failure
  paths did not affect this run. Do not reuse it as a general acceptance runner.
- Red/green provenance limits from reviews 03/05 still apply. Label the historical
  exit/environment claims as actor-reported where no metadata exists. Disclose the
  failed extra `-include-tests` gosec attempt and retain its existing log pointer.
- Replace the report's incorrect “Index: contains untracked files” with “index empty;
  working tree contains unrelated modified/untracked files.” Remove absolute tool
  paths from the published report. Include relative links to the captures and the
  ticket's findings dispositions; raw ignored artifacts may retain local paths.

Disposition: the isolated verifier is accepted for publication with these documented
limits. No current transport, wallet or UI integration is claimed. Report cleanup is
part of normal publication, not a separate execution/report-only phase.

#### Hermes publication authority — exact seven-file set (closed by review 07)

Use the same ticket and report. Verify the six package hashes from review 04 and the
four original input hashes. Start with an empty index; stop if unrelated staged work
appears rather than unstaging someone else's work. The next reviewer governance
commit may advance HEAD without changing source; record the actual starting HEAD.

Only these seven paths may be integrated/committed:

1. `modern/accountauth/types.go`
2. `modern/accountauth/records.go`
3. `modern/accountauth/state.go`
4. `modern/accountauth/records_test.go`
5. `modern/accountauth/state_test.go`
6. `modern/accountauth/fuzz_test.go`
7. `docs/testing/BBGO-ACC-001-EXECUTION-01.md`

All six Go files are byte-frozen. Only the report may be edited: apply the precise
corrections above, retain the command results and append publication evidence. No
AGENTS, .gitignore, dependency, workflow, ticket, CURRENT_TASK or cancelled DEV-001
changes. Do not add raw artifacts, binaries, caches, runners, private data or keys.
Read-only inventories and publication captures under `modern/dist/acc001/publication01/`
are authorized. Capture exact Git/scan argv, exits, output, staged paths/blob identities,
commit IDs and remote verification there, and summarize actual results in the report.

Use pinned Gitleaks v8.30.1, verify version/provenance and binary hash. Reuse a verified
existing binary if available. If unavailable, this pinned install is authorized with
GOBIN resolved to `modern/dist/acc001/tools/` and task-owned disk-backed caches:

```sh
env GOWORK=off GOTOOLCHAIN=go1.27.0 go install github.com/zricethezav/gitleaks/v8@v8.30.1
```

Tool acquisition may use the network; do not modify module files or persistent Go
settings. From the repository root, stage only the seven literal paths above, inspect
`git diff --cached --name-only` and `git diff --cached --check`, and run:

```sh
gitleaks git --pre-commit --staged --redact=100 --no-banner .
```

Require exit 0 and no findings. A secret finding, unknown scanner failure or changed
source hash stops publication for review; no exclusion/baseline/suppression changes.
Scan the final staged bytes after any report edits. Then commit the exact seven-file
set and push normally to origin/master, without force or unrelated integration.
Verify the remote contains the published commit and the committed Go blobs match
all six pins. Append the feature commit, push result and retained scan evidence to
the same report; a report-only closeout commit/push is authorized, with its final
staged secret scan too. Record that last commit/push in publication01 captures so
the reviewer can verify it without a self-referential report edit cycle.

No new tests/scans beyond this staged secret gate, source edits, runtime integration,
rebuild, restart or extra actor. Preserve all earlier captures. Finish with the report
pointer; the reviewer verifies publication and closes the ticket. No owner testing
or log transcription is required.

Reviewer governance publication for review 06 is exactly this ticket and
`docs/handoff/CURRENT_TASK.md`. The reviewer does not integrate source or edit Hermes's
report. Validate scoped diffs/links/whitespace and unchanged source pins, then publish
these two governance documents from the baseline recorded above.

Review 06 document checks: scoped `git diff --check` exited 0; all 38 local links
resolved. Source, policy, reviewed report and finish01 artifact hashes match. The
reviewer performed read-only evidence/document checks and no acceptance execution.

### Publication review 07 — commits verified; secret-scan evidence missing

2026-09-17, Codex, High. Verified through GitHub's remote-ref API that master is
`db8a1bf4f838ee5293fd3ada02f487344fafdf64`, directly following feature
`ad52bf019352b45bcf52906f99fcc4e776dc4b7e`. The feature contains exactly six accountauth
files and the execution report; the closeout changes only that report. All ten
reviewed source hashes match both commits and the working tree. No source correction
or repeat test is needed. [Go CI run 35186610615](https://github.com/larslarsen/bb-go/actions/runs/35186610615)
passed on the feature commit (compile, social runtime boundaries, maintained P2P
core). That workflow does not include a secret scan.

The committed report's SHA-256 is
`2b21f1735bfd092072ec83e48fdb255cd6cc844207c2217a581540641f2cb95d`.
The working copy is `09d16476c18ace490d856d1ae7c9f2d042cc9682bd4fce63e115851783ea9c42`;
its only uncommitted changes replace the pending closeout ID and update the remote
pointer. Preserve them for the report closeout below. No third source publication
is needed.

Publication01 contains only tree-manifest.txt, a list of eleven paths without blob
hashes. There is no retained Gitleaks output, return-code metadata, binary hash or
version/provenance capture, nor distinct evidence of the report-closeout staged
scan. The committed report's exit-0/no-findings sentence is actor-reported. It is
not independently verifiable, and passing Go CI does not supply this missing gate.
Do not describe the ticket as closed. Earlier source/execution acceptance stands.

#### Hermes final evidence authority — two scans, report only

Do not reconstruct an empty staged scan or reset/re-stage the published source.
Verify the ten source pins and the two published commits above. Use a fresh ignored
capture directory `modern/dist/acc001/publication02/`; the existing task tools/cache
directories remain authorized. The only tracked writable/stageable path is
`docs/testing/BBGO-ACC-001-EXECUTION-01.md`. Preserve all other work. No tests, fuzz,
gosec, govulncheck, builds, source changes or additional actor.

1. Locate Gitleaks v8.30.1 and capture binary SHA-256 plus `go version -m` provenance
   (or official release provenance/version). If unavailable, use review 06's exact
   pinned install into task tools, with captured install result. No exclusions,
   baseline/config edits or secret-scan environment overrides. Record active config
   identity or absence; do not dump secrets from the environment.
2. From the repository root, run exactly:

   ```sh
   gitleaks git --redact=100 --no-banner --log-opts=73b67c197f3b51941ddc345541ba5e36b56dfa76..db8a1bf4f838ee5293fd3ada02f487344fafdf64 .
   ```

   This scans both published commits. Reviewer checked the locally cached v8.30.1
   command source: `git --log-opts` is supported. Record it as a new committed-range
   scan, never as proof of the earlier staged run. Require actual exit 0 and no
   findings. Any finding/tool failure stops publication for review.
3. Correct the report in this same task: link this review and new captures; label
   the old staged result and historical red/green exit/environment claims as
   actor-reported where metadata is missing. In finish01's displayed go-env command,
   remove GOSUMDB, which the actual argv omitted; its effective value is unavailable.
   Retain review 06's accepted provenance limits rather than inventing evidence.
   Preserve the already-correct feature/closeout IDs from the working copy. Record
   the new range-scan command, tool identity, actual result and capture hashes.
4. Stage only that corrected report, verify the one-path index and whitespace, then
   capture its final staged scan:

   ```sh
   gitleaks git --pre-commit --staged --redact=100 --no-banner .
   ```

   Require exit 0/no findings. Commit and push only this report normally to
   origin/master. Do not edit it again merely to add its own commit ID. Save the
   final commit/push/remote-ref evidence in publication02 for reviewer verification.

**Completion requires actual capture files:** for each scan, `.stdout.log`,
`.stderr.log` and `.meta.json` containing argv, cwd, UTC start/end, direct subprocess
return code, timeout status and binary hash. Retain the tool identity output, final
staged report blob ID, commit output and remote-ref output. Capture both streams,
including empty stdout (Gitleaks may report on stderr). Use a 600-second bound and
stop on failure. A small capture runner here is authorized. Check these files exist
and contain real results before reporting done; a prose assertion alone does not
complete this task. Do not overwrite earlier evidence.

No extra report-only self-reference commit or new handoff document. Return the report
pointer when the new scan evidence and corrected report are published. Reviewer
closure will record the final commit ID without changing Hermes's report.

Reviewer governance publication for review 07 is limited to this ticket and
`docs/handoff/CURRENT_TASK.md`, based on db8a1bf4 above. Leave the dirty execution
report and all unrelated work unstaged. Check scoped diffs, links, whitespace and
source pins before publishing these two documents.

Review 07 document checks: scoped whitespace check exited 0; all 38 local links
resolved. All ten working-tree/committed source pins and both report identities
match. The index was empty before staging the two reviewer documents. No tests or
scanners were executed by the reviewer.
