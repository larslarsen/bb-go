# BBGO-PAY-002 phase A — Hermes publication 01

Actor: Hermes, free Nous Portal model, manually relayed by owner.
Reviewer: Codex. Status: ACTIVE — conditional publication of accepted phase A.

Read AGENTS.md, TESTING.md, CURRENT_TASK.md, tickets/BBGO-PAY-002.md, and
[integrated green review 01](../testing/BBGO-PAY-002-GREEN-REVIEW-01.md).
Source, test-first red, integrated green, falsification/restoration, and the five-package
race suite are accepted. Do not rerun local tests or alter implementation.

## Preconditions

Require branch master and HEAD `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Require origin to identify `larslarsen/bb-go` on GitHub; never push upstream.
Use `git ls-remote origin refs/heads/master` to confirm the remote baseline before
staging. Require no pre-existing staged changes. Stop on any divergence; no merge,
rebase, reset, force-push, branch change, or automatic repair is authorized.

Verify the accepted 225-line main.go SHA-256
`6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b`,
733-line test SHA-256
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`,
and all eight other frozen inputs from the original inventory. Preserve all raw logs
and reports; read the corrected counts in green review 01 rather than rewriting the
original evidence. Require clean `git diff --check`.

## Exact publication paths

Stage exactly these 23 files using explicit paths, never git add -A or a directory:

```text
AGENTS.md
TESTING.md
docs/engineering/DEVELOPMENT_ROLES.md
docs/handoff/CURRENT_TASK.md
docs/handoff/GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_01.md
docs/handoff/GROK_BUILD_BBGO_PAY_002_DAEMON_TESTS_CORRECTION_01.md
docs/handoff/GROK_BUILD_BBGO_PAY_002_DAEMON_WIRING_01.md
docs/handoff/HERMES_BBGO_PAY_002_EXPECTED_RED_01.md
docs/handoff/HERMES_BBGO_PAY_002_EXPECTED_RED_RECOVERY_01.md
docs/handoff/HERMES_BBGO_PAY_002_GREEN_01.md
docs/handoff/HERMES_BBGO_PAY_002_PUBLISH_01.md
docs/testing/BBGO-PAY-002-EXPECTED-RED-01.md
docs/testing/BBGO-PAY-002-EXPECTED-RED-RECOVERY-01.md
docs/testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-01.md
docs/testing/BBGO-PAY-002-EXPECTED-RED-REVIEW-02.md
docs/testing/BBGO-PAY-002-GREEN-01.md
docs/testing/BBGO-PAY-002-GREEN-REVIEW-01.md
docs/testing/BBGO-PAY-002-PRODUCTION-SOURCE-REVIEW-01.md
docs/testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-01.md
docs/testing/BBGO-PAY-002-TEST-SOURCE-REVIEW-02.md
modern/cmd/bitbookd/main.go
modern/cmd/bitbookd/payment_test.go
tickets/BBGO-PAY-002.md
```

Require the staged path set to equal that list. Review the staged diff/stat and
`git diff --cached --check`. Verify staged main.go/test bytes equal accepted worktree
bytes. The untracked `modern/bitbookd` is explicitly excluded. No raw logs, backups,
cache, temporary state, private material, unrelated path, or cross-repository content
may be committed. The three role-policy files are the owner's previously requested
Hermes replacement and Sr Dev focused-test changes, not new policy work.

## Pinned staged-content secret gate

Use the existing scanner at repository-relative
`../.security-tools/bbgo-sec-tools-20260829/gitleaks`. Verify its embedded module/version
using the already accepted cached Go executable's `version -m` metadata command;
require `github.com/zricethezav/gitleaks/v8` at `v8.30.1`. No installs/downloads.
The stage scan uses the default rules, no suppressions, and no history baseline;
the unchanged history was covered by the accepted prior security review.

After staging and with no intervening content mutation, run once from the repo root:

```text
timeout --signal=TERM --kill-after=10s 300s env -u GITLEAKS_CONFIG -u GITLEAKS_CONFIG_TOML ../.security-tools/bbgo-sec-tools-20260829/gitleaks git --pre-commit --staged --redact=100 --ignore-gitleaks-allow --no-banner .
```

Require exit 0 and zero leaks. Retain only fully redacted output, exact command,
scanner version, shell exit, duration, and staged tree identity in owned disk-backed
`modern/dist/pay002-publish-01/` runner artifacts. Recheck filesystem placement before
creating that directory. Do not add reports there to Git. Stop on any finding/error
or unexpected skip; do not suppress, repair, or publish. Do not display secret values.

## Commit, push, and observe

If and only if all preceding gates pass and the staged tree is unchanged, run:

```text
git commit -m "feat(payment): start payment service in daemon lifecycle"
git push origin HEAD:refs/heads/master
```

Do not amend an existing commit or bypass hooks. If commit fails, preserve the index
and report it. If push fails or is rejected, preserve the local commit and report;
do not force, merge, or retry with another destination. Record the commit ID and verify
origin master equals it with read-only remote lookup.

Observe only this commit's automatically triggered GitHub Actions runs via read-only
gh run/API calls. Record the Go 1.27 workflow ID/URL, head SHA, and conclusion. If
other workflows trigger, report their results too. Poll existing runs to completion
without rerunning/canceling them or modifying workflows; report a concrete CI failure
for reviewer action. Keep progress visible while waiting. No additional local tests,
scanners, release builds, deployments, Git writes, or further source work.

Return the commit SHA, pushed ref, exact changed paths, staged secret-scan result,
accepted source hashes, final status, and CI URLs/conclusions. Expect the owner binary
to remain untracked; do not claim a completely clean worktree while it is present.
No repository record edits are authorized in this handoff: the durable review already
records runtime acceptance and this exact publication authority. Codex records the
publication result after reviewing your report.
