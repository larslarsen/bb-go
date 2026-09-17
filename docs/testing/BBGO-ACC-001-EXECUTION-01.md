# BBGO-ACC-001 — Hermes execution 01 — expected red (historical) + green + finish01 + publication

Date: 2026-09-16. Actor: Hermes, free Nous Portal model (meituan/longcat-2.0:free),
manually relayed by owner. Reviewer: Codex.

## Baseline

HEAD: `d5fca0b693f64f9b316420a6daf3a57b16c8c5ad`
Index: empty. Working tree contains unrelated modified/untracked files
(Grok DEV-001 work, review docs).

## Actor

Hermes Agent v0.18.2 (2026.7.7.2) · upstream 3c3ab69a · local 10b6d1a9 (+1 carried commit)
Provider: nous · Model: meituan/longcat-2.0:free
Go: go1.27.0 linux/amd64
Filesystem: ext2/ext3 (disk-backed)

## Source manifest (10 inputs verified)

| Path | SHA-256 | Lines |
| --- | --- | --- |
| modern/go.mod | `1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783` | 133 |
| modern/go.sum | `4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a` | 374 |
| modern/network/identity.go | `b43f6ad90149ce41526aa3b0f85329dbbf847feaaf0442529a204c637833b4fa` | 95 |
| modern/payment/signature.go | `9573cecec02e0df951ccb08d70ed98ca112019f72ab94827846ffc2179ac9613` | 156 |
| modern/accountauth/types.go | `ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb` | 78 |
| modern/accountauth/records.go | `cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640` | 177 |
| modern/accountauth/state.go | `b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1` | 122 |
| modern/accountauth/records_test.go | `264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9` | 564 |
| modern/accountauth/state_test.go | `949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed` | 545 |
| modern/accountauth/fuzz_test.go | `08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833` | 277 |

## Expected red (historical, red01)

Initial test run: compile failed on undefined public contract names (Kind, AccountID,
GrantID, Capability, Record, KnownState). Three production files absent. This was
the historical red; production source has since been authored by Sol.

Retained artifacts:
- `modern/dist/acc001/red01/test.stdout.log`
- `modern/dist/acc001/red01/test.stderr.log`

## Green + falsification + security (historical, green01)

### run-01-go-test
Command: `go test ./accountauth -count=1`
Exit: 0. PASS (0.627s).

### run-02-go-test-race
Command: `go test -race ./accountauth -count=1`
Exit: 0. PASS (7.581s, no races detected).

### run-03-go-vet
Command: `go vet ./accountauth`
Exit: 0. Clean.

### run-04-fuzz-verify-record
Command: `go test ./accountauth -run '^$' -fuzz '^FuzzVerifyRecord$' -fuzztime=30s -parallel=2`
Exit: 0. PASS (1,937,477 record-verifier executions).

### run-05-fuzz-known-state
Command: `go test ./accountauth -run '^$' -fuzz '^FuzzKnownState$' -fuzztime=30s -parallel=2`
Exit: 0. PASS (426,538 state executions).

### run-06-falsification
**Fault:** In `state.go:83`, changed `if _, revoked := s.revokedDevices[device]; revoked {`
to `if _, revoked := s.revokedDevices[device]; revoked && false {`.

**Test run:** `go test ./accountauth -run '^TestKnownStateRevocation$' -count=1`
Exit: 1. FAIL — a device's grant remained authorized after device revocation.

**Restoration:** Original bytes restored, hash `b1807c69...` verified.
**Restored test run:** Exit: 0. PASS.

**Falsification confirmed effective.**

### run-07-gosec
Tool: gosec v2.29.0 (embedded module v2.29.0).
Result: **1 issue found:**
- **G115 (CWE-190):** integer overflow conversion int -> uint16
  - Location: `fuzz_test.go:157`
  - Severity: HIGH, Confidence: MEDIUM
  - **Disposition (review 05):** Non-blocking. `packFuzzRecords` is called only with
    fixed seeds (130-byte revocations, 234-byte grants, one 235-byte malformed grant).
    All fit uint16. The fuzz callback calls `unpackFuzzRecords`, never the packing
    helper. Helper is not production code. No source change needed.

An additional `-include-tests` invocation was attempted but failed (flag not defined
in this gosec version). Log retained at `modern/dist/acc001/green01/gosec2.log`.

### run-08-govulncheck-policy (historical)
Hermes originally ran `govulncheck -mode source ./accountauth/...` which returned a
one-line clean result but was **not** the authorized whole-modern source/test scan.
This gate was recorded as incomplete. The required policy script was later executed
in finish01.

## Finish01 execution (captured by runner)

### Scanner build metadata (tools.log)

**gosec v2.29.0** (embedded: `github.com/securego/gosec/v2 v2.29.0`).
**govulncheck v1.7.0** (embedded: `golang.org/x/vuln v1.7.0`).

### 01-version
Command: `go version`
Exit: 0. Output: `go version go1.27.0 linux/amd64`.

