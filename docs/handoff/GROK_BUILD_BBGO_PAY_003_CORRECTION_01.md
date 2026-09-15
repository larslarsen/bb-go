# PAY-003 source review and bounded correction

## Closed addendum — completion documentation only

Grok's completion report is present and reviewed in
[acceptance review 02](../testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-02.md).
No further Grok work is authorized; the instructions below are historical.

The source correction below has been statically reviewed in
[source review 02](../testing/BBGO-PAY-003-SOURCE-REVIEW-02.md). Its source/test
execution authority is closed. The owner requires Grok's completion report in the
repository control plane; asking the owner to paste evidence into chat was incorrect.

Grok Build 4.6 High must write exactly
`docs/testing/BBGO-PAY-003-GROK-CORRECTION-01.md`. This is a narrow authorization
to document its own completed work, not Hermes's acceptance evidence. Read the
existing source and retained execution records as needed. Keep all source, tests,
dependencies and other documents unchanged. No new test execution, Git mutation,
scanner, build, process operation or implementation is authorized by this addendum.

The report must contain:

- actual actor/model, task baseline and changed paths;
- final source hashes and line counts, identifying any difference from review 02;
- each actual correction command, working directory, relevant environment,
  exit result, observed test counts and meaningful diagnostics;
- the requested pre-fix red, focused green and fuzz-seed outcomes separately;
- repository-relative retained log references where available, and explicit
  identification of results that were not run or cannot be recovered.

Use only actual prior execution records, including Grok's own retained tool output.
Do not invent results, rerun commands to reconstruct history, or present current
hashes as verified execution-time hashes without evidence. An unavailable result
must be documented as unavailable so Codex can decide the next step.

Completion is the saved repository report. Return its path as the completion notice.
Codex reads the report before authorizing advancement. Hermes acceptance 01 remains
inactive until that review. The original correction contract below is historical.

## Original correction contract — historical

Reviewer: Codex. Decision: changes required before Hermes acceptance.
Actor: Grok Build 4.6 High, owner-relayed. This is one correction pass with the
existing focused development command. No separate test-only handoff.

## Findings

1. **Reject parsed chunked bodies (P2).** server.go:396 checks ContentLength > 0
   and the raw Transfer-Encoding header. Go's HTTP parser removes that header and
   puts chunked framing in Request.TransferEncoding, with ContentLength == -1.
   An authenticated chunked GET therefore reaches List and returns records instead
   of 400. The current body fixture supplies a known content length and misses it.
   Inspect the parsed framing fields; reject unknown/chunked body framing without
   reading or draining the body in the handler. Add one real-listener regression
   using a streaming request body with unknown length, asserting 400 and zero reads.
   Keep the existing bodyless successful read as the positive control.

2. **Remove only the published descriptor owned by this server (P2).**
   server.go:183 unconditionally removes connection.json. If that path is replaced
   after startup, closing this server deletes the replacement. This violates the
   ticket's explicit instance-owned cleanup contract. Retain the published file's
   identity and compare it with the current non-symlink regular entry before removal;
   leave replacements untouched. No prior-token reads or process manager are needed.
   Add one regression that replaces the descriptor atomically with another file,
   closes the server, and proves the replacement bytes survive. Retain the existing
   test that an unchanged descriptor is removed. The private OS account remains
   trusted; no new claim of resisting hostile same-user filesystem races is needed.

These are source findings, not claimed runtime reproductions. The framing behavior
was cross-checked in installed Go net/http/transfer.go: parseTransferEncoding removes
the header, and readTransfer assigns Request.TransferEncoding. Hermes acceptance has
not run. No retained PAY-003 command report was found under modern/dist; report the
actual correction commands and exit results in your return message.

## Small corrections to the existing tests

- FuzzLocalClientAuth currently creates a listener, identity, private directory and
  random credentials on every input, then fuzzes a Host that never matches the newly
  chosen port. Almost all inputs stop at the Host check before testing auth. Reuse
  one fixture per fuzz worker, supply its valid Host/RemoteAddr, and vary credentials
  and origin relative to that fixture. Include a positive seed with a nonempty read
  and negative seeds with zero reads. Preserve a deterministic input-to-outcome
  mapping; do not store generated credentials in corpus files. No new fuzz framework.
- Linux-specific success tests currently fail on other supported host platforms,
  where Start deliberately returns ErrUnavailable. Guard the existing new tests
  and fuzz target appropriately. The unsupported-platform branch may assert
  ErrUnavailable directly; it must not require a Linux listener or POSIX permissions.
- TestLocalClientStorageFailure decodes the body in assertErrorStatus, then reads
  the consumed stream to look for leaked error text. Inspect one captured body, or
  make the existing error assertion require exactly the closed {"error":code}
  object and EOF. Avoid adding a duplicate storage test.

## Scope and verification

Same bb-go baseline: 2b695a8719a7381f3919bb83c63e54251f43536d.
Only these three source files may change in this correction:
- modern/localclient/server.go
- modern/localclient/server_test.go
- modern/cmd/bitbookd/localclient_test.go (platform guard only)

Keep main.go unchanged at its reviewed drop. It starts the local channel using the
existing payment service, handles unsupported platforms, surfaces listener errors,
and defers local shutdown before payment-service closure. No dependency, payment
transport, social API, desktop, helper, or process-harness edits.

Reviewed drop identities (SHA-256, lines):

| Path | SHA-256 | Lines |
| --- | --- | ---: |
| modern/localclient/server.go | 7258ee22c65eb8d53d4387f5408988868ac2b0f6007c310c40b088c77e6f7496 | 416 |
| modern/localclient/server_test.go | f9e2c5dd50959c47a5bb9427fbc9a26cc8f08d0c97cf445baf2fa6727c5658de | 610 |
| modern/cmd/bitbookd/localclient_test.go | 333eb8597c5592243282c016595e913487ac31ad429ba4d9390c6f245733f017 | 191 |
| modern/cmd/bitbookd/main.go | b7b72438c3e41131a5fb9578bc0e22ca12bef11baeac2f3d6a0cdd9429631a20 | 253 |

Seven TestLocalClient functions and one fuzz target were inspected. The daemon test
uses the existing real-daemon/local-peer helpers and checks signed record retrieval,
restart persistence, credential rotation and stale-credential rejection. Frozen
payment/service.go, api/handler.go, go.mod and go.sum still match the prior pins.
Tracked diff whitespace check passed. These observations do not claim test success.

Add the two focused regression cases first and observe their intended failures,
then fix the production code and existing tests above. Format only authorized paths.
From modern, with GOTOOLCHAIN=go1.27.0, GOWORK=off and disk-backed TMPDIR as the ticket
specifies, use the existing command:

`go test ./localclient ./cmd/bitbookd -run 'TestLocalClient' -count=1`

Also exercise the corrected existing fuzz seed corpus:
`go test ./localclient -run '^FuzzLocalClientAuth$' -count=1`

Return exact commands/results and changed file identities for Codex review. Do not
run broad scans or Git operations. Hermes's already-defined acceptance and routine
binary rebuild remain in this same payment task after source acceptance.
