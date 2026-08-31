# BBGO-PAY-001 Green 01 — Consolidated Evidence

Execution actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Result: **EVIDENCE CAPTURED — XHIGH ACCEPTANCE REQUIRED**

This document consolidates the accepted, non-duplicated execution and security
evidence for BBGO-PAY-001. It does not claim engineering acceptance.

## Baselines and final source inventory

- Daemon baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`.
- Cross-repository oracle: `../bb-desktop` commit
  `d472785ab896bb5d1367c4117ffd659a9a8512ae`.
- Oracle fixture: `../bb-desktop/test/fixtures/wallet-contract/golden-v1.json`,
  231 lines, SHA-256
  `08905d2b082fa6370211ebd494130d62e3696db0f8314636f3685ba5887f929f`.
- Integrated frozen-test baseline:
  `403df23a63f413c11e13085719fc7e767c2f15be`.
- Integration governance baseline and preflight HEAD/upstream:
  `0f31cbdcd1f8ef546197b0168af817fc3174ae42`.

The final accepted source and module inventory before integration is:

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/go.mod` | 133 | `1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783` |
| `modern/go.sum` (clean control) | 374 | `4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a` |
| `modern/network/protocols.go` | 24 | `502c6224e135f6342f1501d13783abefcf343c19a280975b75d6b43d04f95669` |
| `modern/payment/types.go` | 121 | `f3aea3cd15e04e80af3d22f2d00f8c9ba73b3d31ddf1c7ddeafbe2ee8c51226d` |
| `modern/payment/canonical.go` | 565 | `5b588333ef4c72c227fcdd5bfafcb157bbaddd387bc97aa2e9956e1159aadbfc` |
| `modern/payment/signature.go` | 156 | `9573cecec02e0df951ccb08d70ed98ca112019f72ab94827846ffc2179ac9613` |
| `modern/payment/frame.go` | 125 | `b910dbc28cf0e1eb023e3caf1233837bdc5a3d1551fb3bb0e210544a1cf0a025` |
| `modern/payment/replay.go` | 21 | `10ed082abba7deeb7c17d757a1e5472e808b129cf4b249411d236f469491797a` |
| `modern/payment/service.go` | 687 | `a523b6b886d9eeae4468a2eaf3609c698dd919981ac2dc653bf23bb4e0d59bea` |

## Module and ordinary acceptance gates

From `modern/`, the exact disk-backed Go environment was used with Go 1.27.0.

`GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go mod tidy` exited 0. The only module change was the accepted direct-dependency move of `golang.org/x/text v0.40.0` from the indirect block to the direct block in `modern/go.mod`; there was no version or unrelated dependency change, and `modern/go.sum` remained clean.

