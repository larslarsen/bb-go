# PAY-003 integrated acceptance

SUPERSEDED: the reports are reviewed. The sole active execution authority is
[Hermes scan completion 02](HERMES_BBGO_PAY_003_SCANS_02.md). Everything below is
retained history; do not restart this completed acceptance sequence.

Latest disposition: [acceptance review 01](../testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-01.md).
Execution results have been submitted. Hermes may now correct only its own
acceptance document using existing records, as that review specifies. The zero-file
Gosec result is invalid; source/testing/build commands below must not be repeated
without a new reviewer authorization. Grok's separate completion report is missing.

STATUS: PREPARED, INACTIVE. Prior execution/staging/build authorization is withdrawn.
Grok must first write docs/testing/BBGO-PAY-003-GROK-CORRECTION-01.md and Codex must
review it and explicitly reactivate this handoff in CURRENT_TASK.md. None of the
commands or mutations below is currently authorized. Retained steps are the
prepared acceptance plan after that gate; do not ask the owner for evidence in chat.
If execution began under the earlier authorization, mandatory exact restoration
still applies. Retain and document results already produced in the designated
acceptance report and stop before starting another stage. Do not discard or repeat
completed work merely because the control-plane documentation was corrected.

Actor: Hermes on a free Nous Portal model, owner-relayed.
Reviewer: Codex. Source is accepted in
[source review 02](../testing/BBGO-PAY-003-SOURCE-REVIEW-02.md).
Follow [BBGO-PAY-003](../../tickets/BBGO-PAY-003.md), AGENTS.md and TESTING.md.
No actor has been launched. Grok's correction authority is closed.

## Scope and preflight

Verify HEAD `2b695a8719a7381f3919bb83c63e54251f43536d`, the four reviewed source
hashes/line counts and frozen payment/API/module hashes in source review 02.
Capture initial Git status and staged paths; preserve all unrelated dirty files.
Stop on a source-pin mismatch rather than repairing or accepting a different drop.
Source is already present in the shared tree; integration means verifying it.

Writable paths are docs/testing/BBGO-PAY-003-ACCEPTANCE-01.md, the active block of
docs/handoff/CURRENT_TASK.md, ignored runner state under modern/dist/pay003-tmp,
modern/localclient/server.go for the exact temporary falsification/restoration
below, and modern/bitbookd for the routine final build. Tests and remaining source
are frozen. No source fixes, test authoring, dependency edits or cancelled DEV-001
helper use. Preserve the owner's running daemon and all its data.

Use Go 1.27.0 with GOTOOLCHAIN=go1.27.0 and GOWORK=off. From modern, inspect the
filesystem and directory ownership/symlink state before creating runner subpaths.
The reviewer found modern/dist/pay003-tmp disk-backed (ext2/ext3); /tmp is tmpfs.
Set TMPDIR and GOTMPDIR to the absolute expansion of "$PWD/dist/pay003-tmp";
use a disk-backed Go cache under that directory or an existing verified disk cache.
Record repository-relative paths in published evidence. Do not recursively delete
runner directories. Verify tool versions before scans: gosec v2.29.0,
govulncheck v1.7.0, Gitleaks v8.30.1. Existing pinned tools may be reused; fetch only
these versions if needed. Network use is limited to tools/advisory data.

## Execute the existing acceptance gates

Retain each exact command/environment, stdout/stderr, source hashes, wall duration
and actual exit code in a named runner log. Preserve child-command exit status.
Use bounded outer timeouts for the commands (15 minutes for race/scans/build,
5 minutes for focused/fuzz/vet); a timeout is a failure, never a pass.
Stop at the first unexpected outcome and report it without source repair or wider
tests. Mandatory restoration below still runs after any failure.

From modern, run once on the pinned source:

```sh
go test -race ./localclient ./cmd/bitbookd ./payment ./api -count=1
go test ./localclient -run '^$' -fuzz '^FuzzLocalClientAuth$' -fuzztime=10s
```

Require exit 0, no race/panic/cleanup error, and a nonzero native fuzz execution
count. Ordinary race-suite fuzz seeds do not substitute for the native campaign.
Record the unsupported-platform skip on Linux separately from passing tests.

Falsify authentication once. Back up server.go byte-for-byte and install guaranteed
restoration before changing its single `if !s.authorized(r) {` to
`if false && !s.authorized(r) {`. Falsified SHA-256 must be
`6872a12fe05958305e5c86dc13afff45ccb88f47570da2d95760603aca480fe0`.
Do not reformat, modify tests or change any other bytes. Run:

```sh
go test ./localclient -run '^TestLocalClientHTTPRejectionsAndSuccessfulRead$/^(wrongToken|wrongInstance)$' -count=1
```

Require exit 1 and both wrongToken/wrongInstance cases receiving 200 instead of
401; compilation failures, timeouts or environment errors do not prove the test.
Restore the exact accepted server.go even on failure; verify its accepted hash.
After the expected falsification, run that identical focused command on restored
source and require exit 0. This is new falsification evidence, not Grok's missing
historical pre-fix red. Then continue:

```sh
go vet ./localclient ./cmd/bitbookd
gosec ./localclient/... ./cmd/bitbookd/...
```

From repository root run:

```sh
python3 scripts/govulncheck_policy.py source
```

Require clean vet/gosec and policy-accepted Govulncheck output; preserve the existing
reviewed dependency exception. New findings block acceptance. Do not add exemptions.

## Evidence, staging, and routine build

Write concise evidence with actual model, baseline, source identities, commands,
results, falsified/restored hashes, scanner versions/findings and log references.
Reference Grok's repository completion document and Codex's disposition of it.
Preserve any historical evidence gaps explicitly; do not reconstruct results.

After explicit reactivation, staging is authorized solely for the staged-content
scan, on these eleven paths:

- modern/localclient/server.go
- modern/localclient/server_test.go
- modern/cmd/bitbookd/localclient_test.go
- modern/cmd/bitbookd/main.go
- tickets/BBGO-PAY-003.md
- docs/handoff/GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md
- docs/testing/BBGO-PAY-003-SOURCE-REVIEW-02.md
- docs/testing/BBGO-PAY-003-GROK-CORRECTION-01.md
- docs/handoff/HERMES_BBGO_PAY_003_ACCEPTANCE_01.md
- docs/testing/BBGO-PAY-003-ACCEPTANCE-01.md
- docs/handoff/CURRENT_TASK.md

If unrelated paths are already staged, report this without changing that staging.
With only the explicit restored ticket path set staged, run from repository root:

```sh
gitleaks git --pre-commit --staged --redact=100 --no-banner .
```

Require zero new findings. Stage final evidence/governance updates and ensure the
final staged bytes also pass this scan. Never stage binaries or ignored runner state.
This enumerated set extends the original ticket's publication set only with the
PAY-003 review/handoff records and Grok's completion document above; cancelled
DEV-001 drafts remain excluded.

Once all gates pass, perform the routine affected local binary refresh from modern:

```sh
go build -mod=readonly -o bitbookd ./cmd/bitbookd
go version -m bitbookd
```

Verify the actual output path, hash, toolchain and build identity. The working tree
contains uncommitted accepted source, so report vcs.modified honestly; a HEAD label
alone is not proof of current contents. Do not restart or signal the real daemon.
If build fails, report failure without claiming the existing binary was refreshed.

Set only the active CURRENT_TASK block to ACCEPTANCE CAPTURED — REVIEW REQUIRED
and link the evidence, or ATTEMPT STOPPED with the failing stage/restoration state.
Return results for Codex acceptance. No commit, push, desktop integration or further
implementation is authorized by this handoff.
