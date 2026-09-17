# BBGO-MSG-001 — libsignal private messaging

Status: **QUEUED — owner-requested implementation direction; contract before source.**
Owner request recorded 2026-09-17. Reviewer: Codex, High.
Companion: [rich media/IPFS attachments](BBGO-MEDIA-001.md).
NET-001 remains active. No source actor, executor, installation or migration is
authorized by this ticket yet.

## Goal

Owner clarification: **public posts and their reactions/attachments do not use
libsignal**. They retain signed public publication and IPFS under MEDIA-001.

Integrate the maintained upstream libsignal into BitBook private messaging on desktop
and Android. Encrypt text, rich-message metadata, reactions, typing/read control
messages and attachment-key delivery end to end between authorized recipient devices.
Use existing BitBook discovery/transport and IPFS; do not depend on Signal accounts,
phone numbers, Signal's service, or a required central messaging server.

Libsignal supplies cryptography/session machinery. BitBook still owns authenticated
device discovery, key distribution, durable delivery, replay handling, persistence and
the UI. Preserve existing history and automatic updates. No group-chat or live-call
encryption is implied by the initial one-to-one implementation.

## Current state and integration direction

The daemon's direct v1 envelope is signed and carried on authenticated libp2p streams;
its message text is stored in the local direct-message datastore. It is not currently
a libsignal session/ciphertext envelope. ACC-001/002 provide isolated account authority
verification/storage, not live enrollment, routing bindings or libsignal key custody.

