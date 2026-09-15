# BBGO-PAY-002 phase A — actual daemon payment-request tests

Status: SUPERSEDED by
[correction 01](GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_CORRECTION_01.md).
The first drop was rejected in
[review 01](../testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-01.md).
Retained as the original contract and frozen-input inventory; its create-only
instruction and persistence sequence are replaced by the active correction handoff.

Actor: Grok Build 4.6 High, manually launched by owner. Reviewer: Codex.
Read bb-go AGENTS.md, TESTING.md, CURRENT_TASK.md and tickets/BBGO-PAY-002.md.
This repository's role rules apply: author test source only; no test execution or Git.

## Exact scope and baseline

HEAD must be 801f5d55d80fe02c6eb512ff35f8c09acfd679af. Create only
modern/cmd/bitbookd/payment_test.go, which must not already exist. Preserve the existing
untracked modern/bitbookd binary and all unrelated files. These inputs stay frozen:

| Path | Lines | SHA-256 |
| --- | ---: | --- |
| modern/cmd/bitbookd/main.go | 219 | 9c7aba19576d162b322dce3dddb61990f8d2a5a65f12b681100bf6e23f8a12ab |
| modern/payment/service.go | 687 | a523b6b886d9eeae4468a2eaf3609c698dd919981ac2dc653bf23bb4e0d59bea |
| modern/payment/types.go | 121 | f3aea3cd15e04e80af3d22f2d00f8c9ba73b3d31ddf1c7ddeafbe2ee8c51226d |
| modern/payment/transport_test.go | 397 | 65fc2eeb90c967d59f1e00b514a47c48edbf4ee2d366f512157391c43c811362 |
| modern/payment/service_test.go | 816 | 3e059272d670ae0b3f74b2fad9613aea3aef7b62bf212569361dd852cf7882c9 |
| modern/network/protocols.go | 24 | 502c6224e135f6342f1501d13783abefcf343c19a280975b75d6b43d04f95669 |
| modern/api/handler.go | 653 | 70bac95bbde93613e5d1759e5cd826d9d1dbe8b9136acfb6720658ad93a0fc6c |
| modern/go.mod | 133 | 1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783 |
| modern/go.sum | 374 | 4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a |

## Required real boundary

Use package main. Add two named tests:

- TestPaymentDaemonReceivesAndPersistsRequests
- TestPaymentDaemonRejectsWrongPayer

A guarded child-helper test may re-exec the current test executable using os.Executable
and a unique exact -test.run filter. The helper calls the existing run() function with
an isolated FlagSet/os.Args and exits through its normal signal-driven lifecycle. It
must not create payment.NewService or install a stream handler itself. Without its
explicit child marker the helper returns immediately. No production test seam or binary
build/spawn command beyond the test executable is needed.

Child arguments: owned data-dir, --listen /ip4/127.0.0.1/tcp/0,
--api 127.0.0.1:0 and --allow-private; no bootstrap peers or inherited user config.
Use bounded stdout/stderr capture to discover the real peer ID and numeric loopback
multiaddress from startup output. Assert ordinary daemon readiness separately from
payment negotiation, so absent protocol support is the intended red, not a startup bug.
All waiting, dialing, streams and process shutdown must have bounded deadlines.

Parent creates an ordinary local network.Node representing the payee and uses existing
public payment codec/signing/frame APIs; no copied implementation of signature/replay
logic. Reuse a valid public test receiver from the existing payment fixtures, with fresh
test request IDs/nonces and the actual two peer IDs. Because the production daemon uses
its clock, create request timestamps relative to test start with a generous valid window;
do not assert narrow timing races or depend on a public clock/server. Amount is a small
synthetic coin-denominated value; this request cannot move funds.

### Receives and persists

1. Connect the parent peer to the actual child daemon and send the signed payer-bound
   request over /bitbook/payment/1.0.0. Require a positive acknowledgement with the
   independently computed request digest, not just a successful stream write.
2. Resend identical signed bytes and require the same successful digest. Stop the
   daemon by SIGTERM, wait for normal exit, and restart against the same owned data-dir.
   Assert the peer ID is unchanged, then resend and require the same idempotent result.
3. Stop and fully reap that daemon before inspecting its datastore. Open only this
   test-owned data-dir with the existing network/payment APIs and assert exactly one
   inbound request record with the original canonical bytes, signer/public key,
   signature and digest. This post-stop inspection is permitted; it cannot substitute
   for the earlier real daemon delivery. Close all inspection services/nodes.

### Rejects wrong payer

1. Establish a successful valid-request delivery through the same actual daemon path
   as a positive control. Otherwise absent payment-service wiring could falsely look
   like successful rejection.
2. Sign a distinct well-formed request with the genuine payee key but bind payer_peer_id
   to a third local fixture identity. Send the envelope directly through the real
   framed stream to the child. Do not use SendSigned for this invalid case: its local
   preflight could reject before exercising the daemon. Require the remote rejection
   acknowledgement and stable PAYER code, not a generic disconnect or timeout.
3. After normal shutdown/reaping, inspect only the owned datastore and prove the
   invalid request was not retained and the valid control is still intact.

## Isolation and cleanup

Use the test framework's owned temporary directories. A later executor must place its
test temp parent/cache on a verified disk-backed location under modern; do not allocate
large data in RAM-backed /tmp. Test cleanup must never target a user directory or the
existing modern/bitbookd file. Reap every child, close pipes/readers/streams/nodes, and
finish bounded reader goroutines on success and failure. If graceful child shutdown
fails, bounded kill-and-wait of that exact owned child is allowed inside test cleanup.
Do not assert private key contents or print full request/signature material in failures.

No HTTP payment API, chain client, broker call, wallet data, mainnet, public peer,
cryptographic change, paid-receipt enablement, retries or production modification.

## Return without execution

Return the test file hash/line count, test names, frozen-input verification and a brief
explanation of how missing daemon registration fails non-vacuously. Do not run gofmt,
tests, binaries, scanners, downloads or Git mutation under this source-only phase.
Codex reviews the test source before authorizing the repository's executor to capture
the expected red. Production wiring follows only after that boundary is established.
