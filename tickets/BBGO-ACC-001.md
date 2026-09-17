# BBGO-ACC-001 — portable account grants and revocation verifier

Reviewer: Codex, 2026-09-16. **Active phase: Sol test-source correction, review 01.**
Actor: Codex Sol, `gpt-5.6-sol`, High, owner-relayed. This ticket is the handoff;
do not create another handoff for each subsection. Read AGENTS.md, TESTING.md and
[CURRENT_TASK](../docs/handoff/CURRENT_TASK.md). The daemon's principal-dev role
routes cryptographic/protocol-core source to Sol. The first test drop is present;
the reviewer has not launched an actor. Read the correction requirements in the
phase record below before editing. Production and execution remain unauthorized.

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
at initial authorization. For the active correction, the phase record pins the three
existing test files; preserve and amend that drop. The three production files must
remain absent. Unexpected source identities require review, not an overwrite.

| Frozen input | SHA-256 |
| --- | --- |
| modern/go.mod | 1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783 |
| modern/go.sum | 4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a |
| modern/network/identity.go | b43f6ad90149ce41526aa3b0f85329dbbf847feaaf0442529a204c637833b4fa |
| modern/payment/signature.go | 9573cecec02e0df951ccb08d70ed98ca112019f72ab94827846ffc2179ac9613 |

**Writable now, test source only:**

- `modern/accountauth/records_test.go`
- `modern/accountauth/state_test.go`
- `modern/accountauth/fuzz_test.go`

**Production paths reserved for the later, explicitly activated production phase:**

- `modern/accountauth/types.go`
- `modern/accountauth/records.go`
- `modern/accountauth/state.go`

Preserve unrelated dirty/untracked files, especially cancelled DEV-001 work. No
dependency, workflow, AGENTS, other source, record, Git, formatting-command or test
execution authority for Sol. Read-only inspection/hashing is allowed. Author tests in
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

**Now:** Sol corrects the three test files under review 01 below, then stops without
execution or production edits. Its completion notice points to the files and this
ticket. The reviewer reads the drop and records its actual paths, hashes, line/test
counts and disposition in the
phase record below; no owner transcription and no invented execution result. Sol's
role excludes repository-record ownership, so Hermes owns the eventual single report
`docs/testing/BBGO-ACC-001-EXECUTION-01.md`.

Next, after reviewer test-source acceptance, Hermes records the targeted red. Only
then can the reviewer activate Sol's three production paths. After source review,
Hermes performs green, one falsification and the named gates, using the same ticket
and report. Review transitions update this ticket and CURRENT_TASK; do not create
separate documents for every command. No execution or developer Git phase is active
yet. Final publication requires reviewer acceptance and an enumerated path set.

Hermes will first record tool identities, clean/dirty inventory and all six source
hashes. Use Go 1.27.0, `GOWORK=off`, and a task-owned disk-backed cache/artifact directory
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

### Test-source review 01 — correction required

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
