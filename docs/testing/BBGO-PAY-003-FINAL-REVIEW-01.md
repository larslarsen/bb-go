# PAY-003 final acceptance

Reviewer: Codex. Decision: ACCEPTED AND PUBLISHED. No further implementation or
executor work is authorized under BBGO-PAY-003.

## Publication independently verified

- Feature: `82ed5f9c62ab22687a4972ba0ad59731bf43013e`, directly descended from the
  approved baseline `2b695a8719a7381f3919bb83c63e54251f43536d`. Its changed-path
  set is exactly the sixteen paths in publication handoff 01.
- Hermes documentation closeout: `290825cb40f08f683d72993e8412a703d6a11836`, directly
  descended from the feature. Its changes are exactly the authorized three documents.
- GitHub's remote master API returned the documentation closeout SHA. The
  [Go 1.27 run](https://github.com/larslarsen/bb-go/actions/runs/35017544700) is
  completed/success with head SHA equal to the exact feature commit; its test job
  and compile/runtime/P2P steps succeeded.
- The four committed source blobs match their accepted SHA-256 pins and the current
  worktree. The other four frozen payment/API/module inputs also match. No source
  change was hidden in the documentation closeout.
- Published feature documents contain no machine-specific local home path. The
  report normalization preserves its declared historical-result limitations.
- Retained Gitleaks publication and closeout logs report no findings over approximately
  127,928 and 4,756 bytes respectively. Hermes reports exit 0. A separate staged
  hash manifest was not retained; this limit remains explicit. The reviewer verified
  committed path sets and source blobs directly, rather than claiming a missing
  manifest exists.

References: [publication record](BBGO-PAY-003-PUBLICATION-01.md),
[acceptance evidence](BBGO-PAY-003-ACCEPTANCE-01.md),
[source review](BBGO-PAY-003-SOURCE-REVIEW-02.md), and
[publication decision](BBGO-PAY-003-ACCEPTANCE-REVIEW-03.md).

## Additional test-file scan adjudication

Hermes ran an additional Gosec scan with -tests during publication despite the
handoff explicitly prohibiting another scanner suite. It then published while its
own report said the new findings awaited review. This was a workflow deviation;
the final review does not retroactively authorize it. The original production scan
remains clean: two files, 693 lines, zero issues.

The additional log reports seven findings. Codex inspected each source location:

| Finding | Source | Reviewer disposition |
| --- | --- | --- |
| G204 | cmd/bitbookd/payment_test.go:284 | Re-executes os.Executable with a fixed test filter and fixed count, using exec.Command without a shell. No network-controlled command is supplied. Existing test harness, no injection defect. |
| G304 | localclient/server_test.go:374 | Reads the replacement file created in this test's own temporary directory to prove cleanup preserved it. No external path input. |
| G304 | localclient/server_test.go:442 | Reads the test-created sentinel in another owned temporary directory to prove a symlink target was untouched. Intentional fixture read. |
| G304 | localclient/server_test.go:587 | Descriptor helper reads beneath the test's own data directory after rejecting a descriptor symlink. Callers supply fixture directories. |
| G304 | cmd/bitbookd/localclient_test.go:110 | Descriptor helper receives the child daemon fixture's owned data-directory path and checks for a symlink before reading. No renderer/network-selected path. |
| G301/G302 | localclient/server_test.go:386,389 | Deliberately creates/chmods a 0755 directory inside t.TempDir, then requires production Start to reject it without changing permissions. Fixing the fixture permissions would remove the intended regression. |

These are nonblocking findings in the inspected test fixtures; no production or
test-source repair is warranted. No scanner suppression or finding-threshold change
is added. Re-review if these helpers acquire untrusted inputs or move into production.

The report's exit-0 claim for the additional scan is not verified. Pinned Gosec
computeExitCode returns failure for unsuppressed findings unless no-fail is enabled,
and the reported command does not include no-fail. Its raw log contains seven
findings and no captured shell status. This review treats it as a findings-bearing
diagnostic, never as a clean exit. The source-level adjudication above closes those
findings independently of the unreliable reported exit. Historical evidence is
preserved, and this final reviewer disposition supersedes its pending verdict.

## Accepted behavior and remaining scope

The Linux daemon exposes authenticated loopback reads of stored signed payment
records, with private per-run discovery credentials, parsed-body rejection,
instance-owned descriptor cleanup and daemon startup/shutdown integration.
The previously reviewed race, native fuzz, authentication falsification/restoration,
vet and Govulncheck policy evidence remain accepted with their documented limits.

The local binary remains 43,899,109 bytes, SHA-256
`345feab607f2422c491b15e67fbe5382e0804b6888e7a3de9c420fed8dd007c0`.
It was built with Go 1.27.0 from the accepted working source before publication;
the later commits changed no executable source. No repeat build is needed solely
to replace its pre-publication VCS metadata. No real daemon restart was performed
by the reviewer; Hermes likewise reports no restart.

Desktop payment-inbox integration is a subsequent bounded task. This ticket does
not deliver wallet approval, submission or payment confirmation. DEV-001 remains
cancelled. Unrelated dirty files and untracked drafts are preserved.

## Reviewer closeout scope

Under AGENTS.md's reviewer-authored governance/review exception, Codex may publish
exactly this file, docs/handoff/CURRENT_TASK.md and tickets/BBGO-PAY-003.md.
No developer source, test, implementation evidence, other governance draft or binary
is included. Validation is read-only remote/CI verification, committed-blob/path
inspection and documentation diff checks; no acceptance commands were executed.
