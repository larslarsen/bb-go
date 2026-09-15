# BBGO-PAY-002 phase A — Grok test-source correction 01

Actor: Sr Dev — Grok Build 4.6 High, manually relayed by owner.
Reviewer: Codex. Status: COMPLETE — corrected source accepted in
[review 02](../testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md).
Next authority: [Hermes expected red 01](HERMES_BBGO_PAY_002_EXPECTED_RED_01.md).
The source-only instructions below are historical; no further Grok source work is
authorized by this completed handoff.

Read AGENTS.md, TESTING.md, CURRENT_TASK.md, tickets/BBGO-PAY-002.md, and
[review 01](../testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-01.md).
This supersedes the original handoff's create-only instruction and persistence
sequence. All other original isolation, frozen-input, and protocol requirements apply.

## Exact scope

HEAD: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Edit only the existing `modern/cmd/bitbookd/payment_test.go`.
Starting identity: 555 lines, SHA-256
`201c570a9f36d5c5bec0d739a459869079d74be40519db3ee13dd62c1670e80b`.
Verify the nine frozen inputs against the table in
[the original handoff](GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_01.md).
Preserve all unrelated working-tree changes and the owner-owned `modern/bitbookd`.
If the baseline differs, stop and report the difference.

## Required corrections

1. Parse the daemon's actual timestamp-prefixed peer-ID and p2p log lines without
   changing production logging or suppressing log flags in the child. Retain the
   numeric loopback address and matching peer-ID requirements. Add a small table test
   covering representative timestamped startup output and malformed/mismatched
   identities. Ordinary readiness must succeed before testing payment negotiation.
2. Strengthen persistence before resending: deliver and duplicate the signed request;
   stop/reap normally; restart with the same data directory and confirm readiness and
   unchanged identity; stop/reap that restarted daemon without sending any payment;
   inspect the owned datastore and require the original single inbound record. Close
   all inspection resources, restart again, resend the identical bytes, require the
   same accepted digest, then stop/reap and inspect again. This must detect loss during
   startup even if a later resend could recreate the request. Preserve the canonical
   bytes, public-key, signature, direction, digest, and exact record-count assertions.
3. Give the child helper explicit ownership of stdout/stderr readers and one process
   wait/completion result. Bound graceful shutdown, kill/wait, and reader completion;
   never follow a timeout with an unbounded WaitGroup wait. Close retained readers on
   failure to unblock drains, clean up partial pipe setup failures, and report cleanup
   errors. A forced kill or unconfirmed reap must fail normal shutdown assertions.
   Do not inspect storage or restart while a preceding child is not confirmed reaped.
   Observe early child exit through the wait/completion mechanism, not an unwaited
   ProcessState. Preserve bounded log capture and exact owned-child cleanup.

Keep both named behavior tests, the guarded run() child, and the wrong-payer positive
control and remote PAYER assertion. No production source or additional file edits.

## Return boundary

Return the updated test SHA-256, line count, test names, frozen-input verification,
and how each review finding was corrected. Report any unresolved limitation.
Do not execute tests, gofmt, binaries, scanners, downloads, or Git mutations in this
source-only correction phase. Sr Dev's standing focused-test capability remains;
the explicit review-before-execution gate still applies to this drop. Codex reviews
the corrected source before issuing execution authority. Hermes owns subsequent
integration and acceptance evidence under a separate handoff.