The following accepted commands all exited 0:

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -run '^(TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal|TestCanonicalJSONNestingBoundaries|TestStatusNonceMustDifferFromLinkedRequest)$' -count=1
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./payment -count=1
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./network -run '^TestPaymentProtocolCurrent$' -count=1
```

The focused regression group exercised three tests; the complete payment package
contained the accepted 49 ordinary tests. The protocol-ID test passed. `go vet
./...` exited 0. `go test -race ./... -count=1` exited 0 for all packages.

## Reversible falsifications

Each falsification recorded the original SHA-256, changed only the named bounded
hunk with `apply_patch`, ran the exact named test once to the intended failure,
restored the exact hunk with `apply_patch`, reproduced the original hash, and reran
the same test green. No falsification hunk remains.

1. Domain separation: `modern/payment/types.go`, only the
   `DomainSeparatorRequest` literal was suffixed with `-falsified`.
   `go test ./payment -run '^TestGoldenValidPaymentObjectsMatchCanonicalAndDigest$' -count=1`
   failed on the independent fixture request domain/digest, then the restored test
   exited 0.
2. Remote-peer binding: `modern/payment/signature.go`, only
   `if signer != remote {` was changed to `if false && signer != remote {`.
   `go test ./payment -run '^(TestSignatureRejectsWrongRemotePeer|TestStatusSignatureRejectsWrongRemotePeer)$' -count=1`
   failed because wrong remote identities were no longer classified `REMOTE`, then
   the restored test exited 0.
3. Durable replay classification: `modern/payment/replay.go`, only the request-key
   condition was changed to `if false && (first.RequestID == second.RequestID || first.Nonce == second.Nonce) {`.
   `go test ./payment -run '^TestConflictingRequestIDAndNonceAreRejected$' -count=1`
   failed because request conflicts no longer produced `REPLAY` (storage could still
   fail closed as `STORAGE`), then the restored test exited 0.

Detailed red/green and restoration records: [green recovery](BBGO-PAY-001-GREEN-RECOVERY-01.md),
[gap expected red](BBGO-PAY-001-GAP-EXPECTED-RED-REVIEW-03.md), and
[production acceptance](BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-02.md).

## Native fuzz evidence

All six targets ran separately with one worker, JSON events, `GOMAXPROCS=1`,
`-parallel=1`, Go's 15-second timeout, and the accepted outer TERM/KILL watchdog.
Every campaign exited 0 with nonzero executions and no failure, artifact, panic,
timeout, or watchdog firing:

| Target | Executions | New / total interesting |
|---|---:|---:|
| `FuzzDecodeSignedObject` | 21,891 | 51 / 56 |
| `FuzzDecodeWireEnvelope` | 4,967 | 63 / 67 |
| `FuzzParseFrame` | 12,261 | 1 / 6 |
| `FuzzJCSDeterminism` | 6,087 | 47 / 50 |
| `FuzzSignatureMutation` | 4,849 | 52 / 55 |
| `FuzzReplayKeyCollision` | 1,433 | 88 / 91 |

The accepted detailed records are [fuzz diagnostic review](BBGO-PAY-001-FUZZ-DIAGNOSTIC-REVIEW-01.md)
and [fuzz completion review](BBGO-PAY-001-FUZZ-COMPLETION-REVIEW-01.md).

## Security evidence

- Policy suites exited 0 with 51, 49, and 27 tests respectively.
- Pinned Actionlint v1.7.12 over `go.yml`, `security.yml`, and `sbom.yml` exited 0
  with no diagnostics.
- Policy-adjudicated source Govulncheck v1.7.0 exited 0. It accepted only the
  reviewed `GO-2024-3218` exception on
  `github.com/libp2p/go-libp2p-kad-dht@v0.42.2`, owned by Lead Engineer/Reviewer —
  Codex and expiring 2026-11-29; error results 1, warning results 0, note results 2.
  Notes `GO-2026-5932` and `GO-2026-6303` on required `golang.org/x/crypto` were
  reviewed non-reachable notes.
- Pinned Gosec v2.29.0 exited 0: 17 files, 4,579 lines, zero Nosec, zero issues.
- The fail-closed Gitleaks baseline validator exited 0: 25 entries, owner
  Lead Engineer/Reviewer — Codex, expiry 2026-11-29, baseline SHA-256
  `ac71e27a9f2954f7d148b8dd9d630c587abbb92b2a183f53b267e7739f418e00`.
- Pinned Gitleaks v8.30.1's fully redacted history scan exited 0 after 3,406 commits,
  approximately 313.91 MB, and 20.4 seconds, with no new leaks. Only the known
  exhaustive-rename informational warnings appeared. No secret or match value was
  accessed, displayed, recorded, or persisted.

Detailed records: [Security A review](BBGO-PAY-001-SECURITY-A-REVIEW-01.md) and
[Security B review](BBGO-PAY-001-SECURITY-B-REVIEW-01.md). The policy and security
contracts are [BBGO-SEC-001](../../tickets/BBGO-SEC-001.md) and
[BBGO-SEC-001 evidence](../security/BBGO-SEC-001-EVIDENCE.md).

## Execution-channel non-results and disposition

The following were retained as non-results, never treated as passes or findings:

- Gap expected-red attempt 01 exited before test execution on the frozen two-result
  `DecodePaymentStatus` binding; the exact two-line test correction was reviewed and
  accepted, and the corrected focused run produced the three intended failures.
- Gap expected-red attempt 02 exited 1 after the restricted sandbox denied the real
  in-process ephemeral `127.0.0.1` listener; the narrowly approved loopback run then
  produced the accepted three intended failures.
- The first and repeated unchanged native target-2 invocations silently stalled
  before output/result, without a pollable session; the repeated invocation was
  interrupted after approximately 626.1 seconds. No process or result remained.
  XHigh accepted the bounded one-worker diagnostic, which later passed target 2.
- The first sandboxed source-Govulncheck invocation exited 1 because the official
  vulnerability database was inaccessible. Its network-authorized retry produced no
  output, exit status, or pollable session for approximately 1,929.9 seconds; no
  scanner remained. XHigh's bounded visible-channel official-database probe returned
  HTTP 200, after which the policy scan and conditional Gosec passed.
- The Luna official-database probe in the continuation channel likewise produced no
  HTTP/exit/session result for approximately 479.6 seconds; no process remained.
  XHigh recovered the channel and ran the exact remaining gate visibly. These are
  executor/transport non-results, not security verdicts.

The known PTY wrapper message `Failed to create stream fd: Operation not permitted`
was execution-channel noise only and did not alter any accepted result.

## Final state and integration decision

Before integration, `git diff --check` passed and the worktree contained exactly the
eight expected source/module paths. No source/test/fixture/policy/workflow/baseline
file was edited by this evidence capture. This routine source ticket built no product
or release binary and regenerated no SBOM; both remain manual release gates.

The detailed durable records are [expected-red review](BBGO-PAY-001-EXPECTED-RED-REVIEW.md),
[test-source review](BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md), [production review](BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-02.md),
[fuzz-hang record](BBGO-PAY-001-FUZZ-HANG-01.md), and the [Security A recovery](BBGO-PAY-001-SECURITY-A-RECOVERY-01.md).

Codex XHigh owns final engineering acceptance. No further engineering work is
authorized pending that decision.
