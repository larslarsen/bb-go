# Codex Sol Handoff — BBGO-PAY-001 Test Source Correction 01

You are **Principal Dev — Codex Sol**, using `gpt-5.6-sol` at High. This is the complete
durable prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: the commit containing this handoff

Production source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`,
`docs/engineering/PAYMENT_ROADMAP_ROUTING.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`, `docs/handoff/GROK_BUILD_BBGO_PAY_001_TESTS.md`,
`docs/testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-01.md`, BBD-WAL-001 §§11 and 14,
and all seven Grok-authored paths.

The copied fixture and protocol-ID test are accepted and frozen at:

- `modern/payment/testdata/golden-v1.json` — 231 lines —
  `08905d2b082fa6370211ebd494130d62e3696db0f8314636f3685ba5887f929f`
- `modern/network/protocols_test.go` — 20 lines —
  `08d065c8c53abc39f9cf9d2c0607fab85eee6f37cf3fa6c5e3da306914abbccf`

Your sole task is to edit exactly:

- `modern/payment/canonical_test.go`
- `modern/payment/signature_test.go`
- `modern/payment/transport_test.go`
- `modern/payment/service_test.go`
- `modern/payment/fuzz_test.go`

Use `apply_patch`. Author test source only. Do not edit the frozen fixture/protocol test,
production, module/lock input, governance, evidence, or any other path.

## Exact correction contract

### Canonical/schema coverage

- Extend the valid asset/network/receiver-kind table so all six allowed networks are
  positive cases: `zec-mainnet`, `zec-testnet`, `zec-regtest`, `xmr-mainnet`,
  `xmr-stagenet`, and `xmr-testnet`.
- Add a closed-schema status table covering duplicate/unknown/missing keys, wrong
  version/type, malformed request/event/nonce IDs, noncanonical/impossible/out-of-range
  timestamps, and prohibited control/non-ASCII cases in applicable signed strings.
  Preserve positive codec coverage for the exact cancelled, paid, and expired shapes.
  Every rejection must require the stable `SCHEMA` code and every mutation must be
  demonstrably non-vacuous.

### Signature trust and validation order

- Replace the undersized field-mutation case with tables that alter every declared
  `PaymentRequestV1` field and every declared `PaymentStatusEventV1` field, plus every
  outer signed-envelope field. Each candidate must differ from the signed candidate;
  every schema-valid canonical mutation must fail as `SIGNATURE`, while a mutation that
  necessarily violates the closed schema must require `SCHEMA`.
- Preserve wrong domain and digest-instead-of-bytes cases, and require stable codes.
  Cover status remote signer/payee linkage as well as request remote/payer/payee binding.
- Make `TestNoncanonicalCopyIsRejectedBeforeSignature` start with a valid canonical
  signature, then substitute only a pretty/noncanonical canonical string without
  resigning. Require `SCHEMA`; signature-first would return `SIGNATURE`.
- Replace OS-random identity generation with deterministic unique Ed25519 seed bytes.
  `newPaymentNode` must pass a deterministic private key through `network.Config` so
  every in-process node identity is reproducible. Do not introduce a new dependency.

### Framing, fuzzing, and leaks

- Correct the split-reader endpoints so prefix and payload splits cannot return a
  zero-byte false EOF. Assert each scripted read boundary is exercised.
- Require exact `SCHEMA` for malformed base64 and exact `FRAME` for zero, truncated,
  oversize, coalesced, and trailing frames.
- Make `FuzzParseFrame` assert that any accepted one-frame input consumes all bytes.
  Expand `FuzzReplayKeyCollision` so both request and status raw inputs are supplied by
  fuzz parameters/seeds; do not reconstruct the same fixed status pair every iteration.
- Warm the network stack before the leak baseline, then perform at least three complete
  node/service connect-send-close cycles. The final allowance must be smaller than one
  leaked resource per cycle, so a persistent per-service goroutine or descriptor leak
  cannot pass. Preserve an explicit bounded convergence deadline and Linux `/proc` FD
  proof.

### Real service persistence/replay and negative capability

- Require inbound and outbound receipt timestamps to equal the injected clocks exactly.
- After request ID and nonce conflicts, prove the original exact digest is still the
  sole request record and neither conflicting canonical object was stored.
- Through the actual two-node send/receive/store path, require reused event ID and reused
  event nonce conflicts to return `REPLAY` and leave no conflicting event record. The
  helper-only replay oracle remains supplemental.
- Require a wrong status signer to return the stable linkage/payee code fixed by the
  ticket and leave no event. Require a distinct later terminal status to return `STATUS`,
  leave its event ID absent, and leave exactly one stored status: the original cancel.
- Make the negative-capability scan fail if it sees zero non-test production files.
  Inspect imports and every exported type name, exported struct field, exported function,
  and exported method for wallet, coin library, HTTP client, exchange-rate, quote,
  account, transaction, broadcast, USB, or device capability. Retain the exact forbidden
  route/string checks and positive references to the reserved payment API.

Preserve all accepted fixture assertions, two-node real-path assertions, failure
injection, protocol constants, API route absence, test/fuzz names not directly replaced
by the stronger tables, and every unrelated assertion. Do not reserve wallet, rate,
ReviewImage, HTTP, transaction, device, or broker authority.

Do not execute Go, tests, builds, formatters, fuzzers, scanners, Git, GitHub, network
commands, daemons, public peers, wallets, rate providers, transactions, hardware, or
devices. You may perform read-only inspection and final `wc -l`/`sha256sum` reporting
over only the seven test paths and named oracle. Do not use root, `sudo`, `/tmp`,
deletion, cleanup, `rm`, globs, or unresolved destructive targets.

Stop and report all five final line counts/hashes, frozen-path integrity, test/fuzz names
and totals, the corrected reserved production API, how each blocking finding is closed,
and confirmation that no prohibited command or unlisted edit occurred. Codex XHigh must
review the correction before Luna executes anything.
