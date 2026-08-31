# BBGO-PAY-001 Production Source Review 01

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Governance baseline: `4d6d8bdae7608d70da8ee88cc2c53ca2d61901f0`

Integrated frozen-test baseline: `403df23a63f413c11e13085719fc7e767c2f15be`

Result: **SOL PRODUCTION DROP REJECTED BEFORE EXECUTION — GAP TESTS REQUIRED**

Sol changed only the seven authorized production paths, ran no command, and left every
test, fixture, module/lock input, document, workflow, policy, and other path untouched.
The reviewer reproduced its line counts and SHA-256 values, found a clean read-only
format diff, and inspected the complete canonical, signature, frame, replay, service,
persistence, and protocol source. No Go, test, build, fuzz, race, vet, scanner, Git, or
network command has run against the drop.

## Reviewed source inventory

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/network/protocols.go` | 24 | `502c6224e135f6342f1501d13783abefcf343c19a280975b75d6b43d04f95669` |
| `modern/payment/types.go` | 121 | `f3aea3cd15e04e80af3d22f2d00f8c9ba73b3d31ddf1c7ddeafbe2ee8c51226d` |
| `modern/payment/canonical.go` | 557 | `1616072b59277510d75dcd92e8acd8b985efe60e450c59fea64fd7af33fa1a1c` |
| `modern/payment/signature.go` | 125 | `3b5b9bb38b91a606d7c1e46f75b049f3af986cb83ceacce864e6efe801f6233d` |
| `modern/payment/frame.go` | 125 | `b910dbc28cf0e1eb023e3caf1233837bdc5a3d1551fb3bb0e210544a1cf0a025` |
| `modern/payment/replay.go` | 21 | `10ed082abba7deeb7c17d757a1e5472e808b129cf4b249411d236f469491797a` |
| `modern/payment/service.go` | 660 | `eff2a65b935b37439d84752f2046364aa8ae10cc28eae184b633407c8ec7cd48` |

## Blocking findings

1. `SignRequest` marshals the typed request before validating its source strings.
   `encoding/json` replaces invalid UTF-8 in a Go string with U+FFFD. An invalid memo can
   therefore be changed into different valid bytes and signed instead of failing
   `SCHEMA`. Signing altered user text violates exact-byte authentication.
2. `parseJSONValue` recursively accepts unbounded object/array nesting. The payment
   envelope is frame-bounded, but a hostile canonical string near that limit can force
   thousands of recursive parser frames before the closed top-level schema rejects it.
   The closed signed-value parser needs an explicit maximum of 32 nested containers,
   with below/at/above boundary coverage.
3. BBD-WAL-001 §11.3 requires a status-event nonce to differ from the linked request
   nonce. The service checks event-to-event nonce reuse but never checks the cross-object
   request/event collision. A real two-node delivery must classify it as `REPLAY` and
   leave no event record.
4. `validateStoredRecords` compares every request/status against every prior record,
   making integrity validation quadratic over a peer-growable durable collection.
   Replace the nested replay/terminal scans with digest, request-ID, request-nonce,
   event-ID, event-nonce, and terminal-request maps. Signature/canonical validation may
   remain linear. This is a static production correction after the gap tests are frozen.

The current source otherwise follows the frozen API, validation order, framing,
half-close, acknowledgement, receipt-clock, one-key persistence, cancellation-only,
replay-before-terminal, and handler-admission design. Those parts are not accepted until
the corrected whole passes review and execution.

Only the three test paths in `CODEX_SOL_BBGO_PAY_001_GAP_TESTS_01.md` may now change.
Production source remains preserved but unaccepted and must not be edited again until
the new tests are reviewed and their expected-red result is captured.
