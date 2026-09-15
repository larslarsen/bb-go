# PAY-003 corrected source review

Current disposition: both repository completion reports are reviewed in
[acceptance review 02](BBGO-PAY-003-ACCEPTANCE-REVIEW-02.md). The report gate below
is historical and closed; corrected source and retained runtime results are accepted
for the remaining bounded scans. Follow CURRENT_TASK and scan completion 02.

Reviewer: Codex. Decision: STATIC SOURCE ASSESSMENT ACCEPTED; DELIVERY INCOMPLETE.
Grok's repository completion document is required before advancement. Prior Hermes
integration/acceptance authorization is withdrawn pending review of that document.
The source findings below stand; runtime, security, publication and binary refresh
are not claimed complete. No actor was launched by this review.

Baseline: `2b695a8719a7381f3919bb83c63e54251f43536d`.
Contract: [BBGO-PAY-003](../../tickets/BBGO-PAY-003.md).
Prior findings: [correction 01](../handoff/GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md).
Next: [Grok report-only addendum](../handoff/GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md).
Prepared but inactive: [Hermes acceptance 01](../handoff/HERMES_BBGO_PAY_003_ACCEPTANCE_01.md).

## Findings resolved

- `hasRequestBody` rejects nonzero ContentLength (including -1) and parsed
  TransferEncoding. The real-listener streaming regression requires 400 and zero
  record-reader calls while the pipe is still open. Its client disables keep-alive;
  installed Go 1.27 HTTP source confirms that this sends Connection: close and
  avoids the server's pre-response keep-alive body drain. The bodyless nonempty
  success test remains the positive control. This is static inspection, not a run.
- Publication retains file identity; cleanup uses rooted Lstat, requires a regular
  non-symlink entry, and compares os.SameFile before removal. The atomic-replacement
  regression verifies replacement bytes survive; the existing unchanged-descriptor
  case still requires removal. This meets the ticket's trusted-account scope and
  makes no new hostile same-user race guarantee.
- Fuzzing creates one fixture per fuzz process rather than per input, supplies a
  valid host/remote address, and maps a stable VALID marker to the run's credentials.
  Six seeds include an authenticated nonempty reader and denied inputs. Random
  credentials are not corpus inputs. Reader-call deltas now exercise authentication.
- Linux guards cover the local listener tests and fuzz target. The new off-Linux
  test checks ErrUnavailable. Removing only the daemon test's runtime import and
  three-line guard reconstructs the prior 191-line test hash exactly.
- Error assertions capture the response once, reject unknown JSON fields, require
  the expected stable error code, and require EOF after the object. The consumed-
  stream storage assertion is gone.

No blocking source finding remains in this correction.

## Reviewed identities

| Path | SHA-256 | Lines |
| --- | --- | ---: |
| modern/localclient/server.go | 5b00cc6e10694a5fbaccda27637751b866cbc6bb1f0bfd98f05dc94479cc7a93 | 440 |
| modern/localclient/server_test.go | 655c7e18769fdd0f066e29636b11184c64b05ab83d29397b32399b018a6bdf6c | 749 |
| modern/cmd/bitbookd/localclient_test.go | 221407662c049dd27e0a31c58952e194d52c8b13ea1e4015de2b129c7f619799 | 195 |
| modern/cmd/bitbookd/main.go | b7b72438c3e41131a5fb9578bc0e22ca12bef11baeac2f3d6a0cdd9429631a20 | 253 |

main.go matches the frozen first drop. payment/service.go and api/handler.go match
their ticket pins. go.mod and go.sum have no diff against HEAD; their hashes are
`1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783` and
`4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a`.
Other dirty work predates this review and is excluded from PAY-003 publication.

Static inventory: eight localclient TestLocalClient functions, one daemon test,
and one fuzz target with six seeds. On Linux the off-platform test should skip;
these are source counts, not observed runtime outcomes. gofmt -l on all four paths
was empty (exit 0); git diff --check passed (exit 0).

The owner reports Grok finished, but the required repository completion document
is absent and no retained PAY-003 log/report was found in modern/dist/pay003-tmp. Neither
the requested pre-fix red nor Grok's focused green/fuzz-seed results are verified.
Do not invent those results or claim a historical red from a later execution.
Grok must document its own work in docs/testing/BBGO-PAY-003-GROK-CORRECTION-01.md;
Codex reads and reviews it before authorizing Hermes. Asking the owner to paste
the report into chat and advancing without the document were reviewer errors.
Unavailable historical results must be explicit in Grok's report. No reviewer test, scanner,
source mutation, build, staging, commit, push or daemon restart occurred here.

Reviewer-authored paths for this review are exactly this file,
docs/handoff/HERMES_BBGO_PAY_003_ACCEPTANCE_01.md,
docs/handoff/CURRENT_TASK.md and tickets/BBGO-PAY-003.md.

The owner-directed control-plane correction also changes AGENTS.md,
docs/engineering/DEVELOPMENT_ROLES.md and
docs/handoff/GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md. It documents report ownership,
names Grok's report path and withdraws premature advancement. No developer evidence
was authored by the reviewer.
