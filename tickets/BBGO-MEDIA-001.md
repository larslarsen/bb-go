# BBGO-MEDIA-001 — rich media posts, messaging and IPFS attachments

Status: **ACTIVE — M2A file storage primitive assigned to Sol High; later slices queued.**
Owner request recorded 2026-09-17. Reviewer: Codex, High.
Companion: [BBGO-MSG-001 — libsignal messaging](BBGO-MSG-001.md).
NET-001 is accepted and closed. The M2A assignment below is the sole active source
authorization. This ticket is the handoff; no separate developer handoff is needed.

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