### 02-env
Command: `go env GOVERSION GOWORK GOCACHE GOTMPDIR GOPROXY GOFLAGS`
Exit: 0. Verified environment: GOWORK=off, GOPROXY=off, GOFLAGS empty.

**Note (review 06):** GOSUMDB was omitted from the runner's environment override
despite review 05 requiring it. The runner's comment about toolchain errors is not
proof; this deviation does not invalidate the observed results on the unchanged
isolated package.

### 03-test
Command: `go test ./accountauth -count=1`
Exit: 0. PASS (0.639s).

### 04-vet
Command: `go vet ./accountauth`
Exit: 0. Clean.

### 05-policy
Command: `python3 scripts/govulncheck_policy.py source`
Exit: 0.
Output: `Govulncheck source scan: accepted reviewed exception`
Accepted reviewed exception **GO-2024-3218** on `github.com/libp2p/go-libp2p-kad-dht@v0.42.2`
(owner: Lead Engineer/Reviewer — Codex; expires: 2026-11-29).
Error results: 1. Warning results: 0. Note results: 4 (x/crypto notes).

## Before/after integrity

`before.json` and `after.json` confirm HEAD unchanged and status identical.
These snapshots contain HEAD/status only and do not independently prove unchanged
source bytes throughout execution. Current hashes match the earlier reviewer
inventory.

## Finish01 capture hashes

| Artifact | SHA-256 |
| --- | --- |
| before.json | `f4a38195be5f9409e98fd6651cf90504e2c5cf701b4a440bdb38044ca0307b02` |
| after.json | `f4a38195be5f9409e98fd6651cf90504e2c5cf701b4a440bdb38044ca0307b02` |
| tools.log | `f5d8be6bd27ab19def6a3aa762fd66de9d69476ce86ab07aca5c2a59faaf2399` |
| 01-version.stdout.log | `76227025cc0bc2be7067aa45d11e09cacfd49c58f498f4c2e4f6a9872a607bf9` |
| 02-env.stdout.log | `1fb4d912839269921ba49284b63d657dbf54c3d55d8f2ec02bd64a00441cf1ee` |
| 03-test.stdout.log | `1b55a9b738a7167920a1bd1959d77b5056e95e678a10b18c288085f1a940130f` |
| 04-vet.stdout.log | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| 05-policy.stdout.log | `05234cec3775b7be8a52ab2326ce83935c9f60cc4e78ae00d016dafd5e6bbf7d` |
| 03-test.meta.json | `a511c1fd8094e9a70c5294e6db0e9463216a0c4003c2bea3b8a4b7b2f7ec3904` |
| 04-vet.meta.json | `4de26344d28cf1275e98fbff952b62b8078d5c7068f37ef1942c60a4bf2272ee` |
| 05-policy.meta.json | `fbac18a5ad0451ccb96c49dc07277749efe85222d090ff6fc21e57496b4bbd01` |

## Verdict

GREEN, FALSIFICATION, FUZZ, RACE, VET, GOVULNCHECK_POLICY ALL PASSED.
Gosec: 1 test-code finding (G115, reviewed exception, non-blocking).
Govulncheck policy: accepted reviewed exception GO-2024-3218.

All 10 source hashes match review 05. No lasting source modification.

## Publication evidence

### Staged files (exact seven-file set)

1. `modern/accountauth/types.go`
2. `modern/accountauth/records.go`
3. `modern/accountauth/state.go`
4. `modern/accountauth/records_test.go`
5. `modern/accountauth/state_test.go`
6. `modern/accountauth/fuzz_test.go`
7. `docs/testing/BBGO-ACC-001-EXECUTION-01.md`

### Feature commit

Commit: `ad52bf019352b45bcf52906f99fcc4e776dc4b7e`
Message: "feat(account): add portable account grants and revocation verifier"
Paths: 7 (6 accountauth files + execution report)

### Report-only closeout

Commit: `db8a1bf4f838ee5293fd3ada02f487344fafdf64`
Message: "docs: record ACC-001 publication and closeout"

Remote verification: origin/master at `db8a1bf4f...` (closeout).

### Committed-range secret scan (review 07)

Command: `gitleaks git --redact=100 --no-banner --log-opts=73b67c197f3b51941ddc345541ba5e36b56dfa76..db8a1bf4f838ee5293fd3ada02f487344fafdf64 .`
Tool: Gitleaks v8.30.1, SHA-256 `444a87409b36e0c330caf3fa61f354dd13e66987ecc9db63d787db761641541a`
Exit: 0. Scanned 2 commits, ~66.48 KB. **No leaks found.**

Evidence: `modern/dist/acc001/publication02/01-range-scan.stdout.log`,
`modern/dist/acc001/publication02/01-range-scan.meta.json`

### Staged scan (report correction)

Command: `gitleaks git --pre-commit --staged --redact=100 --no-banner .`
Tool: Gitleaks v8.30.1
Exit: 0. Scanned ~982 bytes (corrected report only). **No leaks found.**

Evidence: `modern/dist/acc001/publication02/02-staged-scan.stdout.log`,
`modern/dist/acc001/publication02/02-staged-scan.meta.json`
