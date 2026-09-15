# PAY-003 publication and recorded closeout

ACTIVE. Actor: Hermes on a free Nous Portal model, owner-relayed.
Reviewer: Codex. Follow [acceptance review 03](../testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-03.md).
Source, runtime and corrected Gosec are accepted. Complete the remaining document
preparation, exact staged scan, publication and CI reporting in this single task.
Do not rerun tests, fuzz, falsification, vet, Gosec, Govulncheck or the local build.

## Baseline and invariants

Require master at `2b695a8719a7381f3919bb83c63e54251f43536d`, the eight pinned
source/module hashes in source review 02, and binary SHA-256
`345feab607f2422c491b15e67fbe5382e0804b6888e7a3de9c420fed8dd007c0`.
The reviewer observed an empty index. Capture status/index before work and preserve
all unrelated edits. Verify origin is the intended larslarsen/bb-go repository and
remote master still equals the baseline before publication. If either baseline
changed or unrelated content is staged, stop and document it; no reset, stash,
rebase, merge, force push or cleanup of another actor's files.

Writable documents: acceptance report, Grok report only for normalization described
below, active CURRENT_TASK block, ticket status, and the new publication report
docs/testing/BBGO-PAY-003-PUBLICATION-01.md. Runner logs/manifests may be written
under disk-backed modern/dist/pay003-tmp after filesystem/ownership checks. All
production, tests, dependencies and the actual local binary are frozen.

## Prepare the final documents

Normalize the machine-specific absolute module path in Grok's report to a declared
`${BBGO_PAY003_MODERN}` placeholder meaning the absolute expansion of this repository's
modern directory. Preserve the meaning of all historical argv/environment values
and identify the edit as Hermes publication normalization. Correct only the already
authorized inventory wording: one runtime import line plus a three-line guard;
the off-Linux test is included in the eight localclient test functions.

In the acceptance report, replace the machine-specific scanner path with a declared
tool-path placeholder and retain its exact hash/module pin. Explain the reported
Gosec argv interpretation from review 03; do not change the historical command or
claim test files were scanned. Reference the three actual historical Gitleaks logs,
state that the index was empty at reviewer inspection and that no final manifest
was retained. Preserve all earlier limitations and results. Record why staging was
cleared if recoverable from your own execution records; otherwise state unknown.

Prepare ticket/CURRENT_TASK as ACCEPTED FOR PUBLICATION — PUBLICATION IN PROGRESS,
pointing to review 03 and this handoff. Retain the DEV-001 cancellation and all
historical blocks. Predeclare final publication scan log/manifest locations in the
acceptance report; do not invent an outcome before running it.

## Exact feature commit: sixteen paths

Stage only these explicit paths:

1. modern/localclient/server.go
2. modern/localclient/server_test.go
3. modern/cmd/bitbookd/localclient_test.go
4. modern/cmd/bitbookd/main.go
5. tickets/BBGO-PAY-003.md
6. docs/handoff/CURRENT_TASK.md
7. docs/handoff/GROK_BUILD_BBGO_PAY_003_CORRECTION_01.md
8. docs/handoff/HERMES_BBGO_PAY_003_ACCEPTANCE_01.md
9. docs/handoff/HERMES_BBGO_PAY_003_SCANS_02.md
10. docs/handoff/HERMES_BBGO_PAY_003_PUBLICATION_01.md
11. docs/testing/BBGO-PAY-003-SOURCE-REVIEW-02.md
12. docs/testing/BBGO-PAY-003-GROK-CORRECTION-01.md
13. docs/testing/BBGO-PAY-003-ACCEPTANCE-01.md
14. docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-01.md
15. docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-02.md
16. docs/testing/BBGO-PAY-003-ACCEPTANCE-REVIEW-03.md

Do not stage AGENTS.md, DEVELOPMENT_ROLES.md, cancelled helper/governance drafts,
binaries, runner logs/state, or any desktop file. Do not use git add -A.
Verify the staged path set exactly, the four staged source hashes against their
accepted pins, and working/index correspondence. Check staged documents for local
home paths and credentials, then run git diff --cached --check. Fix only the
authorized document normalization if a content check fails.

Use verified Gitleaks v8.30.1 and run from repository root:

```sh
gitleaks git --pre-commit --staged --redact=100 --no-banner .
```

Capture literal command/environment, tool provenance, actual exit code, stdout/stderr
and elapsed time in a new publication log. Require exit 0 and zero findings. Capture
SHA-256 for each staged file plus the staged tree ID in a runner manifest. Do not
alter or unstage the scanned bytes before committing. Any necessary byte change
requires another scan of those changed final bytes; do not repeat unchanged scans.

After all checks pass, the exact feature commit and normal push are authorized:

```sh
git commit -m "feat(payment): add authenticated local payment record access"
git push origin HEAD:master
```

Verify the commit contains exactly the sixteen reviewed paths and the four source
pins. Verify remote master equals that commit. Preserve all unrelated dirty work.
If commit succeeds but push fails, record the local commit and failure; do not
rewrite it or broaden permissions/paths.

## CI and documentation closeout

Observe the existing Go 1.27 push workflow for that exact feature commit using
read-only gh commands. Record its run URL, head SHA and conclusion. Do not start
additional workflows. If it fails, report the failed job/logs without source repair
or retry loops. If still pending after a bounded observation period, record pending
and return; do not claim success.

Write docs/testing/BBGO-PAY-003-PUBLICATION-01.md with actual actor, baseline,
feature commit, remote verification, exact scan command/exit, final staged manifest
and scan-log references, normalization performed, source/binary invariance, CI
head/run/result and remaining unrelated Git status. Use repository-relative paths.
The report is the completion record; chat supplies only its pointer.

If feature push and CI succeed, one documentation closeout commit is also authorized
in this same task, limited to these three paths:

- docs/testing/BBGO-PAY-003-PUBLICATION-01.md
- docs/handoff/CURRENT_TASK.md (active block only)
- tickets/BBGO-PAY-003.md (status/pointers only)

Set status PUBLISHED — REVIEWER FINAL VERIFICATION and point to the feature commit,
successful CI and saved publication report. Do not authorize desktop/source work.
Stage only those three paths, repeat the same final staged-content Gitleaks and
whitespace checks, retain a separate manifest/result, then commit with message
`docs(payment): record PAY-003 publication and CI` and push normally. Verify the
remote closeout commit and confirm it changes no source. Documentation-only changes
need no local build or extra workflow dispatch. If CI/push fails or remains pending,
leave the honest publication report in the worktree for reviewer recovery instead
of claiming a completed closeout.

Return the publication-report path. No additional implementation, daemon restart,
deployment, wallet transaction or cross-repository work is authorized.
