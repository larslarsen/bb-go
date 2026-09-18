# BBGO-MEDIA-001 — rich media posts, messaging and IPFS attachments

Status: **QUEUED — architecture and implementation breakdown; no source actor active.**
Owner request recorded 2026-09-17. Reviewer: Codex, High.
Companion: [BBGO-MSG-001 — libsignal messaging](BBGO-MSG-001.md).
NET-001 remains the active assignment. This ticket does not interrupt its source work.

## Goal and existing foundation

Support mixed text, Unicode emoji, inline images, animated GIFs, video attachments and
reactions in both public posts and private messages on Electron desktop and Android
using shared UI components. Owner clarification: posts do not use libsignal.
IPFS content identifiers replace object-storage/CDN URLs. Updates remain automatic;
no manual Connect or Refresh control.

The desktop currently uses plain JavaScript in social/app.js/core.js and a textarea,
not React. There is no Android project in the reviewed workspace. The daemon has
signed direct-message envelopes with a 128 KiB frame limit and 20,000-rune text limit,
plus block-level IPFS Put/Get. It does not yet provide a large-file importer, attachment
retention manager or authenticated media-serving API. Existing private-message
transport encryption is not the libsignal integration requested in MSG-001.

This ticket specifies the target and implementation slices. Exact dependency versions,
wire bytes, source paths, hashes and red/green commands must be frozen when a slice
is activated. Logical schemas below are not authorization to change existing v1
signatures. No app rewrite, new infrastructure service or real-data migration is
authorized by this queued record.

## 1. Shared composer and rendering

- Use the open-source Tiptap editor with a restricted schema: paragraphs, text,
  line breaks, bold/italic/code, safe links and attachment-reference nodes. Render
  emojis as Unicode text. Serialize a small BitBook content model, not arbitrary
  Tiptap JSON or HTML from peers. Paste formatting is filtered through this schema.
- Target a shared React/TypeScript composer and media component package. Mount it in
  the existing Electron post composer/feed and Messages view. The proposed
  Android host is Capacitor/WebView with native adapters. The platform slice must
  prove keyboard, composition/IME, accessibility, selection and native integration
  before treating that host choice as production-ready. React Native is not assumed
  to run this DOM editor unchanged.
- Bundle Emoji Mart and its data locally. Emoji picker, keyboard navigation, skin-tone
  selection and insertion must work without a remote script/data CDN.
- Use **Klipy** for GIF-picker search in both posts and messages, with recents and
  favorites (owner update, 2026-09-17). Keep locally retained GIFs and user imports
  available offline. Review Klipy's integration requirements, import/rehosting terms
  and privacy behavior when activating the UI slice. Selected GIF bytes go through
  the attachment pipeline and IPFS, never a required hotlinked CDN URL.
- Provide an accessible shared MediaViewer/lightbox: full-screen image preview,
  zoom/pan, next/previous, Escape/back dismissal and focus restoration. Use native
  video controls, poster frames and explicit playback. GIF animation respects reduced
  motion and pauses off-screen; virtualize long histories and release decoded media.
- Preserve post/message drafts, reply context, selection, feed/conversation scroll
  position and payment-request cards
  during live updates. Show preparation/transfer progress, cancellation, retry and
  unavailable-media states. Media receipt is not a read receipt.
- Include public post composition/feed rendering and private one-to-one conversations.
  Share editor/viewer components but give each an explicit public/private publication
  context. Group messaging and live WebRTC calls are outside the initial scope.

