# BBGO-SEC-001 Integration Evidence

Status: BLOCKED — reachable Govulncheck vulnerability; integration stopped before Git.

Ticket: [BBGO-SEC-001](../../tickets/BBGO-SEC-001.md)
Integration actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)
Source actor: Sr Dev — Grok Build (Grok 4.6 High)
Source baseline: `5289c564490a54f1adc5be1d451277d2576f7090`

## Source-drop verification

The five Grok-authorized source paths were present and matched the durable delivered
report's SHA-256 hashes and line counts. No unauthorized source path was changed.

## Test-first evidence

### Red

Command:

```text
python3 -m unittest scripts/security_policy_test.py
```

Executed with `scripts/security_policy.py`, `.github/workflows/security.yml`, and
`.github/workflows/sbom.yml` temporarily withheld outside the repository. Result:
expected failure, exit code 1. The required-source test failed because those three
required files were absent; the remaining test classes consequently reported missing
checker errors. The files were restored unchanged immediately afterward.

### Green attempt

Command:

```text
python3 -m unittest scripts/security_policy_test.py
```

Result: failure, exit code 1. `Ran 32 tests`; 31 passed and one errored:
`RequiredCommandsTest.test_committed_workflows_satisfy_the_checker`.

Redacted failure metadata:

```text
bb_go_security_policy.PolicyError:
sbom.yml must build the daemon and SBOM in the same environment
```

This is a blocking policy-test finding. No source was repaired, suppressed, baselined,
downgraded, or otherwise altered.

## Stop condition

Per BBGO-SEC-001, integration stopped before Actionlint, scanners, Gitleaks, maintained
race tests, SBOM build/generation/validation, `git diff --check`, evidence completion,
commit, or push. No secret value was recorded. No binary or SBOM was generated.

Reviewer disposition is required before any correction or further acceptance work.

## Integration safety interruption

After Grok correction cycle 1, Codex Luna was resumed. Before it changed repository state
or updated this evidence, it proposed a recursive deletion whose target was expressed
indirectly. The owner rejected the command as not safely reviewable, and the reviewer
interrupted Luna. No deletion, scanner, build, Git operation, or evidence mutation from
that resumed attempt occurred.

The ticket and Luna handoff now prohibit recursive deletion and deletion through
variables, substitutions, globs, symlinks, or other unresolved targets. Temporary state
must remain at the explicit disk-backed paths recorded below or on the ephemeral runner.
Integration may resume from green under those constraints.

## Local resource safety interruption

On the next resume, Luna began installing the pinned tools with
`GOBIN=/tmp/bbgo-sec-tools-20260829`. The owner stopped the run because `/tmp` is a tmpfs
RAM drive and the combined Go tool builds could cause memory exhaustion. The reviewer
interrupted Luna before any scanner, build, SBOM, repository mutation, or Git operation.

Read-only inspection confirmed `/tmp` is `tmpfs` and `/home/lars` is ext4. Four completed
binaries occupied about 115 MiB: `actionlint`, `gitleaks`, `gosec`, and `govulncheck`.
They were moved intact, without recursive deletion or copy amplification, to:

`/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829`

The old `/tmp/bbgo-sec-tools-20260829` path is absent. Resumed integration must reuse
those binaries, place all Go cache/temp and artifacts in the exact disk-backed paths in
the ticket/handoff, install only the missing CycloneDX tool, and leave the directories in
place.

## Correction-cycle resume validation

The corrected checker source was independently verified against the delivered report:
`scripts/security_policy.py` SHA-256 is
`4d80708822bf22e1e05abe56bb131db6176c217047001f68895dfdab3dc7058a`, line count 1,010,
with no other Grok source-path changes.

Green policy result:

```text
python3 -m unittest scripts/security_policy_test.py
Ran 32 tests; OK; exit code 0
```

Immutable-pin falsification:

```text
python3 -m unittest scripts.security_policy_test.CheckerRejectionTest.test_mutable_action_tag_is_rejected
Ran 1 test; OK; exit code 0
```

The existing in-memory test replaced an immutable Action SHA with a mutable tag and the
checker rejected it. No temporary falsification tree was created or deleted. Full policy
green was rerun afterward (`32/32`, exit code 0).

Actionlint:

```text
actionlint .github/workflows/go.yml .github/workflows/security.yml .github/workflows/sbom.yml
exit code 0; no diagnostics
```

