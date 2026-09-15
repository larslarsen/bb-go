# BBGO-PAY-003 — Authenticated local access to received payment requests

Status: PUBLISHED — REVIEWER FINAL VERIFICATION. Reviewer: Codex.
Review: [source review 02](../docs/testing/BBGO-PAY-003-SOURCE-REVIEW-02.md).
Evidence review: [acceptance review 03](../docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-03.md).
Publication: [publication 01](../docs/testing/BBGO-PAY-003-PUBLICATION-01.md).
Feature commit: `82ed5f9c62ab22687a4972ba0ad59731bf43013e` on origin/master.
CI: [Go 1.27, run 35017544700](https://github.com/larslarsen/bb-go/actions/runs/35017544700) passed.
The original contract below governs behavior; publication 01 is current authority.

## Outcome and scope

A running Linux daemon provides an authenticated local client with its stored signed
payment records. This is the daemon half of the desktop request inbox. Desktop
wiring follows after review of this API. No sending, cancellation, wallet access,
coin signing, submission, or paid-status acceptance is added here.

The existing social HTTP API permits wildcard CORS and renderer-selected endpoints.
Payment records therefore use a separate loopback listener with a per-run credential.
Reuse payment.Service.List and its RecordedObject schema; preserve signed canonical
bytes. This is private request metadata, never a generic wallet HTTP service.

## Baseline and authorized paths

bb-go HEAD: 2b695a8719a7381f3919bb83c63e54251f43536d.
bb-desktop read-only HEAD: 7a31c41cb29692a94acf1f24adb379f3a829d237.
Preserve all pre-existing dirty files, including cancelled DEV-001 drafts. Do not use
the draft build helper, edit dependencies, or integrate unrelated work.

Tests first:
- modern/localclient/server_test.go (new)
- modern/cmd/bitbookd/localclient_test.go (new; reuse PAY-002 child-process helpers)

Then production:
- modern/localclient/server.go (new; listener, credential lifecycle, read handler)
- modern/cmd/bitbookd/main.go (small startup/shutdown integration only)

Starting main.go SHA-256:
6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b (225 lines).
Frozen payment/service.go SHA-256:
a523b6b886d9eeae4468a2eaf3609c698dd919981ac2dc653bf23bb4e0d59bea.
Frozen api/handler.go SHA-256:
70bac95bbde93613e5d1759e5cd826d9d1dbe8b9136acfb6720658ad93a0fc6c.

## Fixed trust and wire contract

1. Bind TCP to 127.0.0.1:0, independently of the social -api setting. Generate a
   fresh 32-byte cryptographic random token and 16-byte instance ID on each start,
   encoded lowercase hex. Never accept these from HTTP, argv or environment.
2. After network.Open has acquired the datastore, create/open local-client beneath
   that data directory. Require an actual private directory (0700); reject symlinks
   and unsafe existing permissions, without changing existing user permissions.
   Use directory-rooted file operations as existing identity persistence does.
   Publish connection.json atomically through an exclusively created 0600 temporary
   regular file. Descriptor schema is exactly:
   {"v":1,"endpoint":"http://127.0.0.1:<bound-port>","peer_id":"<node ID>",
    "instance_id":"<32 hex>","token":"<64 hex>"}.
   Publish only after a successful bind. Replace stale descriptors atomically;
   do not follow symlink destinations or read prior tokens. Never log the descriptor
   body, credential, request contents, or authorization header.
3. This first implementation enables this channel on Linux, where its permission
   contract is defined. Other platforms retain normal social/P2P startup and report
   local payment access unavailable; do not assume POSIX modes enforce Windows ACLs.
   On Linux, credential setup/bind failure must report an error and close owned
   resources. Never fall back to unauthenticated access.
4. The listener serves only GET /v1/payment/records, with no query parameters or
   request body. Authenticate before touching storage or returning route details:
   exactly one Authorization header, value "Bearer <token>" (constant-time token
   comparison), and one X-BitBook-Instance header matching this run's instance ID.
   Missing, malformed, duplicate or wrong credentials/instance yield 401. Any Origin
   header, including null or empty, yields 403. Require loopback RemoteAddr and exact
   bound host:port; reject mismatches with 403. No proxy trust, redirects or CORS.
   Authenticated wrong methods yield 405, unknown paths 404, query/body input 400.
5. Success is JSON {"v":1,"peer_id":"...","instance_id":"...","records":[...]},
   with records serialized as existing payment.RecordedObject values. Return [] for
   an empty store. Include inbound/outbound requests and accepted cancellation
   records, without interpreting a peer claim as payment confirmation. Storage
   failures return 503 {"error":"UNAVAILABLE"}; no partial success or raw errors.
   All responses use Cache-Control: no-store. Successful JSON is limited to 4 MiB;
   detect overflow before sending 200 and return 503 {"error":"TOO_LARGE"}. Do not
   silently truncate or add pagination/index/storage migrations to this slice.
   Service.List already loads the store; this wire cap is not a claim of bounded
   datastore memory use. Set bounded header/read/write/idle timeouts and a 16 KiB
   maximum request-header setting. Other error bodies contain only a stable code.
6. Start the channel using the already-created payment service. Shutdown stops its
   HTTP handlers before closing payment.Service or the datastore, with a bounded
   graceful shutdown and forced close on timeout. Remove only this instance's
   descriptor and owned temporary file; retain the directory. Unexpected listener
   failure is surfaced to run(), not silently ignored. A stale descriptor after a
   crash is harmless: new startup rotates both credentials. Never restart or signal
   the owner's running daemon during development.

The subsequent desktop bridge will read the descriptor from a main-process-selected
data directory, never from renderer API settings. It must keep credentials out of
preload/renderer, disable redirects/proxies, and check peer/instance in the response.
The local OS account and private data directory are trusted here. This channel does
not claim protection from compromise of that account or provide mutual TLS. Its
credential authorizes reading records only; future mutations need explicit review.

## Development and acceptance

Grok Build 4.6 High owns the four source paths above. Author focused tests first,
run the red command, implement, format only those paths, and iterate the same focused
command to green. A missing package/symbol before implementation may be the initial
red; the completed tests must exercise actual rejection and successful record reads.
Write the source inventory and exact command results into
docs/testing/BBGO-PAY-003-GROK-CORRECTION-01.md for Codex review. A chat completion
notice may point to that document; it does not replace it. No Git operations.

Use a compact table for authentication/origin/host/method/input failures alongside a
successful nonempty read. Assert denied requests never call the record reader. Cover
storage failure, response-size boundary, safe credential publication and cleanup.
One real daemon test uses existing local-peer helpers: deliver a signed request over
libp2p, discover the private descriptor, read the exact signed record over this HTTP
listener, restart, and prove persistence with changed credentials and rejection of
the previous credential against the new listener. Check credentials are absent from
captured logs. Use owned test data and loopback peers only; no new process harness.
One small fuzz target exercises auth/header parsing with a fake record reader;
unauthenticated variants must never reach it. Do not duplicate codec/crypto tests.

Workdir modern. Commands use GOTOOLCHAIN=go1.27.0 and GOWORK=off. For large temporary
work use disk-backed modern/dist/pay003-tmp, after checking its filesystem type;
set TMPDIR to that absolute directory. Never use cancelled DEV-001 tools.

Focused red/green (Grok):
`go test ./localclient ./cmd/bitbookd -run 'TestLocalClient' -count=1`

After source review, Hermes runs final acceptance once:
- `go test -race ./localclient ./cmd/bitbookd ./payment ./api -count=1`
- `go test ./localclient -run '^$' -fuzz '^FuzzLocalClientAuth$' -fuzztime=10s`
- Falsify the credential check once, prove the focused rejection test fails, restore
  exact production bytes and rerun that test. No separate relay for each step.
- `go vet ./localclient ./cmd/bitbookd`
- gosec v2.29.0: `gosec ./localclient/... ./cmd/bitbookd/...`
- From repository root, using govulncheck v1.7.0:
  `python3 scripts/govulncheck_policy.py source`
- Scan the explicitly staged ticket paths using Gitleaks v8.30.1:
  `gitleaks git --pre-commit --staged --redact=100 --no-banner .`
  New findings block acceptance; preserve existing reviewed dependency policy.

Hermes records concise evidence in docs/testing/BBGO-PAY-003-ACCEPTANCE-01.md and
stages only the four source paths, this ticket, that evidence and the current bb-go
handoff. Codex reviews source/results before publication authorization. Do not pull
cancelled helper drafts or unrelated governance changes into this commit.

Routine completion includes building modern/bitbookd from accepted source:
`go build -mod=readonly -o bitbookd ./cmd/bitbookd`.
Verify `go version -m bitbookd` and its output path. No build-helper work, additional
build tests, separate build ticket, or automatic process restart.
