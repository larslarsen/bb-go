# BBGO-PAY-001 Gap Expected Red 03

Governance baseline: `3b25483e5b28b1baa9f4c221218c8b5e5de14bb7`

Integrated frozen-test baseline: `403df23a63f413c11e13085719fc7e767c2f15be`

Preflight passed with exactly eleven dirty paths, clean `git diff --check`, empty
`gofmt -d`, and all reviewed line counts and hashes matching. The seven production paths
remained byte-identical to review 01. The execution override permitted only ephemeral
`127.0.0.1` loopback sockets for the in-process libp2p test; it did not grant root access
or use any public service.

Frozen paths:

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/payment/canonical_test.go` | 732 | `f9d371d9e3cae0796775e7b57f771707e2ca9da54a033554266c30e8198227e4` |
| `modern/payment/signature_test.go` | 465 | `88bc9fdc71868137f75c820dc882577c24bf30b96adcb35b908527c0137e28d8` |
| `modern/payment/service_test.go` | 816 | `3e059272d670ae0b3f74b2fad9613aea3aef7b62bf212569361dd852cf7882c9` |
| `modern/payment/transport_test.go` | 397 | `65fc2eeb90c967d59f1e00b514a47c48edbf4ee2d366f512157391c43c811362` |
| `modern/network/protocols.go` | 24 | `502c6224e135f6342f1501d13783abefcf343c19a280975b75d6b43d04f95669` |
| `modern/payment/types.go` | 121 | `f3aea3cd15e04e80af3d22f2d00f8c9ba73b3d31ddf1c7ddeafbe2ee8c51226d` |
| `modern/payment/canonical.go` | 557 | `1616072b59277510d75dcd92e8acd8b985efe60e450c59fea64fd7af33fa1a1c` |
| `modern/payment/signature.go` | 125 | `3b5b9bb38b91a606d7c1e46f75b049f3af986cb83ceacce864e6efe801f6233d` |
| `modern/payment/frame.go` | 125 | `b910dbc28cf0e1eb023e3caf1233837bdc5a3d1551fb3bb0e210544a1cf0a025` |
| `modern/payment/replay.go` | 21 | `10ed082abba7deeb7c17d757a1e5472e808b129cf4b249411d236f469491797a` |
| `modern/payment/service.go` | 660 | `eff2a65b935b37439d84752f2046364aa8ae10cc28eae184b633407c8ec7cd48` |

Command:
`GOTOOLCHAIN=go1.27.0 go test ./payment -run '^(TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal|TestCanonicalJSONNestingBoundaries|TestStatusNonceMustDifferFromLinkedRequest)$' -count=1`

Exit code: `1`. Exact diagnostics:

```text
--- FAIL: TestCanonicalJSONNestingBoundaries (0.00s)
    --- FAIL: TestCanonicalJSONNestingBoundaries/depth33 (0.00s)
        canonical_test.go:578: depth 33 code = "", want "SCHEMA" (<nil>)
--- FAIL: TestStatusNonceMustDifferFromLinkedRequest (0.01s)
    service_test.go:309: object was accepted, want rejection code REPLAY
--- FAIL: TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal (0.00s)
    signature_test.go:91: invalid UTF-8 producer code = "", want "SCHEMA" (<nil>)
FAIL
FAIL	github.com/larslarsen/bb-go/modern/payment	0.019s
FAIL
```

Each failure is the intended missing production protection: depth 33 is accepted,
cross-object request/status nonce reuse is accepted, and invalid typed UTF-8 is signed
without `SCHEMA`. No compile, syntax, import, dependency, environment, bind, panic, or
unrelated failure occurred. Attempt 01 never reached test execution and remains recorded
only as superseded execution evidence. No production path was edited or staged.