## Blocking scanner failure

Command:

```text
(cd modern && govulncheck -test ./...)
```

Tool: `golang.org/x/vuln/cmd/govulncheck@v1.7.0`, invoked from the approved
disk-backed tool directory with the required Go cache/temp paths. Result: exit code 1
while loading packages. Redacted triage metadata: the scanner executable reported it was
built with Go 1.26, while the maintained module and downloaded Go standard-library inputs
require Go 1.27.0; package loading therefore emitted multiple compiler/toolchain errors
and did not produce a vulnerability result.

Per the ticket stop condition, integration stops here. `gosec`, Gitleaks, maintained race
tests, SBOM build/binary scan/generation/validation, `git diff --check`, evidence
completion, commit, and push were not run. No finding was suppressed or baselined, and
no secret value was recorded. The approved disk-backed tool/cache/temp/artifact
directories are intentionally left in place.

## Local Toolchain Correction 1

Per the authorized correction, exactly `govulncheck@v1.7.0`, `gosec@v2.29.0`,
`gitleaks@v8.30.1`, and `actionlint@v1.7.12` were rebuilt with `GOTOOLCHAIN=go1.27.0`
using the approved disk-backed tool, cache, and temp paths. `cyclonedx-gomod@v1.12.0`
was not rebuilt. `go version -m` verified all five binaries report `go1.27.0` and the
ticketed module versions.

## Blocking resumed scanner failure

Command:

```text
(cd modern && govulncheck -test ./...)
```

Tool: `golang.org/x/vuln/cmd/govulncheck@v1.7.0`, now rebuilt with Go 1.27.0. Result:
exit code 1 before package analysis because the sandbox blocked DNS/network access while
fetching the vulnerability database from `vuln.go.dev`. No vulnerability result was
produced and no secret value was recorded. Per the ticket stop condition, gosec, Gitleaks,
maintained race tests, SBOM build/binary scan/generation/validation, `git diff --check`,
evidence completion, commit, and push were not run.

Reviewer disposition: this is an execution-environment restriction, not a source or
vulnerability finding. The ticket now authorizes only the exact source and binary
Govulncheck invocations to use external network access for the official `vuln.go.dev`
database. No credentials or broader command receives that authority.

## Local toolchain disposition

Reviewer inspection originally found that `govulncheck`, `gosec`, `gitleaks`, and
`actionlint` were built with Go 1.26.0 while `cyclonedx-gomod` used Go 1.27.0. Luna then
rebuilt exactly those four tools and verified all five now report Go 1.27.0 and the
ticketed versions. No source repair, version change, suppression, or cleanup occurred.

## Network execution correction and blocking finding

The exact source scan was rerun with the authorized external access to the official
`vuln.go.dev` advisory database:

```text
(cd modern && govulncheck -test ./...)
```

The rebuilt pinned `golang.org/x/vuln/cmd/govulncheck@v1.7.0` completed advisory analysis
and exited 3. Redacted finding metadata: reachable vulnerability `GO-2024-3218` affects
`github.com/libp2p/go-libp2p-kad-dht@v0.42.1`; the scanner reported one vulnerability
affecting code in the maintained `modern/` module. The report also noted additional
required-module vulnerabilities that were not reachable; those are not being treated as
the blocking result here. No secret value was recorded.

Per BBGO-SEC-001, integration stopped immediately on this scanner finding. Gosec,
Gitleaks, maintained race tests, SBOM build/binary scan/generation/validation,
`git diff --check`, evidence completion, commit, and push were not run. No finding was
suppressed, allowlisted, downgraded, baselined, or repaired.

## Reviewer finding triage

The official Go record currently declares the module affected from version `0` and has
no fixed version. GitHub's reviewed advisory instead limits the vulnerable range to
`<=0.20.0`; BitBook was using `v0.42.1`. The upstream DHT maintainer's January 2026
disposition states that the attack is mitigated by routing-table IP-diversity filters.

Reviewer source inspection found that BitBook's single-DHT construction does not install
the upstream diversity filter. Its private-address development/LAN option explicitly
passes a nil diversity filter as well. This is a concrete defense gap independent of the
advisory-database disagreement. Reachable Finding Correction 1 therefore requires a
test-first, all-network-mode diversity-filter correction and an update to upstream
`v0.42.2`. No scanner exception is authorized in that correction.
