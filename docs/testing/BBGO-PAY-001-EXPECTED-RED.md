# BBGO-PAY-001 Expected Red

Governance baseline: `8f2663eb6bcf171cdcac6356aa481fa8cda9acc2`

Production source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Cross-repository oracle: `../bb-desktop` commit `d472785ab896bb5d1367c4117ffd659a9a8512ae`

Frozen test source inventory:

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/payment/testdata/golden-v1.json` | 231 | `08905d2b082fa6370211ebd494130d62e3696db0f8314636f3685ba5887f929f` |
| `modern/payment/canonical_test.go` | 686 | `55afb662622118bf2e7b3d311adeecf45dafe7e56b9f1bc47ff72e0fcd02c150` |
| `modern/payment/signature_test.go` | 452 | `673cdfa99506673cdc0b063aa9dcb2418bbea35557485996a1629513f2a83c31` |
| `modern/payment/transport_test.go` | 397 | `6942bc48ada7c0f4d096f7e9b337f5052c33a4c3b726c9b571e56e4d045a682e` |
| `modern/payment/service_test.go` | 771 | `eb46b15c6c52dc363e65281958a804996d6df8a7d2e8284997079e471d52ffcd` |
| `modern/payment/fuzz_test.go` | 203 | `b17336d2c21e94dc8af77288a2fe933db41d849ab0f3b3e88d51246457daf7ee` |
| `modern/network/protocols_test.go` | 20 | `08d065c8c53abc39f9cf9d2c0607fab85eee6f37cf3fa6c5e3da306914abbccf` |

The copied fixture comparison against the desktop oracle exited 0; the six Go test files
were clean under `gofmt -d`.

1. `GOTOOLCHAIN=go1.27.0 go test ./payment -count=1` exited 1. Diagnostics were
   missing-production red only: undefined `Service`, `RecordedObject`, `Kind`,
   `Acknowledgement`, `SignedObject`, and `PaymentRequestV1` in accepted tests. No
   syntax, import, module, dependency, environment, panic, or existing-production error
   occurred.
2. `GOTOOLCHAIN=go1.27.0 go test ./network -run TestPaymentProtocolCurrent -count=1`
   exited 1. Diagnostics were missing-production red only: `PaymentProtocolCurrent` is
   undefined at `modern/network/protocols_test.go:11:5`, `:12:52`, and `:16:6`. No
   unrelated or environmental error occurred.

No test source, fixture, production, module, or lock input was edited. No secret,
wallet, rate, transaction, public network, daemon, hardware, or device was used.