Component basis: [Tiptap React integration](https://tiptap.dev/docs/editor/getting-started/install/react),
[Emoji Mart local data and React picker](https://github.com/missive/emoji-mart), and
[Capacitor native adapters](https://capacitorjs.com/docs). Pin reviewed versions and
licenses at activation; do not install moving latest versions or paid editor services.

## 2. Desktop and Android file ingestion

One intake interface feeds the same daemon attachment job regardless of platform.

- Electron: handle user-initiated clipboard paste through ClipboardEvent file items,
  Ctrl+V/Cmd+V and HTML5 drop. Support a file chooser. Prevent dropped-file navigation,
  duplicate paste insertion and silent fetching of remote image URLs from pasted HTML.
  If a platform needs native clipboard access, expose a narrow user-action bridge;
  retain sandbox/context isolation and disabled renderer Node integration.
- Android: use the system Photo Picker for images/video and Storage Access Framework
  for other supported files. Handle cancellation, permission denial, content URIs,
  cloud-backed selections, temporary grants and app suspension. Copy or stream the
  selection into bounded app-owned staging; do not assume a URI is a filesystem path.
  Neither the picker nor WebView owns the daemon's private keys.
- Treat declared MIME, filename, dimensions and length as untrusted. Enforce byte,
  decoded-pixel, frame-count and duration limits; reject directories, executable/HTML/SVG
  previews and unsupported codecs. Strip location metadata from sent image renditions.
  Escape filenames and never use them as output paths.
- Keep expensive decoding, thumbnails and optional transcoding off the UI thread,
  with memory/time bounds and cancellation. Start with JPEG/PNG/WebP, GIF and a tested
  MP4/WebM codec matrix. Unsupported media gives a clear error, not a silent corruption.

Native basis: [Android Photo Picker](https://developer.android.com/training/data-storage/shared/photo-picker)
and [Storage Access Framework](https://developer.android.com/training/data-storage/shared/documents-files).

## 3. Post, message and reaction model

Use a versioned JSON content model with separate public-post and private-message
envelopes. Private messages go inside MSG-001's authenticated encryption, separate
from libsignal's opaque ciphertext/session records. A private-message example:

    {
      "schema": "bitbook.rich-message/1",
      "messageId": "<stable logical ID>",
      "conversationId": "<authenticated conversation>",
      "kind": "message",
      "body": [
        {"type": "text", "text": "Look at this 🙂", "marks": []},
        {"type": "attachment", "attachmentId": "a1"}
      ],
      "attachments": [{
        "attachmentId": "a1",
        "manifestCid": "<CID of encrypted manifest>",
        "crypto": {"profile": "<reviewed attachment format>", "key": "<secret>"},
        "mediaType": "image/png",
        "byteLength": 12345,
        "width": 800,
        "height": 600,
        "durationMs": null,
        "filename": "photo.png",
        "thumbnail": {"manifestCid": "<encrypted thumbnail CID>", "crypto": "<descriptor>"},
        "blurhash": "<optional>"
      }],
      "replyTo": null
    }

For private messages, this entire object is encrypted before network delivery.
Keys, filenames, captions,
dimensions and preview metadata are never public IPFS metadata or gateway URL fragments.
The CID addresses ciphertext; renderer display URLs are local, short-lived handles.
Private manifests describe ordered encrypted chunks and verified sizes. Thumbnail
descriptors have their own unambiguous encryption parameters.

Public posts use a separately versioned, signed public-post payload with the same
body/attachment-reference structure. Public descriptors contain the media CID, MIME,
length, dimensions, optional duration, public thumbnail CID and optional blurhash;
they contain no private-message key or libsignal fields. Preserve author signatures,
post IDs, current text limits and existing social-root/IPNS publication. Do not import
message recipients, session state or private history into public post records.

Reactions are separate authenticated events, not edits to another author's original
content: target ID and target kind (post/message), emoji, action, unique operation ID
and observed add IDs for removal. Message reactions are encrypted private control
messages; post reactions are signed public events distributed through the social
publication/synchronization path. Authenticated sender identity supplies the reactor.
Add tags and removal tombstones must converge under duplicate/reordered delivery;
removal can affect only
that reactor's additions. Multiple devices aggregate under verified account identity.
Never order reaction truth solely by wall-clock timestamps. Bound per-message state,
pending unknown targets and removal lists before allocation; freeze numeric wire limits
and compaction rules with the applicable account/device contract. Public post reactions
must not depend on libsignal sessions or expose private conversation identifiers.

Logical IDs survive retries; no duplicate posts, bubbles, reactions or notifications.
Capabilities must be negotiated. Preserve v1 historical text and signatures. Unsupported
peers receive an explicit unsupported-feature state, never secret attachment keys in
a plaintext fallback. Identity and encryption framing are governed by MSG-001; readable
name uniqueness is not a prerequisite.

## 4. Attachment pipeline, retention and delivery

Shared intake: select -> validate/prepare -> choose the composer-owned visibility.
Public posts: chunk/import public media into IPFS -> retain -> sign/publish post and
CID descriptors -> fetch/verify/render in the feed. No libsignal operation.
Private messages: encrypt -> chunk/import ciphertext into IPFS -> retain -> send the
encrypted descriptor through libsignal -> fetch/verify/decrypt on the recipient.

- The daemon owns jobs, resource limits, encryption, durable staging, IPFS imports,
  retries and storage accounting. UI/native adapters stream bytes through an
  authenticated, origin-restricted local interface with opaque job handles. A web
  page cannot submit arbitrary paths, fetch arbitrary URLs or decrypt arbitrary CIDs.
- For private attachments, generate independent random keys and use a reviewed streaming
  AEAD
  format/library. Freeze algorithm, framing, nonce handling, authenticated context,
  truncation detection and vectors before production. Do not invent a cipher or
  reuse libsignal session keys for bulk files. MSG-001 protects descriptor/key delivery.
- Encrypt private originals/renditions, thumbnails and manifests before block storage.
  Public post media and thumbnails are intentionally public and need no message key.
  Stream bounded chunks into the existing blockstore/Bitswap path; do not call the
  current whole-block Put with a complete large video. Use established IPFS file/DAG
  import machinery where applicable. IPFS chunking is not itself encryption.
- Because private encrypted manifests hide child links, retention must track every
  required ciphertext block; pinning only the manifest cannot be assumed recursive.
  Public file DAGs may use ordinary recursive retention. Durably retain a completed
  import before committing its message to the outbox or publishing its public post.
  Recover after crash without advertising partial files as complete.
- Author retains public post media while its post requires it; readers use the bounded
  public-media cache. Sender retains private content while its message/history requires
  it. Recipient
  verifies and retains downloaded content independently. Track queued, message
  delivered, media available locally and read as different states. Acknowledgment of
  the message alone must not release the sender's only copy.
- Resume by verified ciphertext chunks after interruption; use bounded parallelism
  and backoff. No new encryption under reused nonces on retry. Corrupt/missing chunks
  produce retry/unavailable states. Verify authentication before exposing each supported
  streaming segment; range playback must respect the encryption format.
- Render through scoped local media handles, with MIME restrictions, no active content
  and bounded range requests. Never navigate the renderer to arbitrary IPFS HTML or
  substitute a public gateway for private authenticated media serving.
- Visibility is never inferred from the CID or a filename. Reusing a private attachment
  in a public post requires a deliberate user publication action and a fresh public
  import of the selected rendition. Never publish its private descriptor/key or
  silently turn a private cached object into a public post attachment.
- Implement explicit retained references and bounded evictable cache. Never silently
  evict the only retained outgoing copy to make a send look successful. Disk-full
  blocks preparation with a recoverable error. Deletion removes local references,
  not a promise to erase copies already received elsewhere.
- Initial engineering defaults, to verify on mobile before release: eight attachments
  per post/message; 25 MiB per image/GIF; 100 MiB per video; 200 MiB total; two preparation
  jobs and four block transfers. Cap decrypted JSON at 64 KiB while preserving the
  existing 20,000-rune message upper bound where it fits; retain the current post text
  limit. Bound manifests, pixels, animation
  frames and cache size in the activated contract; do not use these defaults to
  justify unbounded decoder allocations.
- Follow the existing trust/Spam policy: filtered content does not auto-fetch or
  decode media before inspection/override. Zero reputation alone does not disable
  attachments; apply bounded behavior/resource controls and the newcomer warning.

IPFS does not encrypt file contents automatically, and availability needs retained
copies on reachable nodes. No required S3, CDN, public HTTP gateway or paid pinning
provider. Encrypted storage does not hide traffic/CID-provider metadata.
[IPFS privacy](https://docs.ipfs.tech/concepts/privacy-and-encryption/),
[persistence](https://docs.ipfs.tech/how-to/pin-files/) and
[file/DAG model](https://docs.ipfs.tech/concepts/file-systems/).

## 5. Actionable slices and acceptance

| Slice | Deliverable | Required proof |
| --- | --- | --- |
| M1 — contract | Separate post/message schemas, private attachment encryption profile, limits, API/jobs and shared UI packaging | Independent vectors; malformed inputs rejected; public/private separation; private key binding agrees with MSG-001 |
| M2 — daemon pipeline | Public and encrypted-private imports, references, recovery, download and scoped media API | Two controlled nodes; public feed retrieval; private ciphertext-only exposure; corruption, cancellation, disk-full and restart |
| M3 — desktop UI | Post/message composers, emoji/GIF pickers, paste/drop, inline feed/chat media, viewer and video | Actual intake-to-publication/delivery paths; keyboard/IME/accessibility; no injected HTML or sandbox weakening |
| M4 — reactions/live updates | Signed public-post reactions, encrypted message reactions, deduplication and reconciliation | Duplicate/reordered delivery converges; drafts/scroll survive; no cross-context leakage or manual refresh |
| M5 — Android | Native pickers, shared components, secure daemon bridge and lifecycle handling | Actual emulator/device boundary; content URI/grant expiry, background/resume, memory/storage constraints |
| M6 — acceptance | Offline end-to-end fixtures, security scans, dependency/license records and local rebuilds | Public posts work without libsignal; private media expose ciphertext only; no leaked keys; bounded checks; preserved text/payment UI |

UI fixtures and contract work can proceed independently when activated. Private media
release requires MSG-001's accepted encrypted messaging and NET-001's accepted network
integration. Public rich-media posts require the public pipeline and networking but
do not depend on libsignal. WebRTC/video conferencing is separate and not a prerequisite.
Every source slice follows the repositories' test-first/falsification rules. Activation
adds exact paths, hashes and commands here or in one bounded child ticket; no actor
is authorized by this architecture record.

## Baselines and documentation publication

Reviewed bb-go: 8d41a06d058c11b6f151543780add3da4818e857.
Reviewed bb-desktop: 8298af916c95e627c28aed8ba3de83daa7cf9509.
Preserve concurrent NET-001 test work and all unrelated dirty files.

Reviewer-only publication for these two owner requests:
- bb-go: tickets/BBGO-MEDIA-001.md, tickets/BBGO-MSG-001.md,
  docs/handoff/CURRENT_TASK.md.
- bb-desktop: docs/handoff/CURRENT_TASK.md.

Validate document links/formatting and commit each repository's exact document set.
No source, dependency installation, acceptance execution, app launch or restart.

Owner's Klipy provider update is a reviewer-only publication of
tickets/BBGO-MEDIA-001.md.
