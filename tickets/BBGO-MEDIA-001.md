# BBGO-MEDIA-001 — rich media posts, messaging and IPFS attachments

Status: **M2A ACCEPTED, PUBLISHED AND CLOSED — later media/UI slices remain queued.**
Owner request recorded 2026-09-17. Reviewer: Codex, High.
Companion: [BBGO-MSG-001 — libsignal messaging](BBGO-MSG-001.md).
NET-001 is accepted and closed. Review 07 closes M2A; its earlier actor assignments
are historical. No implementation or executor task is currently active.

## Goal and existing foundation

Support mixed text, Unicode emoji, inline images, animated GIFs, video attachments and
reactions in both public posts and private messages on Electron desktop and Android
using shared UI components. Owner clarification: posts do not use libsignal.
IPFS content identifiers replace object-storage/CDN URLs for user-uploaded files.
Provider-selected GIFs may use their provider's supplied media URLs. Updates remain
automatic; no manual Connect or Refresh control.

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
authorized outside the explicit active assignment below.

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
  favorites (owner update, 2026-09-17). Use its supplied media URLs through the adapter
  contract below. IPFS rehosting is not required. Offline availability applies to
  locally retained user imports; it is not promised for provider-hosted GIFs. Review
  provider requirements and request privacy when activating the UI slice.
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

### GIF provider adapters and unified JSON

Owner requirement, 2026-09-17: normalize GIF APIs so feed/picker/message components
do not depend on the source provider. Start with Klipy; adding another provider later
requires an adapter and fixtures, with no provider-specific rendering branches.

Flow: provider API -> adapter -> GifPage/GifItem -> common picker -> external GIF
descriptor -> common post/message attachment renderer. User file imports use the
IPFS pipeline. Keep raw API payloads,
authentication, pagination translation, URL refresh and provider event callbacks
inside the adapter/service. Provider identity is provenance, not rendering logic.

Logical JSON contract (illustrative IDs and URLs, not an actual API response):

```json
{
  "schema": "bitbook.gif-page/1",
  "items": [{
    "id": "klipy:example-id",
    "kind": "gif",
    "title": "Waving hello",
    "altText": "A person waving hello",
    "source": {"provider": "klipy", "itemId": "example-id"},
    "attribution": {"label": "KLIPY", "url": null},
    "poster": null,
    "renditions": [{
      "id": "gif",
      "url": "https://media.example.invalid/hello.gif",
      "mimeType": "image/gif",
      "width": 320,
      "height": 240,
      "byteLength": null,
      "durationMs": null,
      "hasAudio": false
    }]
  }],
  "nextCursor": null
}
```

- Item IDs are stable provider-qualified strings, treated as opaque by the UI.
  Identical native IDs from different providers must not collide. They are not CIDs.
- Title/alt text/attribution are plain data. Missing optional metadata is null, not
  fabricated zero dimensions or durations. A poster is null or a rendition-shaped
  static preview. Keep at least one supported animated rendition; invalid results
  become a normalized adapter error, never executable HTML.
- Renditions carry their actual MIME type: a GIF result may offer image/gif,
  video/mp4 or video/webm. Shared media selection uses format support, dimensions
  and size limits. It never examines provider-specific format dictionaries.
  Provider metadata remains untrusted; apply bounded loading and decoding without
  requiring import into the daemon's blockstore.
- Expose search(query, cursor, limit, locale, cancellation), trending, resolve(itemId)
  and share(itemId) through one adapter interface. Return the same GifPage/GifItem
  types, with errors normalized as unavailable, rate_limited, unauthorized,
  invalid_response or cancelled, plus an optional retry delay. Keep provider-native
  errors out of renderer branches. Debounce/cancel obsolete searches.
- nextCursor is null at the end or an opaque service token bound to provider, query,
  locale and filters. The UI never computes native page numbers or combines cursors.
  Preserve each provider's returned order and expose section labels/branding as data.
- Recents/favorites retain stable IDs and permitted metadata; resolve refreshes
  expiring URLs. Do not persist API keys, tracking identifiers or raw responses in
  posts/messages. Required provider events run through the adapter after the defined
  user action, not as a side effect of rendering received messages.
- On selection, pass the normalized item/rendition ID to the media service.
  Feed and message renderers consume section 3's common attachment descriptors and
  section 4's media handles. They never call Klipy or parse its API responses.
  Preserve necessary attribution as generic metadata in the same visibility context.

**Concrete Klipy mapping:** the documented v2 response supplies a usable initial
adapter shape; pin the chosen endpoint/version and captured fixtures at activation.

| Klipy v2 field | Unified field / conversion |
| --- | --- |
| results | items |
| result.id | source.itemId as string; provider-qualified id |
| result.title | title, default empty string |
| result.content_description | altText; fall back to title |
| result.itemurl | attribution.url; nullable |
| result.media_formats | renditions; map supported format keys to MIME types |
| media_formats.preview | poster when available |
| media.url, dims, size | url, width/height, byteLength |
| media.duration | seconds to durationMs; absent/unknown stays null |
| result.hasaudio | hasAudio |
| next | opaque nextCursor; end-of-results becomes null |

