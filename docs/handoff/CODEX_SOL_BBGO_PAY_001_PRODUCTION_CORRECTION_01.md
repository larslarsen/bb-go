# Codex Sol Handoff — BBGO-PAY-001 Production Correction 01

You are **Principal Dev — Codex Sol** using `gpt-5.6-sol` at High. This is the complete
durable prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Integrated gap-test baseline: `9dce8b68ebd02cb9a2030170c80a3efdfe647ba5`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`,
`docs/handoff/CODEX_SOL_BBGO_PAY_001_PRODUCTION_01.md`,
`docs/testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md`, and
`docs/testing/BBGO-PAY-001-GAP-EXPECTED-RED-REVIEW-03.md`.

## Path authorization

Edit only these three existing uncommitted production paths with `apply_patch`:

- `modern/payment/canonical.go`
- `modern/payment/signature.go`
- `modern/payment/service.go`

Preserve every test, fixture, module/lock input, document, protocol, public API, and other
path. In particular, these four production files must remain byte-identical:

| Path | SHA-256 |
|---|---|
| `modern/network/protocols.go` | `502c6224e135f6342f1501d13783abefcf343c19a280975b75d6b43d04f95669` |
| `modern/payment/types.go` | `f3aea3cd15e04e80af3d22f2d00f8c9ba73b3d31ddf1c7ddeafbe2ee8c51226d` |
| `modern/payment/frame.go` | `b910dbc28cf0e1eb023e3caf1233837bdc5a3d1551fb3bb0e210544a1cf0a025` |
| `modern/payment/replay.go` | `10ed082abba7deeb7c17d757a1e5472e808b129cf4b249411d236f469491797a` |

## Required corrections

### 1. Bound strict JSON parsing to 32 nested containers

In `canonical.go`, add one unexported constant for maximum JSON container depth and pass
an explicit depth through `parseJSONValue`. Count only object and array containers, not
their scalar leaf. The root call starts at depth zero. Before accepting a `{` or `[`,
reject when the current depth is already 32; recursive children receive `depth+1`.
Therefore 31 and 32 nested containers remain valid and 33 returns `SCHEMA` through the
existing strict-parser error conversion. Apply the same bound to ordinary payment JSON
and Boolean-enabled acknowledgement JSON. Preserve duplicate-key, UTF-8, surrogate,
number, Boolean, canonicalization, and stable-code behavior.

### 2. Reject invalid typed UTF-8 before `encoding/json` can repair it

In `signature.go`, add unexported typed-string validators for `PaymentRequestV1` and
`PaymentStatusEventV1`. Check every string field with `utf8.ValidString`; return
`schemaError` before `json.Marshal` if any field is invalid. Keep the current nil-key
check first in both signing functions. `SignRequest` and `SignStatus` must call the
corresponding validator immediately after that nil-key check.

`Service.SendStatus` currently marshals a typed status before calling `SignStatus`; call
the same status UTF-8 validator before its marshal so this path cannot replace invalid
bytes first. `SendRequest` already reaches `SignRequest` directly. Do not weaken or
reorder any later closed-schema, identity, policy, linkage, or signature validation.

### 3. Enforce linked request/status nonce separation as replay

In `Service.SendStatus`, after decoding the stored linked request and before signing or
opening a stream, reject `status.Nonce == request.Nonce` with `CodeReplay`.

In `acceptInbound`, after resolving the linked request and before status-to-status replay
or terminal checks, reject the same equality with `CodeReplay`. It must leave no record
and produce a rejected acknowledgement carrying `REPLAY`.

In `storeOutbound`, enforce the same invariant against the locally stored linked request
before appending. This is defense in depth for direct `SendSigned` callers. Preserve the
existing replay-before-terminal classification order and identical-delivery idempotence.

### 4. Make stored-record integrity validation linear

Rewrite only `validateStoredRecords` as an O(n) pass plus an O(n) linkage pass. Remove
the nested prior-request and prior-status scans and repeated decode loops. Use maps for:

- digest;
- request ID;
- request nonce;
- status event ID;
- status event nonce; and
- terminal status by request ID.

Decode and retain each request/status at most a constant number of times. Any duplicate
digest or conflicting replay/terminal key in stored data remains `CodeStorage`, not
`CodeReplay`. Retain enough status metadata (decoded status, signer, direction) for the
second linkage pass. In that pass, preserve all existing missing-link, signer, and
direction-binding rules, and additionally classify a stored status whose nonce equals
its linked request nonce as `CodeStorage`. Do not assume request records precede status
records in the slice. Overall time must be O(n); maps/slices may use O(n) space.

## Restrictions and report

Do not run commands, tests, formatters, builds, fuzzers, vet, scanners, Git, GitHub, or
network tools. Do not edit tests, fixtures, types, public signatures, framing, replay
helpers, protocols, module/lock inputs, docs, workflows, policies, wallets, rates,
transactions, hardware/device code, release binaries, or SBOMs. Do not use root, `sudo`,
deletion, cleanup, `rm`, `/tmp`, globs, unresolved targets, public peers, external
services, or background work.

Report the three edited paths and a concise mapping from each required correction to its
implementation. Codex XHigh will inspect the entire corrected production drop before any
execution.
