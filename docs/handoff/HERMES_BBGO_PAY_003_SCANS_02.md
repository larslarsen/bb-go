# PAY-003 remaining scans and final evidence

CLOSED. The corrected Gosec result is accepted in acceptance review 03.
Follow [publication 01](HERMES_BBGO_PAY_003_PUBLICATION_01.md) for remaining work;
do not repeat this scan task. The original authorization below is historical.

Actor: Hermes on a free Nous Portal model, owner-relayed.
Reviewer authorization: [acceptance review 02](../testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-02.md).
This handoff supersedes the inactive acceptance 01 execution plan. Grok's report
gate is closed. Complete the remaining work in this single pass and save results
in docs/testing/BBGO-PAY-003-ACCEPTANCE-01.md.

## Frozen state and writable scope

Require HEAD `2b695a8719a7381f3919bb83c63e54251f43536d` and all eight source/module
pins from source review 02. Preserve the already verified binary hash
`345feab607f2422c491b15e67fbe5382e0804b6888e7a3de9c420fed8dd007c0`.
Capture current Git/index state and preserve unrelated changes. Do not integrate a
different source drop, modify tests/dependencies, or repeat tests/builds.

Writable content: the acceptance report; Grok's report only for the explicit
publication normalization below; only the active PAY-003 block in CURRENT_TASK;
and ignored runner artifacts under modern/dist/pay003-tmp. Staging is separately
authorized for the exact fourteen paths listed below. No commit or push.

## 1. Pin tools and diagnose package loading

Inspect filesystem and directory ownership/symlink state before placing artifacts.
modern/dist/pay003-tmp is disk-backed (ext2/ext3); /tmp is tmpfs. Use disk-backed
TMPDIR, GOTMPDIR and GOCACHE. Keep each original log; use new scan-02 log names.

Use the cached Go 1.27.0 toolchain: its executable SHA-256 is
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
Locate it through the installed toolchain, verify version/hash, and put its bin
directory first on PATH so Gosec's child go command uses it. Set GOTOOLCHAIN=go1.27.0,
GOWORK=off and GO111MODULE=on; clear an inherited conflicting GOROOT and GOFLAGS
for this scan process. Run from the modern module directory. Record actual settings.

Reuse the inspected Gosec executable under the user's Go bin directory after
verifying SHA-256 `eb00a1fb095b161a48c5bcadbe1e246bbafe270da497a122d2e63ade346954c2`
and `go version -m` main module github.com/securego/gosec/v2 v2.29.0. Its display
version may say dev; module provenance and hash are the pin. Do not enable AI
features, suppression/exclusion flags, or failure-ignoring options.

Authorized package-loading diagnostic from modern:

```sh
go list -json ./localclient ./cmd/bitbookd
```

Require the two expected modern-module import paths and production GoFiles
server.go and main.go, without package errors. This is package inspection, not
test execution. Inspect the scanner's retained errors/cached source as needed.
Correct only invocation/environment issues. No repository/dependency or scanner
source changes, version changes, or broader package scope.

## 2. Run the corrected Gosec scan

From modern, invoke the pinned executable with this command's exact arguments:

```sh
gosec ./localclient/... ./cmd/bitbookd/...
```

Retain complete stdout/stderr, literal command/environment, executable/module
identity, source pins, actual exit code and elapsed time. Use a 15-minute outer
timeout that covers children. Require exit 0, zero issues, no loading errors and
positive coverage of BOTH localclient/server.go and cmd/bitbookd/main.go. For this
unchanged drop these are the two non-test Go source files; a zero-file/zero-line
result fails regardless of exit code. Include processed-file evidence in the report.

If loading or execution fails, a bounded retry is allowed only after identifying
and correcting a specific environment/invocation cause; keep both attempts. Do not
repeat unchanged failing invocations, change pins, waive findings, or broaden scans.
Stop and document unresolved tool failure or any finding for Codex review.

## 3. Finish the documents and stage the exact set

Update the acceptance report with the new scan evidence and review 02's disposition
of Grok's report. Retain the original invalid scan and authorization timing as
historical limitations. Link each existing stage log. State explicitly which old
command/environment/exit metadata cannot be recovered; do not manufacture it.
Clarify that the former Gitleaks scan used the correct handoff path, but current
document bytes changed afterward. Do not treat it as the final publication scan.

Publication normalization of Grok's report is authorized: replace machine-specific
absolute module paths with a declared `${BBGO_PAY003_MODERN}` placeholder denoting
the absolute expansion of the repository's modern directory. Preserve argv and
environment meaning and state that paths were normalized for publication. Correct
the runtime import description to one added import line plus the three-line guard;
clarify that the off-Linux test is included in the eight localclient test functions.
Do not change Grok's outcomes or claim it wrote Hermes's normalization note.

Prepare CURRENT_TASK's active block as SCANS CAPTURED — REVIEW REQUIRED, with
report/log pointers and no publication authority. Keep the DEV-001 cancellation
and all historical content intact. Exact authorized staged set:

1. modern/localclient/server.go
2. modern/localclient/server_test.go
3. modern/cmd/bitbookd/localclient_test.go
4. modern/cmd/bitbookd/main.go
5. tickets/BBGO-PAY-003.md
6. docs/handoff/CURRENT_TASK.md
7. docs/handoff/GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md
8. docs/handoff/HERMES_BBGO_PAY_003_ACCEPTANCE_01.md
9. docs/handoff/HERMES_BBGO_PAY_003_SCANS_02.md
10. docs/testing/BBGO-PAY-003-SOURCE-REVIEW-02.md
11. docs/testing/BBGO-PAY-003-GROK-CORRECTION-01.md
12. docs/testing/BBGO-PAY-003-ACCEPTANCE-01.md
13. docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-01.md
14. docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-02.md

Use explicit paths, never git add -A. Existing ten staged paths are a subset of
this set. If an unrelated path is staged, stop without altering that staging.
Do not stage AGENTS.md, DEVELOPMENT_ROLES.md, cancelled DEV-001 changes, binaries,
runner logs/state or local paths/credentials. Those governance changes remain
outside this ticket's publication set.

## 4. Scan final staged content and stop

Verify Gitleaks v8.30.1 using its version/module provenance. Reuse the previously
used pinned executable if available. If unavailable, installing precisely
github.com/zricethezav/gitleaks/v8@v8.30.1 into the disk-backed runner tools directory
is authorized; retain version/install results and make no repository dependency
change. Network use is limited to obtaining that pinned tool and its dependencies.

From repository root, with the exact final paths staged:

```sh
gitleaks git --pre-commit --staged --redact=100 --no-banner .
```

Use the same complete result capture and a 15-minute timeout. Require exit 0 and
no findings; do not add suppressions. Record the staged inventory/content hashes
in a runner manifest, including the report and CURRENT_TASK bytes.

To keep the final evidence itself scanned: record the first scan's actual result
in the acceptance report, restage the report and any final status text, then run
the same scan on those final bytes. Keep that last scan result in its predeclared
runner log; do not change staged documents afterward. A failure remains a failure
and must be reported. This necessary final scan is not a test-suite rerun.

Recheck all frozen source and binary pins and working/index correspondence for
the fourteen publication paths. Return the saved evidence path for Codex review.
No commit, push, process restart, desktop integration, new source task or further
acceptance suite is authorized here.