Use upstream signalapp/libsignal rather than a protocol reimplementation. The upstream
core is Rust with Java, Swift and TypeScript APIs; its README does not provide a Go
binding and explicitly states outside use is unsupported and APIs can change. Pin an
exact reviewed revision and lock its build inputs. Record its AGPL-3.0 license and
distribution obligations in the dependency/release inventory.
[Upstream source and integration notes](https://github.com/signalapp/libsignal).

Preferred daemon integration: a narrow native adapter over upstream Rust protocol code,
owned by the daemon's private messaging service. Keep transport and IPFS in Go; keep
ratchet keys/state out of the renderer. The first slice must select and demonstrate
in-process FFI versus an owned private worker, including Android packaging/lifecycle.
This is an explicit engineering item, not permission to place private keys in renderer
JavaScript or invent an unsupported Go port. Native adapters are implementation-owned;
upstream is not claimed to support our wrapper.

The shared React/Android UI calls an authenticated local messaging interface and receives
only required display/state data. Android must preserve the same custody/session-store
ownership; platform library/ABI selection is frozen in S1 before production.

## Required protocol and identity contract

1. **Keep account, application device, transport and Signal keys distinct.** Bind a
   Signal identity key and device identifier to an authorized BitBook account/device
   using the selected controller/grant model. Verify that binding and current known
   revocations before session establishment. Never derive/convert keys ad hoc or use
   display-name equality as identity. Preserve original identities on old history.
2. **Specify recipient/device routing.** Sessions are per authorized device pair;
   portable-account delivery fans out to eligible recipient devices and the sender's
   other authorized devices. Deduplicate by logical message ID. Enrollment, device
   removal, key replacement and stale authority must have explicit outcomes. Revocation
   stops future delivery; it cannot recall plaintext/keys already received.
3. **Use the pinned library's supported prekey and ratchet APIs.** Freeze suite/version,
   bundle schema, signed validity/revision, key-rotation cadence and library limits.
   Do not implement PQXDH or a ratchet from the papers. Do not claim post-quantum or
   forward-secrecy properties beyond the actual selected upstream flow.
4. **Serverless prekey discovery needs a real contract.** Signed public bundles may be
   carried by BitBook/IPFS, with account/device authentication independent of provider
   identity. Handle expiry, rollback, exhaustion, unavailable peers and bounded fetches.
   Immutable DHT/IPFS publication is not atomic consumption of one-time prekeys.
   S1 must demonstrate a library-supported offline-start flow, and specify where
   one-time consumption is authoritative. When usable keys are unavailable, keep the
   send pending and retry automatically; never send plaintext as fallback.
5. **Version the outer transport separately from encrypted application content.**
   Authenticated routing/device/session headers carry bounded opaque libsignal bytes.
   Inside are the media ticket's content schema and stable logical ID. Bind sender,
   recipient, conversation and content type to verified session/application context;
   reject cross-conversation replay or identity substitution. Preserve outer framing
   bounds before allocating or invoking native code.
6. **Compatibility is explicit.** Negotiate capability/version; preserve legacy history
   without rewriting signatures or pretending it was encrypted by libsignal. No silent
   downgrade when an encrypted conversation encounters an old/unavailable peer.
   Show upgrade-required/pending state. Plaintext fallback is not part of this task.
7. **Device identity changes are visible and actionable.** Established contacts cannot
   silently trust replacement Signal keys. Provide verification/fingerprint UX,
   explain pending sends, and follow the verified account device-authorization rules.
   Do not require another readable-name decision.
8. **Separate cryptography from trust filtering.** Account/device/session authentication
   does not confer good reputation. Integrate the existing Spam/override behavior and
   bounded resource controls; zero reputation alone must not disable ordinary messaging.

References for protocol behavior, not substitute implementation instructions:
[PQXDH](https://signal.org/docs/specifications/pqxdh/),
[Double Ratchet](https://signal.org/docs/specifications/doubleratchet/), and
[Sesame device/session management](https://signal.org/docs/specifications/sesame/).

## Durable state, attachments and privacy

- One authoritative owner serializes operations for each session. Commit session/prekey
  state and outgoing ciphertext/outbox state consistently before acknowledging send.
  Retry persisted ciphertext; do not roll back ratchets or reuse consumed key material.
  Receive/decrypt state, deduplication and delivered plaintext must recover consistently
  after crashes. Specify transaction and recovery boundaries across any native bridge.
- Bound skipped keys, stored sessions, pending unknown senders, prekeys and input sizes.
  Malformed native input, lost worker, cancellation, full disk or unavailable secure
  storage must fail closed without corrupting the durable state.
- Protect keys/state at rest through the platform custody layer (Android Keystore-backed
  protection and the selected desktop protected store). Define locked/background behavior
  and secret-free diagnostics. Encrypting network messages alone is not an at-rest
  storage claim. Media caches and local message history need their own storage policy.
- Backing up the account controller does not restore old libsignal ratchet sessions.
  Specify safe new-device/session initialization, history transfer and restore behavior;
  never clone active session stores onto concurrently running devices.
- Large files stay outside libsignal messages: independently authenticated encrypted
  IPFS media, with descriptors and decryption keys inside the encrypted message.
  Use MEDIA-001's reviewed attachment encryption format. Libsignal integration alone
  does not define a bulk-file format or make IPFS plaintext private.
- Offline sends remain durable locally. Any future relay/mailbox may hold bounded
  ciphertext only under an explicit retention/delivery contract. A DHT is not a durable
  inbox. This slice must not promise guaranteed delivery while every serving device is
  offline or introduce a required centralized relay.
- Routing metadata/traffic analysis remain outside content confidentiality. Reuse the
  same encrypted envelopes over later WebRTC transports; calling media encryption is
  a separate contract.

## Implementation slices and acceptance

| Slice | Deliverable | Required proof |
| --- | --- | --- |
| S1 — upstream/native boundary | Pinned libsignal, desktop/Android build matrix, bridge/custody/storage decision, bundle and ciphertext wire contracts | Real upstream session exchange through proposed adapter; version/license inventory; no renderer secrets |
| S2 — authority/prekeys | Account-device-Signal key bindings, verified discovery, key changes/revocation and offline-start policy | Substitution, stale bundle, revoked device, exhausted prekeys and unavailable-peer cases |
| S3 — durable encryption | Session store, encrypted send/receive, outbox, duplicate/reordered handling and crash recovery | Interrupted commits, simultaneous sends, replay, restart and no ratchet rollback; independent library vectors |
| S4 — application integration | Text, media descriptors, reactions, receipts/typing, live UI and legacy-history compatibility | Two devices per account; correct fan-out/deduplication; encrypted attachments remain unreadable to unrelated peers |
| S5 — platform/release | Native packaging, key-change/verification UX, locked/background recovery and local rebuilds | Desktop plus Android boundary tests; bounds fuzzing, race/resource checks, dependency/binary scans and release identity |

Before source authorization, freeze exact paths, hashes, library version, test-first
commands, security scans and failure thresholds in this ticket or a bounded child.
Tests use synthetic identities, controlled peers and no real Signal account/service.
At least one falsification must disable recipient/device binding or authentication
checking and make a high-value test fail; restore exact source and rerun successfully.

NET-001 networking may complete independently. Account binding work is a prerequisite
for enabling portable multi-device encrypted delivery; it is not a new naming gate.
MEDIA-001 may develop UI/isolated pipeline fixtures in parallel when authorized, but
private-media release uses this accepted encrypted messaging boundary.

## Baseline and publication

Reviewed bb-go: 8d41a06d058c11b6f151543780add3da4818e857.
Reviewed bb-desktop: 8298af916c95e627c28aed8ba3de83daa7cf9509.
Account direction: [selected account architecture](../../bb-desktop/docs/architecture/BB-ACCOUNT-RECIPIENT-PROPOSAL-01.md).
Trust behavior: [ring-of-trust design](../../bb-desktop/docs/architecture/BB-TRUST-ARCHITECTURE-01.md).

The exact reviewer-only document publication set is enumerated in MEDIA-001.
No source, package installation, cryptographic key generation or live migration in
this documentation task. Preserve ongoing NET-001 tests and unrelated working files.