Mapping source: [Klipy's documented response and media objects](https://docs.klipy.com/migrate-from-tenor/response-objects/content-formats).
Another API version belongs inside its adapter; its native field names must not
change this UI contract.

**Delivery correction, 2026-09-18:** the owner questioned why Klipy would need IPFS
rehosting. That was an overbroad reviewer interpretation of replacing S3/CDN for file
uploads, not an owner requirement. The selected design uses Klipy's supplied media
URLs. No Klipy rehosting approval is a prerequisite for this ticket. Klipy's standard
integration requires direct loading and preserved delivery data. Custom caching,
proxying or combined-provider results would need separate approval if later requested;
none is selected here. Unified JSON/rendering does not require a combined search grid.
Provider-hosted media depends on provider availability, and media requests reach the
provider even when the containing message is encrypted. Do not describe these public
GIF bytes as encrypted private attachments. Provider-specific request handling belongs
in adapters, with request credentials/delivery requirements reviewed at UI activation.
[Klipy integration requirements](https://docs.klipy.com/#integration-requirements).

Add adapter contract fixtures to M1/M3: Klipy plus a synthetic second provider with
different field names, pagination and duration units must produce the same normalized
shape and use the same UI. Cover absent previews/metadata, malformed URLs/dimensions,
unsupported formats, duplicate native IDs across providers, pagination termination,
expired URLs, cancellation and rate limits. No live credentials or API calls in tests.
This remains queued specification work, not source implementation authorization.

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
from libsignal's opaque ciphertext/session records.

Separate media location from display metadata. The shared renderer receives a typed
location (IPFS file or external GIF reference) plus normalized MIME, dimensions,
duration, caption and attribution. External GIF references preserve provider/item and
rendition identifiers; their adapter supplies the permitted direct URL. Rendering can
branch on location/format without provider-specific response parsing. The private
message encrypts either kind of descriptor, but encryption of an external reference
does not encrypt the provider's media or conceal the subsequent provider request.
Exact signed JSON bytes are a later integration contract, not part of M2A.

An illustrative private user-file message (bulk encryption framing remains to be
selected with MSG-001; this example does not mandate a custom chunk manifest):

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
The eventual reviewed encryption profile determines file/manifest framing and verified
sizes. Thumbnail descriptors have their own unambiguous encryption parameters.

Public posts use a separately versioned, signed public-post payload with the same
body/attachment-reference structure. Public user-file descriptors contain the media CID, MIME,
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

User-file intake: select -> validate/prepare -> choose the composer-owned visibility.
Public posts: chunk/import public media into IPFS -> retain -> sign/publish post and
CID descriptors -> fetch/verify/render in the feed. No libsignal operation.
Private messages: encrypt -> chunk/import ciphertext into IPFS -> retain -> send the
encrypted descriptor through libsignal -> fetch/verify/decrypt on the recipient.
Provider GIF selection creates an external reference instead of invoking this file
import/encryption pipeline. References in private messages still travel inside libsignal.

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
- Retention must cover every required ciphertext block. If the selected encryption
  format hides child links, pinning only a manifest cannot be assumed recursive.
  Ordinary UnixFS file DAGs can use recursive retention. Durably retain a completed
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
copies on reachable nodes. User-uploaded files require no S3, CDN, public HTTP gateway
or paid pinning provider. Provider-selected GIFs use the provider's delivery service.
Encrypted storage does not hide traffic/CID-provider metadata.
[IPFS privacy](https://docs.ipfs.tech/concepts/privacy-and-encryption/),
[persistence](https://docs.ipfs.tech/how-to/pin-files/) and
[file/DAG model](https://docs.ipfs.tech/concepts/file-systems/).

## 5. Actionable slices and acceptance

| Slice | Deliverable | Required proof |
| --- | --- | --- |
| M1 — contract | Separate post/message schemas, unified GIF adapter JSON, private attachment encryption profile, limits, API/jobs and shared UI packaging | Independent vectors; malformed inputs rejected; provider adapter fixtures; public/private separation; private key binding agrees with MSG-001 |
| M2A — active file primitive | Bounded standard UnixFS import/read on the existing node | Independent upstream reader, controlled peer transfer, cancellation, malformed DAGs and persistence |
| M2 — daemon pipeline | Public and encrypted-private imports, references, recovery, download and scoped media API | Two controlled nodes; public feed retrieval; private ciphertext-only exposure; corruption, cancellation, disk-full and restart |
| M3 — desktop UI | Post/message composers, provider-neutral GIF picker with Klipy adapter, emoji picker, paste/drop, inline feed/chat media, viewer and video | Same renderer for Klipy and a second-provider fixture; actual intake-to-publication/delivery paths; keyboard/IME/accessibility; no injected HTML or sandbox weakening |
| M4 — reactions/live updates | Signed public-post reactions, encrypted message reactions, deduplication and reconciliation | Duplicate/reordered delivery converges; drafts/scroll survive; no cross-context leakage or manual refresh |
| M5 — Android | Native pickers, shared components, secure daemon bridge and lifecycle handling | Actual emulator/device boundary; content URI/grant expiry, background/resume, memory/storage constraints |
| M6 — acceptance | Offline end-to-end fixtures, security scans, dependency/license records and local rebuilds | Public posts work without libsignal; private media expose ciphertext only; no leaked keys; bounded checks; preserved text/payment UI |

UI fixtures and contract work can proceed independently when activated. Private media
release requires MSG-001's accepted encrypted messaging and NET-001's accepted network
integration. Public rich-media posts require the public pipeline and networking but
do not depend on libsignal. WebRTC/video conferencing is separate and not a prerequisite.
Every source slice follows the repositories' test-first/falsification rules. Activation
adds exact paths, hashes and commands here. M2A below is active; other rows remain
queued. Private cryptography and GIF-provider approval do not block public file storage.

## Baselines and documentation publication

Activation baseline bb-go: 05e7d092d37d11b31969839b3f01dddc3fcfbd7c.
Routing-only baseline bb-desktop: 8a66ca7ee7f3af8f36f27ba202a5841cad5bf40b.
Subsequent reviewer documentation commits do not change the source baseline.
Preserve all unrelated dirty files and retained NET-001 evidence.

Reviewer-only publication for these two owner requests:

- bb-go: tickets/BBGO-MEDIA-001.md, tickets/BBGO-MSG-001.md,
  docs/handoff/CURRENT_TASK.md.
- bb-desktop: docs/handoff/CURRENT_TASK.md.

Validate document links/formatting and commit each repository's exact document set.
No source, dependency installation, acceptance execution, app launch or restart.

This publication also corrects the Klipy rehosting assumption and stale NET-001 routing.

## Active M2A assignment — Sol High, 2026-09-18

**Deliverable:** add bounded streaming file import/read to the existing IPFS node.
The current whole-block Put/Get cannot safely serve as a large-file attachment API.
Use Boxo already pinned in this module. This is a daemon foundation; it adds no visible
composer, upload endpoint or post/message publication yet. Those need retention and
the authenticated media service before UI wiring. Do not wait for private encryption
or a GIF-provider decision to implement this independent file layer.

### Scope and baseline

Authorize tests first: new modern/network/files_test.go and
modern/network/files_fuzz_test.go. Production: new modern/network/files.go.
The only other source-input paths allowed are modern/go.mod and modern/go.sum for
the minimal requirements made necessary by these Boxo imports. Keep all existing
dependency versions; use the already pinned module graph and recorded checksums.
In particular Boxo stays v0.42.1; go-ipld-legacy v0.3.0 and go-codec-dagpb v1.7.0 are
already present in go.sum. No dependency upgrades or new media/crypto libraries.
Small deterministic fixtures belong in test source; no binary fixture tree is needed.

Completion record: create docs/testing/BBGO-MEDIA-001-EXECUTION-01.md with a Sol
section. Write exact commands, exits, test counts, raw-output paths/hashes, changed
file hashes/line counts, and remaining limitations there. Do not edit this ticket,
CURRENT_TASK, acceptance conclusions or other actors' records. No developer Git work.

Verify these unchanged baseline inputs before editing; report a mismatch rather than
overwriting another actor's work:

| Path in modern | SHA-256 |
| --- | --- |
| go.mod | 1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783 |
| go.sum | 4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a |
| network/node.go | ee15f7a120468679a7f52a8e0fa73aa38aa86813ea5a62a493bfd990daf13555 |
| network/open.go | 96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b |
| network/node_test.go | 9abc12fc479252c390f78802c9df546e6c4a4e78d71b09acf9db20ba815ab000 |

All existing network, discovery, API, social and direct-message behavior stays frozen.
No edits in bb-desktop or go-ipfs. No account migration, pin manager, garbage collector,
HTTP gateway, provider URL fetcher, native picker or daemon restart in this assignment.

### File contract

Add these Go APIs in package network (internal helpers are the developer's choice):

```go
type PublicFile struct {
    CID cid.Cid
    ByteLength int64
}

func (n *Node) ImportPublicFile(ctx context.Context, src io.Reader, maxBytes int64) (PublicFile, error)
func (n *Node) CopyPublicFile(ctx context.Context, file PublicFile, dst io.Writer) (int64, error)
```

- These operations put/read publicly retrievable blocks. They provide no encryption,
  recipient authentication, media decoding or proof of long-term retention. Private
  plaintext must never be routed here by future application integration. A later
  reviewed encryption service can reuse the underlying storage for ciphertext.
- Maximum file size is 100 MiB. Import maxBytes must be 1..100 MiB; an empty file is
  valid. Reject invalid limits before reading/storing. Detect maxBytes+1, reader or
  storage failure; never return a usable PublicFile on incomplete import. Partial
  unreferenced blocks may remain, and must be documented as such. Do not delete shared
  blocks to simulate rollback. A future retained-import job owns completion/publishing.
- Import standard balanced UnixFS, explicit CIDv1/SHA2-256 (32-byte digest), fixed
  1 MiB chunks, raw leaves and maximum 1024 links per branch, matching the file settings
  of Boxo's unixfs-v1-2025 profile. No filenames, mode, mtime or wrapper directory.
  Identical bytes produce identical roots, including empty and one-chunk files.
  Pass per-operation parameters; never call ApplyGlobals or mutate Boxo globals.
  Reference: [UnixFS specification](https://specs.ipfs.tech/unixfs/) and the pinned
  Boxo ipld/unixfs/io/profile.go and importer packages.
- Reuse this Node's blockstore/Bitswap; do not create a second node, routing namespace
  or transfer protocol. Notify the exchange of completed block writes using its
  established API. Existing Node.Put/Get retain their current semantics.
- Copy is local-first with existing Bitswap retrieval on missing blocks. Accept only
  CIDv1/SHA2-256 raw or DAG-PB file nodes; reject unsupported CIDs, directories, HAMTs,
  symlinks and metadata-wrapper traversal. A raw root is a valid file. Use maintained
  Boxo UnixFS decoding/reading, with bounded validation around it.
- Treat every loaded block, including local cache hits, as untrusted: verify the CID
  against its bytes before decoding or output. Cap encoded blocks at 2 MiB, links per
  node at 1024, traversal depth at 32, total visited node occurrences at 4096 and total
  encoded bytes processed at 256 MiB per copy. Count repeated links each time traversed;
  a unique-CID set alone does not bound expansion. Enforce budgets on upstream prefetch
  paths too. Check lengths, child counts and integer arithmetic before allocation/use.
- The declared ByteLength must be 0..100 MiB. Enforce it while writing and require exact
  completion: declared sizes alone do not establish successful EOF. Reject malformed
  UnixFS size/block tables, truncation, extra output and overflow. A failed copy may
  have written a prefix; return its count and error, and document that callers must
  discard incomplete results. Never emit bytes beyond the declared length.
- Stream with bounded working memory, not whole-file buffers or io.ReadAll of a file.
  Use errors.Is-compatible errors for invalid files and exceeded limits; preserve
  context and underlying I/O errors. Test nil/invalid inputs without panics.
- Caller cancellation and Node shutdown must stop owned network/storage work. Close
  per-operation readers and join any owned workers; never close the shared Bitswap
  exchange from a per-file service. Boxo importer Add currently uses context.TODO:
  bind storage/exchange calls to the operation context rather than trusting that call.
  No goroutine per blocking caller I/O: an arbitrary Reader/Writer cannot be forcibly
  interrupted, so callers supply cooperative I/O and retain its close responsibility.

### Tests, iteration and developer evidence

Sol authors meaningful tests first, runs the red, implements, and iterates source plus
targeted tests until green in this one assignment. No separate red/green handoff.
Retain raw command output under fresh modern/dist/media001/developerNN directories;
inspect the filesystem first and use this repository's disk-backed storage for caches
and artifacts, not RAM-backed /tmp. These generated captures are not publication files.

Use the cached Go 1.27.0 toolchain (executable SHA-256
1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8).
Record actual executable and environment. Tests use GOTOOLCHAIN=local, GOWORK=off,
GOENV=off, GOPROXY=off, GOSUMDB=off, GOFLAGS='-mod=readonly -p=2', GOMAXPROCS=2.
Minimal `go mod tidy` is authorized with GOFLAGS='-p=2' only if the new imports require
it; record the diff and do not upgrade versions. Missing cached dependencies are an
explicit execution gap, not a test failure or permission to select replacements.

Reviewer prerequisite correction, 2026-09-18: the initial offline attempt found
`github.com/crackcomm/go-gitignore@v0.0.0-20241020182519-7843d2ba8fdf` missing from
the local source/archive cache. Boxo v0.42.1 already requires this exact version;
its retained go.sum records module checksum
`h1:dwGgBWn84wUS1pVikGiruW+x5XM4amhjaZO20vCjay4=` and go.mod checksum
`h1:p1d6YEZWvFzEh4KLyvBcVSnrfNDDvK2zfK/4x2v/4pE=`. Sol may fetch only this
exact dependency from the Go module proxy into a writable disk-backed cache and
verify both checksums before offline tidy/testing. This is prerequisite recovery,
not a version change or an intended red. Retain its command, output and checksum
verification in the same execution report. All test commands retain GOPROXY=off
and GOSUMDB=off; any further missing dependency must be reported before fetching.

From modern, exact first red and final targeted green:

```sh
go test ./network -run '^TestMEDIA001' -count=1 -timeout=180s
```

The initial red may be missing PublicFile/ImportPublicFile/CopyPublicFile symbols after
test source exists; unrelated dependency/compiler failures are not the intended red.
Final developer checks (in addition to the green above):

```sh
go test -race ./network -run '^TestMEDIA001' -count=1 -timeout=300s
go test ./network -run '^$' -fuzz '^FuzzMEDIA001FileNode$' -fuzztime=30s -parallel=2
```

Cover empty/single/multiple chunks; chunk-size and import-limit boundaries; deterministic
CIDs; streaming input/output failures and cancellation; partial-store failure; bounded
malformed DAGs including dishonest sizes/repeated links; local-block corruption; missing
remote blocks; concurrent calls and operation after Node closure. Test persistence by
reopening an isolated temporary datastore. Store a private-namespace sentinel and prove
the file transfer does not serve it as a public block.

Use an independently constructed upstream Boxo reader/importer as an interoperability
oracle and verify full byte equality on a multi-chunk file, including a golden root
computed outside the BitBook helper. Controlled two-node transfer must start with the
recipient missing every fixture block. Manual dialing is appropriate here because this
proves file transfer, not discovery; reuse existing offline upstream-fixture helpers.
No public swarm or user daemon/data. Fuzz the actual bounded block-validation path with
valid raw/DAG-PB seeds and malformed inputs, not a separate test-only parser.

Falsify the import-limit regression by temporarily bypassing its enforcement in
production, proving the above-limit test fails, restoring the exact source and rerunning
that test green. Record the precise temporary diff and outputs; never retain the fault
in submitted source. This is part of the developer task, not a separate assignment.

### Single acceptance/publication phase after source review

Codex reviews the complete source and Sol evidence in this ticket. On source acceptance,
Hermes performs the remaining commands below, appends to the same execution report and
publishes the exact accepted file set. Valid retained developer checks are reused;
do not repeat them merely because the actor changes. No implementation/test authorship
or semantic corrections by Hermes. Any discovered defect returns with concrete evidence.

With the same Go environment, from modern:

```sh
go test ./... -count=1 -timeout=300s
go test -race ./... -count=1 -timeout=600s
go vet ./...
gosec -tests ./network/...
go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$' -count=1
```

Immediately after the diversity check, from repository root:

```sh
python3 scripts/govulncheck_policy.py source
```

Use existing pinned gosec v2.29.0 (SHA-256
eb00a1fb095b161a48c5bcadbe1e246bbafe270da497a122d2e63ade346954c2),
govulncheck v1.7.0 (6c92f0536311f5e2083a839c75558e3fb986758a320a402aa8f524c85ffd7400)
and the unchanged dependency policy; the existing DHT exception keeps its exact scope
and expiry. Only advisory-data retrieval uses the network. New scanner findings require
reviewer adjudication; inherited findings retain only their previously accepted exact
scope from NET-001 review 16. Test/security execution failures block acceptance.

After gates pass, rebuild from modern with `go build -o bitbookd ./cmd/bitbookd` and
record `go version -m bitbookd`, output hash and size. No restart. Preserve the other
untracked binary at modern/cmd/bitbookd/bitbookd. Before publication, stage only accepted
M2A source/module/report paths, then from repository root:

```sh
../.security-tools/bbgo-sec-tools-20260829/gitleaks git --pre-commit --staged --redact=100 --no-banner .
git diff --cached --check
```

Gitleaks stays v8.30.1 (SHA-256
444a87409b36e0c330caf3fa61f354dd13e66987ecc9db63d787db761641541a).
Secrets block publication. Record actual commit/push results and CI identity in the
same report. Preserve unrelated work. Reviewer governance publication is separate and
limited to the four documentation paths already enumerated above.

## Review 01 — preserve source failures across chunking

2026-09-18, Codex reviewer, High. **M2A source acceptance withheld for one defect.**
Sol remains the source actor; Hermes acceptance/publication has not started. This
section supersedes the initial implementation scope for the correction below.

### Verified drop and evidence

All five changed source/module hashes and line counts match the execution report:
files.go f088afa9f213a5b112b4443b49eb8bbb025b7436fc029d457b06e396bfab032d (470 lines),
files_test.go ca2ebcf7e779f57b8b62baea1abeb081080bfed7a29e2a386923015285898a46 (827),
files_fuzz_test.go 1951689375fb002eb81d34acf3e5c9e8c1cbf55411a679683ccc83bd2a37944a (50),
go.mod df7f1e5d4fa1d20083c1fcc872415a4203f6afefb4a7b2a980d45cdd286b3c38 (138),
go.sum 58614c1e27170525cf205f0f03c54085c70e4b37e51ee0aebf73a75ff955e800 (376).
The three frozen node input hashes still match the original baseline.

Reviewer read and hash-verified the retained intended red (09), limit falsification
red (17), restored green (19), final targeted green (20), 30-second fuzz (21), and
race green (22). Their hashes match the report. Fuzz records 207,216 executions;
targeted/race package times are 1.621s/4.619s. The seven top-level tests and sixteen
named subtests are consistent with source enumeration; the non-verbose package output
does not independently enumerate individual test events. No reviewer tests/scans ran.
The report reviewed here hashes to
62e26386a7eb57d652235a88fd9b2fcc5e0d72398dd6140f6f2f28c6cb621ef1.

The module diff preserves all versions. The existing prerequisite note is verified
against pinned Boxo v0.42.1's go.mod/go.sum: the go-gitignore version and both sums
agree. Retain that note; no further dependency change or fetch is needed.

### Blocking finding: failed source reads can produce a successful descriptor

In modern/network/files.go:166, publicFileLimitReader returns source errors without
remembering them, and ImportPublicFile checks only Layout's returned error and context
before returning a descriptor at line 80. Two upstream behaviors defeat that check:

1. Boxo v0.42.1 chunker/splitting.go NextBytes treats errors matching
   io.ErrUnexpectedEOF as normal final-chunk completion. A source returning a short
   prefix with io.ErrUnexpectedEOF therefore produces an apparently complete file.
   The same problem applies to an error wrapping io.ErrUnexpectedEOF.
2. Go 1.27 io.ReadFull/ReadAtLeast drops an error returned alongside enough bytes to
   fill its buffer (io/io.go:338). A reader returning exactly one chunk with a custom
   failure, followed by EOF, therefore also yields a successful descriptor.

These are source-traced findings, not claims of a reviewer-run reproduction. Both
violate the ticket's requirement that a source failure returns no usable PublicFile.
The current short-prefix custom-error test misses them: it uses neither UnexpectedEOF
nor a full chunk, and its helper repeats the error on subsequent reads.

### Sol correction — regression, repair and targeted checks in one task

Writable paths: modern/network/files_test.go first, modern/network/files.go second,
and Sol's appended correction section/current file table in
docs/testing/BBGO-MEDIA-001-EXECUTION-01.md. Preserve the initial report and all raw
captures. Module files, fuzz source, all other production/tests and binaries are frozen
at the hashes above. No Git work, source expansion or new dependencies.

Add TestMEDIA001ImportPreservesSourceErrors exercising the public import API. Include
a partial chunk with direct and wrapped io.ErrUnexpectedEOF, and a full 1 MiB chunk
returned with a custom error followed by ordinary EOF. For each failure, require the
original error to remain identifiable with errors.Is and a zero PublicFile. Include
successful partial/full-chunk ordinary EOF controls so the repair does not reject
valid final chunks. Run the exact regression against the submitted production first:

```sh
go test ./network -run '^TestMEDIA001ImportPreservesSourceErrors$' -count=1 -timeout=180s
```

Retain the expected failed assertions, then preserve observed non-EOF source failures
across the splitter and reject completion if one occurred. Ordinary EOF is completion;
source io.ErrUnexpectedEOF is failure even though the splitter uses the same sentinel
internally for normal short EOF. Keep the streaming limits, cancellation, error identity
and partial-block policy. Internal repair design is Sol's choice.

Repeat that command green and run:

```sh
go test ./network -run '^TestMEDIA001' -count=1 -timeout=180s
go test -race ./network -run '^TestMEDIA001' -count=1 -timeout=300s
```

Use the existing verified Go toolchain and developer01 disk-backed caches with the
same offline environment; place new raw captures under a fresh developer02 directory.
Record exact command exits, outputs/hashes and final source identities in the existing
report. The regression red proves the submitted faulty behavior; no additional fault
injection is required. The existing import-limit falsification remains retained.
Fuzz exercises the unchanged block validator; its existing accepted capture need not
be rerun for this reader-only correction. Sol may iterate the correction and targeted
tests to completion without another handoff. Return the repository report for review.

Reviewer-only publication for this review is tickets/BBGO-MEDIA-001.md and
docs/handoff/CURRENT_TASK.md, including the verified existing prerequisite note.
No developer source, module changes, report or generated artifacts are included.

## Review 02 — correction accepted; Hermes acceptance and publication

2026-09-18, Codex reviewer, High. **M2A source and targeted evidence accepted.**
Review 01's correction is complete; Sol has no further assignment. Hermes may now
execute the single acceptance/publication phase specified above, under this section's
exact file pins and evidence rules. This ticket remains the only handoff.

The source now records source read errors before chunking can discard them and checks
the retained error before returning success. A concurrent layout error preserves both
identities. The regression reproduces the three prior false successes and asserts
their errors/zero descriptors after repair; ordinary EOF controls also round-trip.
Removing only this repair and its new regression/helper in memory reconstructs the
exact review 01 production/test hashes. No unrelated source correction is present.

Reviewer read and hash-verified all developer02 captures against the report: regression
red (three failures), regression green (0.018s), targeted green (1.631s) and race green
(4.646s). Current source hashes match the correction table below. The block validator,
fuzz source, module files and three original node inputs remain unchanged. Retained
fuzz and limit falsification evidence remains accepted; do not rerun it or the targeted
tests merely to change actors. No tests/scanners/build were executed by the reviewer.

### Frozen acceptance and publication inputs

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| modern/network/files.go | 485 | 5a2f7a8515fa4578edbedf3ec9899791c5f8145c570aa9d568c232a10a727492 |
| modern/network/files_test.go | 886 | ec224bd2b3cd22b00fbf512028f5464327d5d7f06dcc19835989208945b9c323 |
| modern/network/files_fuzz_test.go | 50 | 1951689375fb002eb81d34acf3e5c9e8c1cbf55411a679683ccc83bd2a37944a |
| modern/go.mod | 138 | df7f1e5d4fa1d20083c1fcc872415a4203f6afefb4a7b2a980d45cdd286b3c38 |
| modern/go.sum | 376 | 58614c1e27170525cf205f0f03c54085c70e4b37e51ee0aebf73a75ff955e800 |

The sixth publication path is docs/testing/BBGO-MEDIA-001-EXECUTION-01.md, currently
8af794fa1c9ad5bd590a0a823281869d58167f391bff42d80d6ac59a24f37c6d.
Hermes appends its acceptance/publication evidence there, preserving the Sol sections.
Only that report is editable by Hermes; source/module files are frozen. Generated
captures and the routine modern/bitbookd build output are local artifacts, not commit
inputs. Do not stage any other path, including unrelated dirty governance/README work,
cancelled DEV-001 files, caches or either daemon binary. No desktop source/publication.

### Hermes execution instructions

1. Verify the five source pins and the three unchanged original node inputs before
   work and again after commands. Use the same cached Go 1.27.0 executable and offline
   environment recorded by Sol. Reuse modern/dist/media001/developer01/gocache and
   its .gomodcache to avoid another dependency fetch. Resolve those cache paths before
   changing cwd for the root-level policy command. PATH must include the pinned Go
   toolchain bin directory and the installed gosec/govulncheck directory. No install,
   module tidy or dependency/policy edits.
2. Retain raw stdout, stderr, actual exit status, command/arguments, cwd, environment,
   timing and source/tool hashes under a fresh modern/dist/media001/acceptance01
   directory. Use disk-backed storage. A temporary capture runner is permitted there;
   it is not test source and is not published. Do not manually reconstruct output or
   call an unexecuted check passed. Reviewer has verified the pinned scanner hashes
   above and policy-script hash
   709cb00d44c62ef6e2d394f457407183d6fb90bc98958c80db0261607bc3c77c.
3. Run the exact broad test, race, vet, network gosec and DHT-diversity commands in
   “Single acceptance/publication phase after source review,” followed immediately by
   the root-level vulnerability-policy command. Tests stay offline with isolated
   loopback fixtures. Only the policy scanner's official advisory retrieval uses the
   network. Stop on test/vet failure or tool-launch failure and record the gap.
   A scanner finding may be retained alongside the other security reports, but new
   findings require reviewer adjudication before build/publication; only NET-001
   review 16's exact inherited sites and the existing dependency policy are accepted.
4. If all gates pass within that policy, perform the already specified local build and
   record modern/bitbookd's SHA-256, size and actual `go version -m` identity. No daemon
   restart. Preserve the untracked modern/cmd/bitbookd/bitbookd file.
5. Append actual results and raw artifact identities to the existing report. If all
   gates passed, stage exactly the five pinned source/module paths and that report.
   Verify the staged path list and source hashes; execute and retain the ticket's
   staged Gitleaks and whitespace commands. A secret finding blocks publication.
   Then commit and push to origin/master, retaining the actual command results.
6. Record the feature commit, push result and its Go CI run/result in the same report.
   A report-only closeout commit is authorized to retain these post-publication facts;
   recheck its exact staging, Gitleaks and whitespace before pushing. No extra owner
   permission or new handoff is needed when the gates pass. Return the report for
   final reviewer acceptance; do not label pending/failed CI as successful.

Reviewer publication for review 02 is only this ticket and docs/handoff/CURRENT_TASK.md.
It does not integrate the developer drop or claim broader acceptance is already complete.

## Review 03 — retain verified acceptance; repair test cleanup and closeout record

2026-09-18, Codex reviewer, High. Production hashes remain source-accepted. Hermes
published before the required gates passed; final acceptance is withheld for the
test-context cleanup below. Do not revert the published feature or rerun valid broad
checks. This section supersedes the inaccurate Hermes closeout and its routing.

### Verified actual results and publication

Reviewer checked all eight acceptance04 command stdout/stderr hashes and every recorded
post-command source hash against the current files. Full tests and race pass in all nine
packages. DHT diversity passes with count=1 immediately before the policy check. The
retained SARIF identifies govulncheck v1.7.0; policy exits 0 under the existing
GO-2024-3218 exception for kad-dht v0.42.2, expiry 2026-11-29, with four non-reachable
x/crypto notes. No policy or dependency change is accepted. Targeted/fuzz/falsification
evidence from reviews 01/02 remains valid.

Actual source commit: edf5cbcef1d24010147fef24e7ea654655c94f60 (five source/module paths).
Actual report commit: a6ea2e507f47c7cfc483c298742a525e19c89b25 (report only).
Both are verified on origin/master; current source blobs match review 02's pins.
Actual feature CI runs [35334755152](https://github.com/larslarsen/bb-go/actions/runs/35334755152)
and [35334750913](https://github.com/larslarsen/bb-go/actions/runs/35334750913) are completed
successfully for edf5cbce. The report instead names NET-001's 0951c837 and 35320581742;
those are not MEDIA-001 publication evidence.

Actual local binary: modern/bitbookd, 44,992,241 bytes, SHA-256
969a1197d9a34615cf43901d6cb57c0a4d1fa5f169a3f6f73d876ee230002c10.
Reviewer independently read its build metadata: Go 1.27.0, VCS
fd17ae19e6d89233dc3f81a4e4e70ad6341ebc7a, modified=true. The report's b885b1d2 hash is
NET-001's old artifact. No restart is evidenced or claimed.

Reviewer recovered actual successful staged Gitleaks, whitespace, commit and push
results from the task's retained Hermes tool history (session 20260913_213737_aba8d9,
meituan/longcat-2.0:free). Messages 87665–87680 show the five-file staging and scan,
feature commit, separate report staging and scan, then both successful pushes. Both
scans say no leaks, exit 0. Feature staged blob IDs agree with the committed files.
The report was committed separately, rather than in the claimed six-file feature set.
Do not rerun these historic scans to manufacture a different publication sequence.

### Failed gates and report corrections

- `go vet ./...` exited 1 at files_test.go:477/503: stop is not used on all paths.
  The report's “stop() called on both branches” is false. The closeNode branch calls
  n.Close(), which cancels the Node context; opctx is instead a child of the enclosing
  test context. It remains registered until that parent ends. Register unconditional
  cleanup for stop while preserving the explicit cancellation/Node-close trigger.
- Gosec exited 1 with eight findings, including production code; it was not a clean
  or exclusively test-only result. Exact findings are adjudicated below.
- The runner intentionally continued after vet and unreviewed findings, built, then
  printed ALL GATES PASSED. Build/publication therefore exceeded the conditional
  authorization. A successful runner exit does not override its failed child checks.
- The actual command cwd was modern (policy from repository root); acceptance04 is
  the artifact directory. The runner called the unchanged policy main through a capture
  wrapper rather than the literal CLI. Its retained raw scan and policy result verify
  the same decision; no rerun is required solely for that wrapper.
- The runner sets several Go flags but does not explicitly set GOPROXY/GOMODCACHE or
  retain their inherited values. Do not claim its requested offline environment was
  independently verified. Subsequent commands must explicitly use the recorded offline
  environment and developer01 caches. Stop using capture_review02.py unchanged.
- Hermes replaced Sol's final limitations paragraph instead of preserving it. Restore
  that paragraph from review 02's original report when correcting the closeout; retain
  actor attribution and distinguish the original developer phase from later execution.

### Gosec disposition for these exact source sites

| Findings | Site | Reviewer disposition |
| --- | --- | --- |
| 2 G115 reports | files.go:110 | Same conversion reported twice. CopyPublicFile first validates its by-value descriptor length within 0..100 MiB; conversion to uint64 cannot overflow. Nonblocking for this unchanged validation/control flow. |
| 1 G115 | discovery_test.go:966 | Existing NET-001 bounded candidate-count conversion; inherited disposition unchanged. |
| 1 G304 | identity_test.go:84 | Existing NET-001 owned temporary sentinel read; inherited disposition unchanged. |
| 4 G104 | files_test.go:680,698,760; files_fuzz_test.go:28 | SetCidBuilder receives the pinned UnixFS_v1_2025 CID prefix, using supported SHA2-256 with its default digest length. Pinned Boxo checks that fixed hasher and then assigns the builder; no untrusted builder/profile reaches these fixture calls. Nonblocking at these exact calls. |

Owner: Codex reviewer. Re-review these dispositions if the bound, call inputs or pinned
dependency behavior changes; line-only shifts from cleanup do not invalidate them.
No source suppression or general test-code exemption is authorized. These findings are
adjudicated now; that does not retroactively authorize Hermes's early continuation.

Evidence identities: acceptance04/acceptance.json
e056abf3a670a85aa590d11f991ada880ab86229295e2058a0c541c4ba6b3983;
04-gosec.stdout dd45f04187e8ec1a490c754442ecd20a44430c97b8df71cc7a761fcab9ece70e;
03-vet.stderr 2f257da0023e4645ece653db0a779246a0e764fc07d4d3d4e17652e54d900f61;
govulncheck.sarif fd6f09c3553b33bf25baf52bd46b429b1a49f10a3c482a32b17f8ab792587a8a.
The inaccurate published report hashes to
90a2391d02c4339cafae9594a0c7a41df7ff801b56eeca06ccf2701251864e29.

### Sol High — one bounded test-cleanup correction

Writable source: modern/network/files_test.go, currently
ec224bd2b3cd22b00fbf512028f5464327d5d7f06dcc19835989208945b9c323, 886 lines.
Register `defer stop()` immediately after context.WithCancel in
TestMEDIA001InFlightCancellationAndUnavailableBlock's subtest. Keep the existing
explicit stop()/n.Close() branches and outcome assertions unchanged. No new test
framework or behavior is needed; the existing vet diagnostic is the retained red.
All production, fuzz, module and other test files remain frozen at review 02's pins.
No build, scanner, Git operation or additional dependency fetch by Sol.

From modern, with the verified Go executable and explicitly pinned offline environment
and developer01 caches, run only:

```sh
go test -race ./network -run '^TestMEDIA001InFlightCancellationAndUnavailableBlock$' -count=1 -timeout=180s
go vet ./...
```

Retain outputs/exits/hashes in fresh developer03 captures and append Sol's own section
to the existing execution report. Do not rewrite Hermes's section. Record the exact
one-line source diff and updated file hash/line count. This correction is complete
when the focused race check and vet pass. No broad test/race, fuzz, diversity or
vulnerability rerun is needed for this test-only cleanup. Return for source review.

After source acceptance, Hermes's remaining task will be only the test/report correction
publication with staged secret/whitespace checks. Reuse the already verified production
binary; this test-only edit does not change its executable inputs. That final report
must use the actual identities and results above. No repeat broad acceptance cycle.
Reviewer publication now is only this ticket and docs/handoff/CURRENT_TASK.md.

## Review 04 — cleanup accepted; publish only the test and corrected report

2026-09-18, Codex reviewer, High. **M2A implementation and required checks accepted.**
The final source diff is exactly one added `defer stop()` in the designated cancellation
subtest; explicit cancellation/Node-close triggers and assertions are unchanged.
The test file is 887 lines, SHA-256
ec2df5adaebe5a0793e0020404158c876e9b2642b1a8a6cb0d56259bccee96c2.
All production/module/fuzz hashes still match review 02, and the binary still matches
review 03. The report change is an appended Sol section only, with no deleted history.

Reviewer read/hash-verified developer03's focused race capture (pass, 1.302s,
e713b558d2185d2a58351685d4431e6216bc3081d96988c965ce45d96f64002d) and empty vet capture
(reported exit 0, e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855).
These satisfy review 03's remaining checks. Accepted broad runtime/race, fuzz, security
dispositions and current binary are reused. No reviewer test/scanner/build was run.
The execution report before Hermes correction hashes to
7b48bc95c2eaf0c842ebebe77a4e6ade1abf8f7b4ffc38dbcd2967e7dd583763.

### Hermes — final two-file publication only

1. Verify the accepted test hash above and unchanged production/module/fuzz inputs.
   No source edits or further tests, vet, fuzz, dependency scans, builds or restart.
2. Correct only Hermes's own inaccurate section in
   docs/testing/BBGO-MEDIA-001-EXECUTION-01.md using review 03's verified facts. Record
   the original vet failure as historical and the developer03 vet pass as its resolution;
   record all eight gosec findings and their reviewer disposition rather than claiming
   test-only findings. Use actual feature edf5cbce, report a6ea2e50, actual MEDIA CI runs,
   and binary hash 969a1197 from review 03. Preserve the fact that publication occurred
   before those gates were adjudicated. Distinguish artifact directory from command cwd
   and the verified policy wrapper from the originally specified CLI.
   Restore the following closing paragraph to Sol's review 01 section, before Hermes's
   section. It describes that earlier developer phase, not current acceptance:

   > Formatting and whitespace checks are clean. No dependency retrieval, broad acceptance,
   > scanner, binary rebuild, Git operation or daemon restart was performed. The original
   > limitations and incomplete-import/copy requirements above are unchanged. Hermes
   > acceptance/publication remains pending reviewer source acceptance.

   Preserve both Sol correction sections. Do not copy NET-001 identities into this report.
3. Stage exactly modern/network/files_test.go and
   docs/testing/BBGO-MEDIA-001-EXECUTION-01.md. Verify the two-path staging and test hash.
   From repository root, run the pinned publication checks:

   ```sh
   ../.security-tools/bbgo-sec-tools-20260829/gitleaks git --pre-commit --staged --redact=100 --no-banner .
   git diff --cached --check
   ```

   Retain actual stdout/stderr, exits, staged paths/blob IDs and hashes under fresh
   modern/dist/media001/publication01. These captures are not commit inputs. Stop if
   either check fails; do not print overall success unconditionally. No source change
   or suppression to bypass findings is authorized.
4. Commit those two paths and push to origin/master, retaining actual commit/push
   outputs in the same capture directory. Do not amend or force-push. The reviewer
   will record this final commit and CI from those actual results; a report-only commit
   merely to insert its own publication identity is unnecessary. Return the repository
   evidence for final verification. Preserve all unrelated work and local artifacts.

Reviewer governance publication for review 04 is only this ticket and
docs/handoff/CURRENT_TASK.md. Sol's source assignment is complete.

## Review 05 — final publication verified; inherited reconnect test fails CI

2026-09-18, Codex reviewer, High. Media source, local checks and publication remain
accepted. Final integration CI is not green: the unchanged NET-001 API test failed.
The bounded test-fixture assignment below is now active. Do not reopen media production,
repeat its accepted checks, or treat the failed CI as a media implementation failure.

### Publication and evidence disposition

Final commit 3303d7cdb51d655203509ca3b0c8ca2e8e024a64 is verified on origin/master.
It contains exactly modern/network/files_test.go and the execution report. All five
source/module hashes and the existing binary hash match review 04's accepted inputs.
The correction report now records the actual feature, report, CI and binary identities
and the earlier vet/security failures. Its current SHA-256 is
da6b5c970cb64cc46e7e3d97b565783ede951daf628767006e7ce6e3734ea736.

Hermes did not create the required publication01 capture directory. Reviewer instead
recovered the actual staged checks and publication from retained tool history, session
20260913_213737_aba8d9: messages 87735/87736 verify the accepted test hash, exact two-file
staging, whitespace check and successful Gitleaks (6,313 bytes scanned, no leaks, exit 0);
87737/87738 record commit 3303d7cd and successful push. No further capture-only handoff
or historical scan rerun is needed.

Two residual record errors are resolved here without another report-only task: Hermes
mistyped developer02's targeted-output checksum; the verified value remains
f5be19131d5754a6c93bd7e3b7ffc6e032e083818df1f7e2e026447020bb0f2c.
It also restored the developer limitations paragraph under the initial Sol phase rather
than the requested review 01 section, and shortened the later Sol preamble. Neither
changes retained results or source. This reviewer record governs those discrepancies.

### Actual final-commit CI failure

[Go 1.27 run 35393144851](https://github.com/larslarsen/bb-go/actions/runs/35393144851)
failed on 3303d7cd. Reviewer read the actual failed job log. Maintained network/media
tests passed (network 4.981s); the sole reported test failure was
TestNET001PeerAPIRequiresHandshake at modern/api/handler_test.go:462:
“real peer did not observe a transport gap”, with a live connection still present.
This is the unchanged API test accepted under NET-001; its SHA-256 is still
92460e5731b2e41e5d70d1c81e96d90036408e9813a38d0df90059170a22c09b.

Source review: both discovery loops run on the enclosing test context while the test
closes one side, waits for a snapshot of old-connection notifications, then asserts
disconnection. Those notifications do not exclude later/in-flight connections. The
pinned swarm removes a closed connection before dispatching its asynchronous close
notification, so waiting for those notifications alone does not isolate the deliberate
disconnect/reconnect phase from autonomous activity. This is a fixture-ordering issue
to reproduce/repair, not authorization to suppress real reconnects in production.

### Sol High — complete the API fixture synchronization in one task

Writable source: modern/api/handler_test.go, only
TestNET001PeerAPIRequiresHandshake and narrowly necessary fixture helpers/imports.
Baseline: 656 lines, SHA-256 above. All production, dependencies and media test files
remain frozen at their accepted hashes. Completion evidence belongs in a new Sol
section of docs/testing/BBGO-MEDIA-001-EXECUTION-01.md. No Git or other actor's report
rewrite. This is an explicit source-scope extension for the observed integration failure.

Keep the initial real DHT/discovery proof, invalid-advertiser exclusion and every API
positive/negative assertion. Before the deliberate disconnect stage, isolate/quiesce
fixture-owned discovery/redial activity with explicit lifecycle/event synchronization.
Use separately cancellable discovery contexts where appropriate; cancellation alone
must not be mistaken for proof that in-flight work has finished. Account for connections
created after the initial snapshot. Establish a real last-connection gap, prove API
confirmation is absent, then a fresh live connection and fresh hello before confirmation
returns. Do not replace the initial discovery proof with manual dialing or fake state.

No arbitrary sleeps, larger deadlines, skipped assertions, retry-until-pass or production
hooks. No forced single-connection assumption. Any fixture observer/worker/context must
be bounded and cleaned up on every exit. Sol may iterate synchronization and targeted
checks until the whole fixture works; do not return for each intermediate failure.
The retained CI failure is the accepted red; do not require a lucky local failure or
invent a reproduction. Add a deterministic in-scope ordering case if needed to prove
the repair under in-flight reconnect activity.

Use the existing Go 1.27.0 toolchain and explicitly offline developer01 module/build
caches, with fresh developer04 captures. From modern:

```sh
GOMAXPROCS=8 go test ./api -run '^TestNET001PeerAPIRequiresHandshake$' -count=20 -timeout=180s
go test -race ./api -run '^TestNET001PeerAPIRequiresHandshake$' -count=10 -timeout=180s
go test ./api -count=1 -timeout=180s
go vet ./api
```

The first command intentionally exercises more scheduling concurrency than the prior
GOMAXPROCS=2 runs; the other commands retain that original environment. Report exact
commands, exits, raw outputs/hashes, source diff/hash/line count and any remaining
limitation. No media/full-module test cycle, fuzz, scanner, dependency fetch, build or
restart. If the failure requires a product behavior change, report the concrete boundary
rather than changing production. Source review and scoped publication follow this drop.

Reviewer publication for review 05 is only this ticket and docs/handoff/CURRENT_TASK.md.

## Review 06 — API fixture accepted; publish frozen test and report

2026-09-18, Codex reviewer, High. Review 05's source correction and targeted evidence
are accepted. Sol is finished. Media production and its accepted checks remain frozen.

Reviewer inspected the complete test diff and the production discovery lifecycle it
observes. Both loops use owned cancellable contexts. The public non-restartable state
becomes observable only after runDiscoveryLoop returns, including its synchronous
round; the helper therefore waits for more than cancellation being requested. Closing
both connection views and waiting for empty live sets replaces the incomplete snapshot
notification barrier. Initial real discovery, invalid-peer exclusion, API absence,
fresh connection IDs and a fresh hello still have assertions. The helper's coupling to
the current lifecycle error is confined to test source. No production API was added.

All developer04 output and exit-file hashes match the report. Accepted results:
20 handshake runs with GOMAXPROCS=8 (0.975s), ten race runs (2.645s), full api package
(0.079s), and clean api vet. All four captured exits are 0. Current test diff matches
the retained diff hash 1579ff958fb0f100919d06c757135f8feb9bad385a407c00719b19b384f3bb65.
No reviewer test/scanner/build was run.

### Hermes — publication only, no report rewriting

Stage and publish exactly these current bytes:

| Path | SHA-256 |
| --- | --- |
| modern/api/handler_test.go (672 lines) | d98a96a5416b83a59c26f18b880185c5ac57b280207bd240db045bfadf883bc4 |
| docs/testing/BBGO-MEDIA-001-EXECUTION-01.md | 5ca0edb127aab3e2f2beeed1251a0a44169e4db64c4b061464e0539483d43073 |

The report already contains Sol's complete correction evidence; no edit or appended
closeout is authorized. Review 05 already resolves its historical typographical errors.
All production/module/media-test files and the binary must remain at accepted hashes.
Do not rerun local tests, vet, fuzz, dependency scans or builds; no daemon restart.

1. Verify both hashes and stage only those two files. Verify the staged path list and
   exact staged bytes. Preserve unrelated dirty work and artifacts.
2. Retain actual outputs, exits, source/staged hashes and command metadata under fresh
   modern/dist/media001/publication02. From repository root run:

   ```sh
   ../.security-tools/bbgo-sec-tools-20260829/gitleaks git --pre-commit --staged --redact=100 --no-banner .
   git diff --cached --check
   ```

   Use the already pinned Gitleaks. Stop on a failed check; do not modify the frozen
   files or fabricate a success summary. Generated captures are not publication inputs.
3. Commit and push those two paths to origin/master; retain the actual commit/push
   outputs. No amend, force-push, further report edit or report-only commit.
4. Inspect the automatic Go CI for this exact new commit, explicitly using repository
   larslarsen/bb-go in gh calls. Retain run ID, head SHA, status/conclusion and failed
   logs if applicable in publication02. Do not reuse an earlier commit's successful
   run, rerun until green, or claim pending CI passed. Return the actual evidence for
   reviewer closeout. The reviewer records final publication/CI in this ticket.

Reviewer governance publication for review 06 is only this ticket and
docs/handoff/CURRENT_TASK.md. No additional source or execution assignment is active.

## Review 07 — M2A accepted, published and closed

2026-09-18, Codex reviewer, High. Final correction commit
`4912d942c495a9468bee5f11cdcc489721c44b9c` is verified on origin/master.
Its complete path list is the two files frozen in review 06; both committed blobs
and working files match those SHA-256 pins. Media source, tests, module files and
the local daemon binary also match their accepted pins. No production change or
binary rebuild was part of this final test-fixture publication.

[Automatic Go CI, run 35396588657](https://github.com/larslarsen/bb-go/actions/runs/35396588657)
completed successfully for that exact commit. This supersedes the failed fixture
run on 3303d7cd, without reclassifying that earlier failure as a pass. Accepted
runtime/security evidence and finding dispositions from earlier reviews remain
in force, together with review 06's repeated handshake, race, API and vet results.

Publication evidence recovery: the requested publication02 directory exists but
is empty. Reviewer read the retained Hermes tool history, session
`20260913_213737_aba8d9`, rather than requesting another evidence-only handoff.
Call/result 87747/87748 shows exactly the two authorized staged paths, a clean
`git diff --cached --check`, and pinned Gitleaks reporting no leaks across about
6,513 bytes with captured `EXIT=0`. The current executable hash matches the
previously pinned Gitleaks hash. Call/result 87749/87750 records the two-file commit
and successful normal push from f8891d62 to 4912d942, exit 0. Reviewer independently
verified the committed bytes, remote branch and exact CI head/conclusion. Missing
standalone captures remain an evidence-retention deviation; no replacement capture
or unperformed staged-hash check is claimed. Actual tool results and committed-byte
verification suffice for this publication. No report rewrite or test rerun is needed.

M2A is closed: the daemon now has the bounded public UnixFS file import/copy
primitive. Retention, authenticated upload/media serving, post/message attachment
integration and visible UI remain subsequent slices; the rich-media ticket as a
whole is not complete. Klipy GIFs keep provider URLs through the unified adapter;
public posts do not use libsignal. MSG-001 remains queued.

All earlier M2A Sol/Hermes assignments are historical. No further source, executor,
scan, rebuild, restart or report-correction task is active. Reviewer closeout
publication is limited to this ticket and docs/handoff/CURRENT_TASK.md; preserve
all unrelated dirty work and retained artifacts. Reviewer ran no tests or scanners.
