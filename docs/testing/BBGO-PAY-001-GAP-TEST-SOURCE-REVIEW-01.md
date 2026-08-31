# BBGO-PAY-001 Gap-Test Source Review 01

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Governance baseline: `9a9d00ca6ba42c9cba7b06a70c0faa7e39f8c21b`

Integrated frozen-test baseline: `403df23a63f413c11e13085719fc7e767c2f15be`

Result: **SOL GAP-TEST DROP ACCEPTED FOR EXPECTED-RED EXECUTION**

Sol changed only the three authorized test paths and ran no command. The reviewer
inspected each new test, corrected one formatting-only line through Sol, reproduced the
final hashes and line counts, confirmed a clean `git diff --check`, and obtained an empty
read-only `gofmt -d` over all three tests and all seven preserved production paths. The
seven uncommitted production paths remain byte-identical to production source review 01.
No Go, test, build, fuzz, race, vet, scanner, Git, public-network, wallet, rate,
transaction, hardware, release-binary, or SBOM command ran against the drop.

## Accepted gap-test inventory

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/payment/canonical_test.go` | 732 | `f9d371d9e3cae0796775e7b57f771707e2ca9da54a033554266c30e8198227e4` |
| `modern/payment/signature_test.go` | 465 | `88bc9fdc71868137f75c820dc882577c24bf30b96adcb35b908527c0137e28d8` |
| `modern/payment/service_test.go` | 816 | `3e059272d670ae0b3f74b2fad9613aea3aef7b62bf212569361dd852cf7882c9` |

## Preserved production inventory

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/network/protocols.go` | 24 | `502c6224e135f6342f1501d13783abefcf343c19a280975b75d6b43d04f95669` |
| `modern/payment/types.go` | 121 | `f3aea3cd15e04e80af3d22f2d00f8c9ba73b3d31ddf1c7ddeafbe2ee8c51226d` |
| `modern/payment/canonical.go` | 557 | `1616072b59277510d75dcd92e8acd8b985efe60e450c59fea64fd7af33fa1a1c` |
| `modern/payment/signature.go` | 125 | `3b5b9bb38b91a606d7c1e46f75b049f3af986cb83ceacce864e6efe801f6233d` |
| `modern/payment/frame.go` | 125 | `b910dbc28cf0e1eb023e3caf1233837bdc5a3d1551fb3bb0e210544a1cf0a025` |
| `modern/payment/replay.go` | 21 | `10ed082abba7deeb7c17d757a1e5472e808b129cf4b249411d236f469491797a` |
| `modern/payment/service.go` | 660 | `eff2a65b935b37439d84752f2046364aa8ae10cc28eae184b633407c8ec7cd48` |

## Why the tests are accepted

1. `TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal` constructs a genuinely invalid
   Go string, proves the fixture is invalid, invokes the real typed `SignRequest` path,
   and requires stable `SCHEMA` rejection before JSON can replace bytes.
2. `TestCanonicalJSONNestingBoundaries` independently constructs 31-, 32-, and
   33-container arrays, checks the generated structure and distinct lengths, requires
   exact unchanged canonical bytes below and at the limit, and requires `SCHEMA` only
   above it. The scalar leaf is not counted as a container.
3. `TestStatusNonceMustDifferFromLinkedRequest` uses the real two-node libp2p send path,
   first proves the linked request is persisted, then signs and sends a cancellation
   whose nonce equals that request nonce. It requires `REPLAY` and proves neither an
   event-addressable record nor any stored status record remains.

The tests are non-vacuous against the preserved source: the producer currently signs the
JSON-repaired memo, the parser currently accepts depth 33, and the service currently
persists the cross-object nonce collision. Luna may now run only the exact focused
expected-red command in `CODEX_LUNA_BBGO_PAY_001_GAP_EXPECTED_RED_01.md`. Production
correction remains unauthorized until Codex XHigh accepts the resulting evidence.
