# BBGO-PAY-001 Gap Expected-Red Attempt 02

Execution actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Governance baseline: `6282ea57fbba7174a73557a8abcb8004cbb38761`

Result: **REJECTED — LOOPBACK BIND BLOCKED BY SANDBOX**

Luna's preflight passed exactly: `HEAD` and upstream equalled the governance baseline;
the eleven expected dirty paths and every reviewed line count and SHA-256 matched;
`git diff --check` passed; and read-only `gofmt -d` was empty over all eleven paths.

Luna ran the sole authorized foreground command once from `modern/`. It exited 1 and
proved two intended missing protections:

```text
--- FAIL: TestCanonicalJSONNestingBoundaries/depth33
    canonical_test.go:578: depth 33 code = "", want "SCHEMA" (<nil>)
--- FAIL: TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal
    signature_test.go:91: invalid UTF-8 producer code = "", want "SCHEMA" (<nil>)
```

The real two-node path could not execute because the restricted sandbox denied an
ephemeral loopback listener:

```text
failed to listen on any addresses: [listen tcp4 127.0.0.1:0: socket: operation not permitted]
--- FAIL: TestStatusNonceMustDifferFromLinkedRequest
    service_test.go:286: creating libp2p host: failed to listen on any addresses: [listen tcp4 127.0.0.1:0: socket: operation not permitted]
```

This environmental failure makes the combined output unacceptable expected red. It does
not require root, `sudo`, a public listener, or a fixed port. A single approved execution
outside the restricted sandbox is required only so the in-process hosts can bind
ephemeral `127.0.0.1` sockets.

No file was edited, staged, committed, or pushed. All seven production paths remain
dirty and unchanged. Luna may make one final foreground attempt only under
`CODEX_LUNA_BBGO_PAY_001_GAP_EXPECTED_RED_03.md` and must request the sandbox override
for that exact command.
