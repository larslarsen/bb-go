# BBGO-PAY-001 Test Source Review 01

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Governance head: `01d709c62fa27cca4ec4869edada3581ced90daa`

Source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Result: **GROK DROP REJECTED — SOL TEST CORRECTION REQUIRED**

Grok changed only the seven authorized paths, ran no Go/test/Git/network command, and
copied the 231-line desktop oracle byte-for-byte at SHA-256
`08905d2b082fa6370211ebd494130d62e3696db0f8314636f3685ba5887f929f`.
`git diff --check` passes. The fixture and 20-line protocol test are accepted and frozen.
The other five test files require correction before any execution.

## Blocking findings

1. `transport_test.go:34` passes split endpoints `[2, 2, 8, len]`. The reader's second
   call therefore returns zero bytes and EOF at offset 2. A correct `ReadFrame` cannot
   pass the test. This is a fixture failure, not expected missing-production red.
2. `signature_test.go:81-112` mutates only the outer version/kind, one request memo, and
   one signature byte. It does not satisfy the ticket's every-field request **and status**
   signature mutation contract. The test also does not prove each mutation changed the
   canonical bytes.
3. `signature_test.go:216-236` signs the pretty/noncanonical copy itself. Signature-first
   and schema-first implementations can both eventually return `SCHEMA`, so the claimed
   validation-order proof is vacuous. The test must carry the signature over canonical
   bytes while presenting the pretty copy; only schema-first returns `SCHEMA`.
4. The malformed-base64 transport test accepts either `SCHEMA` or `SIGNATURE`, weakening
   the fixed decode-before-signature order. Truncated prefix/payload cases likewise omit
   the stable `FRAME` assertion. `FuzzParseFrame` does not prove an accepted frame consumed
   all input, so trailing/coalesced bytes can escape its property.
5. `service_test.go:47-49` checks only that `ReceivedAt` is nonzero. Production can ignore
   the injected clock and use wall time. Receipt time must equal the exact fake clock.
6. Event replay tests combine an already-terminal cancellation with ID/nonce conflicts,
   permit `STATUS` in place of `REPLAY`, and do not prove conflicting event records are
   absent. The pure `CheckStatusReplay` call cannot substitute for the actual stream,
   verification, and durable-service path. The terminal-status test counts only the
   first ID and can pass even if the conflicting later event was also stored.
7. The wrong-status-signer case asserts neither its stable linkage code nor absence from
   storage. Request replay cases likewise do not prove the original record remains the
   sole exact digest after both conflicts.
8. The negative-capability AST scan can pass with zero production files, checks exported
   type names but not their public fields/functions/methods, and therefore misses a
   forbidden wallet/rate/account/transaction field placed inside an allowed type.
9. The transport leak test permits eight leaked goroutines and eight leaked descriptors
   after one cycle. A warm-up baseline plus repeated create/send/close cycles must make a
   per-service leak observable without relying on a single loose threshold.
10. `FuzzReplayKeyCollision` fuzzes request bytes but reconstructs the same two status
    objects on every iteration. Status replay inputs are not fuzzed. Network/receiver
    valid coverage also omits the allowed `zec-regtest` and `xmr-testnet` relations, and
    status closed-schema/type/ID/timestamp/control boundaries are materially thinner than
    the request table.

The existing random identity helper is also inconsistent with the ticket's reproducible
test contract. The corrected helper must derive deterministic unique Ed25519 seeds and
`newPaymentNode` must pass the resulting key in `network.Config`; clocks and datastores
remain injected/in-memory.

These findings affect signature trust, attacker-controlled framing, durable replay,
concurrency/leak detection, and the negative wallet/rate boundary. Repository routing
therefore assigns the correction to Principal Dev — Codex Sol at High, not Grok Build.
No test execution, integration, production source, Git, or GitHub action is authorized.
