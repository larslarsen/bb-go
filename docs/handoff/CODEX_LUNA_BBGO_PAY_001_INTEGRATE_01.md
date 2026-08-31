# Codex Luna Handoff — BBGO-PAY-001 Evidence and Integration 01

You are **Jr Dev — Codex Luna** using `gpt-5.6-luna`. Ephemeral chat is not
authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Governance baseline: the commit containing this handoff

Feature commit subject: `feat(payment): add signed payment transport`

Read completely before acting: `AGENTS.md`, `TESTING.md`,
`docs/engineering/DEVELOPMENT_ROLES.md`, `tickets/BBGO-PAY-001.md`,
`docs/handoff/CURRENT_TASK.md`, every `docs/testing/BBGO-PAY-001-*.md` record referenced
there, and this complete handoff. Ephemeral agent summaries are not evidence.

## Purpose and non-duplication boundary

Every expected-red, focused/full, falsification, vet, race, six-target native fuzz,
policy-unit, Actionlint, source Govulncheck, Gosec, baseline-validator, and Gitleaks gate
is complete and reviewer-accepted. Do not execute or duplicate any test, validator,
scanner, linter, formatter, build, or network command. Integrate the already accepted
source with one consolidated evidence document.

## Preflight

From the repository root, require `HEAD` and upstream to equal the governance baseline
and `git diff --check` to pass. Require exactly these eight dirty paths and no others:

```text
modern/go.mod
modern/network/protocols.go
modern/payment/canonical.go
modern/payment/frame.go
modern/payment/replay.go
modern/payment/service.go
modern/payment/signature.go
modern/payment/types.go
```

Require every line count/SHA-256 in `BBGO-PAY-001-GREEN-RECOVERY-01.md`, only the
accepted `golang.org/x/text v0.40.0` direct-dependency move in `modern/go.mod`, and a
clean `modern/go.sum`. Stop on any mismatch.

## Authorized evidence edits

Create only `docs/testing/BBGO-PAY-001-GREEN-01.md` and update only
`docs/handoff/CURRENT_TASK.md`. Do not edit the accepted production/module paths,
tests/fixtures, ticket, another document, policy, workflow, baseline, or dependency.

The consolidated evidence must be self-contained and include:

- daemon/oracle/frozen-test/governance baselines and the final seven production plus
  `go.mod`/clean-`go.sum` hashes and line counts;
- the exact `go mod tidy` classification;
- accepted focused payment regressions, complete payment package, protocol-ID test,
  all three intentional falsification failures with exact restoration and green reruns,
  `go vet ./...`, and `go test -race ./... -count=1` results;
- all six native fuzz targets with nonzero execution/interesting counts: 21,891 and 56
  total; 4,967 and 67 total; 12,261 and 6 total; 6,087 and 50 total; 4,849 and 55 total;
  1,433 and 91 total, in target order from the accepted fuzz reviews;
- policy-suite counts 51, 49, and 27; clean Actionlint; source Govulncheck's accepted
  reviewed `GO-2024-3218` exception on exact DHT v0.42.2 (owner, expiry 2026-11-29),
  zero warnings, and the two non-reachable `GO-2026-5932`/`GO-2026-6303` notes;
- Gosec's 17 files / 4,579 lines / zero Nosec / zero issues result;
- the exact reviewed Gitleaks baseline metadata and the successful 3,406-commit,
  approximately 313.91 MB, 20.4-second, zero-new-leak scan;
- every execution-channel non-result and its reviewer disposition without treating it
  as a test/scanner result;
- final clean diff/state checks and the decision that this routine source ticket built
  no product/release binary and regenerated no SBOM because both remain manual release
  gates; and
- links to the detailed durable records instead of reproducing secret-bearing or
  excessively verbose output. Never include a secret or match value.

Update `CURRENT_TASK.md` to state `GREEN 01 CAPTURED — XHIGH ACCEPTANCE REQUIRED`, link
the consolidated evidence, name this feature commit as the commit under review, and
authorize no further engineering work pending Codex XHigh's decision.

## Exact integration

After a final `git diff --check`, stage only these ten literal paths:

```text
docs/handoff/CURRENT_TASK.md
docs/testing/BBGO-PAY-001-GREEN-01.md
modern/go.mod
modern/network/protocols.go
modern/payment/canonical.go
modern/payment/frame.go
modern/payment/replay.go
modern/payment/service.go
modern/payment/signature.go
modern/payment/types.go
```

Require `git diff --cached --name-only` to contain exactly those ten paths, review the
staged diff/stat, and require cached `git diff --check` to pass. Commit once with exact
subject `feat(payment): add signed payment transport`, then push `master`. Require the
final worktree to be clean and local `HEAD`/upstream to match. Report the full commit
SHA, push result, exact committed paths/stat, evidence path, and final state.

Do not amend, rebase, merge, tag, dispatch GitHub Actions, run a test/scan/build, or claim
engineering acceptance. Do not install, use `/tmp`, delete/clean anything, use root or
`sudo`, contact a network/service/peer except the authorized `git push`, or perform
wallet, rate, transaction, hardware/device, desktop, release, binary, SBOM, or unrelated
work. Codex XHigh owns final acceptance.
