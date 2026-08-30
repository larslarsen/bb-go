# BBGO-SEC-001 Integration Evidence

Status: BLOCKED — correction test environment failure; integration stopped before Git.

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

## Reachable Finding Correction 1 integration

The three-path Grok source drop was independently verified before execution:

```text
modern/network/node_test.go  448 lines  SHA-256 2c791449967c412bc35756400dc832b3ec31557f0d9d868882e5103a4ea4ba74
modern/network/node.go       261 lines  SHA-256 5add3a890d232af2ed8f53fcb9bd062660b69608937fdb0ea0fa6e0d86e057d9
modern/go.mod                133 lines  SHA-256 a69dbd8a9ab76f75f8329782d1f3309122ac933c1ea10ab8503267f400d3b2ce
```

`GOTOOLCHAIN=go1.27.0 go mod tidy` completed using the exact disk-backed cache/temp
paths. It updated the DHT checksums in `modern/go.sum` for v0.42.2 and promoted the
already-required kbucket module to a direct requirement because the authorized regression
test imports it. Resulting `modern/go.sum` is 374 lines, SHA-256
`4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a`.

For the required red reconstruction and falsification, only the Amino import, diversity
filter option, and prior private-mode nil option were temporarily changed. The exact
targeted command was run with Go 1.27.0 and exited 1, but could not reach the intended
`constructed BitBook DHT has no routing-table IP-diversity filter` assertion: both
subtests failed first because sandbox socket permissions denied binding
`127.0.0.1:0`. This is recorded as an environmental test failure, not a source finding.
The three production hunks were immediately restored with bounded patches; `node.go`
returned to the delivered SHA-256 above. A post-restore `git diff --check` over the four
correction paths passed.

Per the correction stop condition, targeted green, maintained race tests, and the
network-authorized source Govulncheck rerun were not run. The known advisory therefore
was not re-evaluated. No source/test repair, suppression, allowlist, cleanup, or Git
operation occurred.

## Localhost Socket Execution Correction 1

With the authorized loopback ephemeral-port execution, the bounded red/falsification
was rerun after removing only the Amino import and diversity-filter option and restoring
the prior private-mode nil option:

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$' -count=1
```

Result: exit code 1 as intended. Both subtests rejected the baseline with
`constructed BitBook DHT has no routing-table IP-diversity filter`. The production hunks
were restored immediately, and hash verification confirmed `modern/network/node.go`
`5add3a890d232af2ed8f53fcb9bd062660b69608937fdb0ea0fa6e0d86e057d9` and
`modern/network/node_test.go`
`2c791449967c412bc35756400dc832b3ec31557f0d9d868882e5103a4ea4ba74`.

Targeted green then passed:

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$' -count=1
ok; exit code 0
```

The maintained race suite passed all five packages (`api`, `direct`, `network`,
`social`, and `cmd` with no test files):

```text
GOTOOLCHAIN=go1.27.0 GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829 GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829 go test -race ./... -count=1
exit code 0
```

The network-authorized source Govulncheck rerun was then performed. It exited 3 with one
reachable vulnerability, `GO-2024-3218`, still affecting
`github.com/libp2p/go-libp2p-kad-dht@v0.42.2`; the advisory reports no fixed version.
The scanner also reported two non-reachable required-module vulnerabilities. This
reachable advisory remains the sole blocking reachable result. No finding was
suppressed, allowlisted, downgraded, baselined, or repaired. Gosec, Gitleaks, later
binary/SBOM checks, `git diff --check`, commit, and push were not run.

## Reviewer exception disposition

The concrete missing mitigation is now implemented and independently falsified: both
normal and private-address modes fail the focused test without the filter, pass with it,
and the complete maintained race suite passes. The remaining scanner result conflicts
with GitHub's reviewed affected range and upstream's mitigation disposition.

Reviewed Govulncheck Exception 1 is therefore authorized only for
`GO-2024-3218` on exact DHT `v0.42.2`, owned by the Lead Engineer/Reviewer and expiring
2026-11-29. Any dependency change, advisory correction, upstream patched mechanism,
mitigation change, additional reachable finding, or expiry forces failure and re-review.
Grok must implement this as a fail-closed SARIF adjudicator with independent tests; no
exception has yet been integrated or executed.

Reviewer disposition: the bind denial is a sandbox execution restriction. The exact
targeted red/green and maintained race commands may be repeated with bounded localhost
socket authority. They remain offline, credential-free, limited to loopback ephemeral
ports, and use the same disk-backed cache/temp paths. No source repair or broader command
authority is authorized.

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
