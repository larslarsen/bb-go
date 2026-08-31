# BBGO-PAY-001 Test Source Review 02

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Governance head reviewed: `1a4af21dd2f2266059088e809d17af4cea9ef43f`

Production source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Result: **SOL CORRECTION ACCEPTED — LUNA EXPECTED-RED EXECUTION AUTHORIZED**

Codex Sol corrected exactly the five payment test files authorized by
`CODEX_SOL_BBGO_PAY_001_TESTS_CORRECTION_01.md`. It ran no Go command, test, formatter,
Git, network command, daemon, wallet, rate provider, transaction, hardware, or device.
The reviewer inspected all seven test paths, performed a read-only format diff, and sent
one exact whitespace-only alignment correction back to Sol. The final format diff is
clean. No production or module/lock input exists in the drop.

## Frozen source inventory

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/payment/testdata/golden-v1.json` | 231 | `08905d2b082fa6370211ebd494130d62e3696db0f8314636f3685ba5887f929f` |
| `modern/payment/canonical_test.go` | 686 | `55afb662622118bf2e7b3d311adeecf45dafe7e56b9f1bc47ff72e0fcd02c150` |
| `modern/payment/signature_test.go` | 452 | `673cdfa99506673cdc0b063aa9dcb2418bbea35557485996a1629513f2a83c31` |
| `modern/payment/transport_test.go` | 397 | `6942bc48ada7c0f4d096f7e9b337f5052c33a4c3b726c9b571e56e4d045a682e` |
| `modern/payment/service_test.go` | 771 | `eb46b15c6c52dc363e65281958a804996d6df8a7d2e8284997079e471d52ffcd` |
| `modern/payment/fuzz_test.go` | 203 | `b17336d2c21e94dc8af77288a2fe933db41d849ab0f3b3e88d51246457daf7ee` |
| `modern/network/protocols_test.go` | 20 | `08d065c8c53abc39f9cf9d2c0607fab85eee6f37cf3fa6c5e3da306914abbccf` |

The copied 231-line oracle remains byte-identical to the accepted desktop fixture. The
drop contains 49 ordinary tests and six fuzz entrypoints.

## Closure of review 01 findings

1. Split-reader endpoints now advance through prefix and payload boundaries and assert
   that every scripted boundary and every byte was consumed.
2. Non-vacuous tables alter all 13 request fields, all seven status fields, and all five
   signed-envelope fields with stable `SCHEMA`, `SIGNATURE`, or identity codes.
3. The validation-order test starts from a valid canonical signature, substitutes only
   a pretty canonical copy, does not re-sign, and requires `SCHEMA`.
4. Malformed base64 requires `SCHEMA`; zero, truncated, oversize, coalesced, and trailing
   frames require `FRAME`; accepted fuzz frames must consume the entire input.
5. Inbound and outbound request/status receipt times must equal the injected clocks.
6. Event ID and nonce conflicts traverse the real two-node send/receive/store path,
   require `REPLAY`, preserve the original event, and leave conflicting event IDs absent.
7. Wrong status signer and linkage failures require fixed codes and leave no event;
   request conflicts preserve the sole exact original digest and canonical object.
8. The capability scan fails on zero production files and inspects imports, exported
   type names, exported struct fields, exported functions, and exported methods while
   retaining forbidden route/string checks and reserved-API references.
9. The leak test warms the network, establishes a converged baseline, runs three full
   create/connect/send/close cycles, and allows fewer than one retained resource per
   cycle with a bounded Linux `/proc` proof.
10. Replay fuzzing takes request and status pairs as fuzz parameters; all six allowed
    networks and the materially missing status schema/type/ID/time/control boundaries
    are covered.

Deterministic Ed25519 seeds now produce unique reproducible identities, and every test
node passes the deterministic key through `network.Config.PrivateKey`. Service clocks
and datastores remain injected and local.

## Authorization

Only Jr Dev — Codex Luna may now execute the two expected-red commands in
`CODEX_LUNA_BBGO_PAY_001_EXPECTED_RED.md`. No production implementation, module/lock
change, broad suite, race test, fuzz campaign, scanner, daemon, public network, wallet,
rate, transaction, hardware, device, release build, binary, or SBOM is authorized.
