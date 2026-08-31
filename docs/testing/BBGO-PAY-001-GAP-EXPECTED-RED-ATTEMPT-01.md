# BBGO-PAY-001 Gap Expected-Red Attempt 01

Execution actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Reviewer: Lead Engineer/Reviewer — Codex XHigh

Governance baseline: `bb12e283a91ddfa896af5f0153e724613fdf438e`

Result: **REJECTED BEFORE TEST EXECUTION — FROZEN TEST API ARITY DEFECT**

Luna's preflight passed. `HEAD` and upstream equalled the governance baseline; the
worktree contained exactly the ten reviewed dirty paths; all ten SHA-256 values and line
counts matched gap-test source review 01; `git diff --check` passed; and read-only
`gofmt -d` was empty across all ten paths.

Luna then ran the sole authorized foreground command once from `modern/`:

```text
GOTOOLCHAIN=go1.27.0 go test ./payment -run '^(TestSignRequestRejectsInvalidUTF8MemoBeforeMarshal|TestCanonicalJSONNestingBoundaries|TestStatusNonceMustDifferFromLinkedRequest)$' -count=1
```

It exited 1 before tests ran, with exactly:

```text
payment/transport_test.go:242:15: assignment mismatch: 2 variables but DecodePaymentStatus returns 4 values
payment/transport_test.go:257:15: assignment mismatch: 2 variables but DecodePaymentStatus returns 4 values
```

This is not acceptable expected red. The production handoff froze
`DecodePaymentStatus(raw []byte) (PaymentStatusEventV1, []byte, string, error)`, and all
other call sites use its four-result API. The two `TestNetworkStatusIsCancellationOnly`
codec assertions accidentally bind two results. Their intended behavior is unchanged by
discarding the decoded value, canonical bytes, and digest while retaining the error.

No file was edited, staged, committed, or pushed during the attempt. The seven
production paths remain dirty and byte-identical to production source review 01. Sol may
make only the two mechanical result-binding corrections authorized in
`CODEX_SOL_BBGO_PAY_001_TEST_COMPILE_CORRECTION_01.md`; no production correction or
execution is authorized.
