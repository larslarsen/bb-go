# BBGO-PAY-002 phase A — production-source review 01

Date: 2026-09-14. Reviewer: Codex.
Decision: SOURCE ACCEPTED; focused development green accepted as supporting evidence.
Integrated acceptance and falsification remain pending.

## Source identity and inspection

HEAD: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Only production change: `modern/cmd/bitbookd/main.go`, six inserted lines, 225 total.
Incoming SHA-256:
`9c7aba19576d162b322dce3dddb61990f8d2a5a65f12b681100bf6e23f8a12ab`.
Accepted SHA-256:
`6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b`.
The 733-line test remains
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
The remaining eight original frozen input hashes match.

The diff adds the payment import and constructs the existing service immediately
after successful network.Open and defer node.Close(). Construction errors return
unchanged. The payment defer runs before the node defer on normal return and later
startup-error returns. Registration precedes social/direct/API construction and client
serving. The service uses the existing node identity and datastore without options.
There are no new goroutines, retries, HTTP routes, dependency changes, or edits to
payment, transport, storage, or test semantics. No blocking source findings remain.

## Focused development result

Grok's retained metadata records the authorized pinned cached Go 1.27.0 command,
GOTOOLCHAIN=local, offline settings, owned disk-backed temp/cache paths, both timeout
layers, shell exit 0, and 1.313 seconds wall time. The raw log shows both named
TestPaymentDaemon behavior tests passing; package duration is 0.150s. The reviewer
read the complete output and metadata and recomputed their hashes:

- `modern/dist/pay002-grok-focused-01/logs/focused-daemon-green.log`:
  `7382d7570481fd0ba3be859c859944c9b59f3654e000a3fb8206c37bd5943f38`.
- `modern/dist/pay002-grok-focused-01/logs/focused-daemon-green.meta`:
  `2d82a12239645b513c72add8afe4bdf829ef34b638e49cf4aed205bf939d2852`.

This supports actual delivery, duplicate handling, persistence across restart before
resend, and remote wrong-payer rejection with retained valid control. No cleanup,
panic, timeout, or unrelated diagnostic appears. Metadata and process cleanup are
actor-reported/test-supported; the reviewer did not rerun tests or perform a
host-wide process audit. Diff whitespace checks pass.

## Next authority

[Hermes green 01](../handoff/HERMES_BBGO_PAY_002_GREEN_01.md) owns integrated focused
green, temporary registration-removal falsification, exact restoration and green,
the ticket's five-package race suite, and consolidated evidence. Source and test
identities are frozen except the explicit temporary falsification. No final ticket
acceptance, Git mutation, public-network activity, wallet work, or cross-repository
change is authorized by this review.
