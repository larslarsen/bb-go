# BBGO-PAY-001 Production Source Review 02

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Governance baseline: `5477b0221dca082456d05f1cf3dbaac1c76a6793`

Integrated test baseline: `9dce8b68ebd02cb9a2030170c80a3efdfe647ba5`

Result: **CORRECTED PRODUCTION SOURCE ACCEPTED FOR GREEN EXECUTION**

Sol edited only the three correction-authorized production files and ran no command.
Codex XHigh inspected the complete corrected parser, producer/signature, service,
transport, persistence, and stored-integrity control flow. `git diff --check` passed and
read-only `gofmt -d` was empty over all seven production paths. The four production files
outside correction scope remain byte-identical to review 01. No Go, test, build, fuzz,
race, vet, scanner, Git, public-network, wallet, rate, transaction, hardware, release
binary, or SBOM command has run against the corrected drop.

## Accepted production inventory

| Path | Lines | SHA-256 |
|---|---:|---|
| `modern/network/protocols.go` | 24 | `502c6224e135f6342f1501d13783abefcf343c19a280975b75d6b43d04f95669` |
| `modern/payment/types.go` | 121 | `f3aea3cd15e04e80af3d22f2d00f8c9ba73b3d31ddf1c7ddeafbe2ee8c51226d` |
| `modern/payment/canonical.go` | 565 | `5b588333ef4c72c227fcdd5bfafcb157bbaddd387bc97aa2e9956e1159aadbfc` |
| `modern/payment/signature.go` | 156 | `9573cecec02e0df951ccb08d70ed98ca112019f72ab94827846ffc2179ac9613` |
| `modern/payment/frame.go` | 125 | `b910dbc28cf0e1eb023e3caf1233837bdc5a3d1551fb3bb0e210544a1cf0a025` |
| `modern/payment/replay.go` | 21 | `10ed082abba7deeb7c17d757a1e5472e808b129cf4b249411d236f469491797a` |
| `modern/payment/service.go` | 687 | `a523b6b886d9eeae4468a2eaf3609c698dd919981ac2dc653bf23bb4e0d59bea` |

## Review conclusions

1. The strict parser counts only object/array containers. Roots start at depth zero,
   container entry rejects at current depth 32, and children receive depth plus one.
   Exactly 32 containers plus a scalar are accepted; a 33rd container fails through the
   existing `SCHEMA` conversion in both ordinary and Boolean-enabled strict JSON.
2. Every string field in both typed payment structs is checked with
   `utf8.ValidString` before any producer-side marshal. Nil signing keys retain priority;
   all later schema, signer, and identity checks retain their order. `SendStatus` also
   validates before its preliminary marshal, so it cannot bypass the signing guard.
3. Linked request/status nonce equality is `REPLAY` before status-to-status and terminal
   admission for real inbound delivery, and is also rejected in typed outbound and
   post-ack outbound persistence paths. Identical valid delivery remains idempotent.
4. Stored integrity validation now uses digest, request-ID, request-nonce, event-ID,
   event-nonce, and terminal-request maps in one collection pass, followed by one status
   linkage pass. Each record is decoded a constant number of times. Duplicate/conflicting
   stored keys and linked cross-nonce reuse remain `STORAGE`; missing-link, signer,
   direction, network-policy, canonical, digest, and signature checks are preserved.
   Time and auxiliary space are O(n).

The entire seven-path source is accepted only for Luna's prescribed green,
falsification, race/fuzz, and pinned source-security execution. Engineering acceptance
still depends on that evidence. This routine source ticket does not build release
binaries or regenerate an SBOM.
