# BBGO-SEC-001 — Add Maintained-Daemon Security Gates and SBOM Evidence

Status: AUTHORIZED

Reviewer: Lead Engineer/Reviewer — Codex

Source actor: Sr Dev — Grok Build (Grok 4.6 High)

Integration actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Source baseline: `5289c564490a54f1adc5be1d451277d2576f7090`

## Objective

Add small, pinned, least-privilege security checks for the maintained `modern/` daemon
and a manual CycloneDX SBOM workflow. Routine validation must not build or upload release
binaries. The manual SBOM job may build one ephemeral Linux amd64 daemon solely for
artifact-aware vulnerability scanning; it uploads only the SBOM and leaves the unuploaded
binary to the ephemeral GitHub runner's own disposal after the job.

## Scope Boundary

- Go source, dependency, and built-artifact analysis applies to `modern/`, the module used
  by the current desktop client.
- Secret scanning applies to the complete repository history.
- The inherited GX/root tree is outside the first blocking source/dependency gate. Its
  retirement or findings baseline requires a separate ticket; silently treating it as
  maintained would make this gate unactionable.
- This ticket does not change daemon behavior, dependencies, release publication,
  signing, cross-platform packaging, or the existing legacy tree.

## Invariants

1. Security checks run on relevant pull requests and explicit manual dispatch only; a
   second push-triggered copy does not consume CI.
2. Routine security validation never builds or uploads a product binary.
3. SBOM generation is manual, uses the same target environment as its ephemeral binary,
   and uploads only a CycloneDX JSON document.
4. Every third-party GitHub Action is pinned to a full immutable commit SHA, every tool
   install is pinned to an exact version, and workflow permissions are read-only.
5. No finding is suppressed, allowlisted, downgraded, or converted to non-blocking by
   this ticket.
6. Secret values are redacted and no secret-finding report is uploaded as an artifact.
7. The existing Go workflow uses immutable Action pins and does not spend a full Go run
   on documentation-only changes.

## Authorized Source Paths

Grok Build may author only:

- `.github/workflows/go.yml`
- `.github/workflows/security.yml`
- `.github/workflows/sbom.yml`
- `scripts/security_policy.py`
- `scripts/security_policy_test.py`

Codex Luna may integrate those paths and author only this evidence record:

- `docs/security/BBGO-SEC-001-EVIDENCE.md`
- `docs/handoff/CURRENT_TASK.md`

No other path is authorized.

## Fixed Toolchain

- Go: `1.27.0`
- `golang.org/x/vuln/cmd/govulncheck@v1.7.0`
- `github.com/securego/gosec/v2/cmd/gosec@v2.29.0`
- `github.com/zricethezav/gitleaks/v8@v8.30.1`
- `github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.12.0`
- `github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`

The initial workflows must pin these Actions, with the readable release tag retained in
an adjacent comment:

- `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1` (`v7.0.1`)
- `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` (`v7.0.0`)
- `actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a`
  (`v7.0.1`, SBOM workflow only)

Changing a tool or Action version is a separate reviewed dependency update, not an
in-ticket substitution.

## Required Workflow Behavior

### `.github/workflows/security.yml`

- Trigger on `pull_request` changes to `modern/**`, any workflow, or either policy script;
  also permit `workflow_dispatch`.
- Declare top-level `permissions: contents: read` and a cancel-in-progress concurrency
  group.
- Check out complete history with `fetch-depth: 0` for Gitleaks.
- Install the four scanners/validators other than CycloneDX into a job-local temporary
  tool directory at the exact versions above.
- From `modern/`, run `govulncheck -test ./...` and `gosec ./...`.
- From the repository root, run `gitleaks git --redact --no-banner .`.
- Run `python3 -m unittest scripts/security_policy_test.py` and Actionlint against all
  three workflow files.
- Do not build a daemon, create an SBOM, upload artifacts, add write permissions, or use
  `continue-on-error`.

### `.github/workflows/sbom.yml`

- Trigger only on `workflow_dispatch`.
- Declare top-level `permissions: contents: read` and a cancel-in-progress concurrency
  group.
- Use `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0` for both the build and SBOM
  generation.
- Build `./cmd/bitbookd` from `modern/` into the runner's temporary directory; never
  write a binary into the repository worktree.
- Run `govulncheck -mode binary` against that exact temporary binary.
- Generate CycloneDX JSON for the application from `modern/` with packages and licenses:
  `cyclonedx-gomod app -json -packages -licenses -main cmd/bitbookd -output <temporary
  .cdx.json> .`.
- Validate that the document is JSON, declares CycloneDX, identifies
  `github.com/larslarsen/bb-go/modern` as its root component, and contains non-empty
  `components` and `dependencies` arrays.
- Upload only the `.cdx.json` document with `retention-days: 7`. Do not upload the daemon,
  publish a release, or request write permission.

### `.github/workflows/go.yml`

- Preserve its existing compile, social-boundary, and maintained-P2P commands.
- Replace mutable Action tags with the exact immutable checkout/setup-go pins in this
  ticket.
- Retain `push` and `pull_request`, but restrict both to Go source, Go module/sum files,
  `vendor/**`, `gx/**`, `scripts/go.sh`, and this workflow. Documentation-only changes
  must not trigger the Go job.
- Retain `permissions: contents: read`, Go 1.27.0, and disabled setup-go caching.

## Test-First Integration Contract

Grok Build authors `scripts/security_policy_test.py` before the checker or workflows
and reports the files as separate test-source and production-source sections. It does
not execute them.

Codex Luna then performs the evidence sequence:

1. Integrate only `scripts/security_policy_test.py` and run
   `python3 -m unittest scripts/security_policy_test.py`. Record the expected red result:
   the required checker/workflows do not yet exist.
2. Integrate `scripts/security_policy.py` and all three workflow changes, then run the
   same command and record green.
3. Falsify the high-value immutable-pin check with the targeted in-memory unittest
   `scripts.security_policy_test.CheckerRejectionTest.test_mutable_action_tag_is_rejected`.
   It replaces one 40-character Action SHA with a mutable tag in memory and must record
   rejection without creating or deleting a temporary tree. Prove full green again.

The policy tests must independently assert the seven invariants above, including that the
routine workflow has no build/upload step and the manual workflow cannot upload a binary.
The checker and tests must use only Python's standard library; no project or CI dependency
may be added. They must also assert immutable pins and documentation-only path filtering
for the existing Go workflow. Tests that merely search for a success string emitted by
the workflow are rejected.

## Codex Luna Acceptance Commands

Codex Luna records exact versions, commands, exit codes, and summarized counts without
publishing secret material:

```text
python3 -m unittest scripts/security_policy_test.py
actionlint .github/workflows/go.yml .github/workflows/security.yml .github/workflows/sbom.yml
(cd modern && govulncheck -test ./...)
(cd modern && gosec ./...)
gitleaks git --redact --no-banner .
(cd modern && go test -race ./... -count=1)
git diff --check
```

Codex Luna also executes the SBOM workflow's build, binary scan, generation, and structural
validation commands locally with a temporary directory and records the resulting SBOM's
SHA-256 hash, byte size, CycloneDX spec version, component count, and dependency count.
The temporary binary and SBOM are not committed.

## Finding Policy and Stop Conditions

- Any Govulncheck reachable vulnerability, Gosec finding, Gitleaks finding, Actionlint
  error, policy-test failure, SBOM validation failure, or existing test regression stops
  integration before Git.
- Codex Luna reports metadata needed for triage but never echoes or records a detected secret
  value.
- No baseline, ignore rule, inline suppression, `continue-on-error`, or altered exit code
  may be added. The reviewer must issue a separate remediation or time-bounded exception
  ticket for every proposed disposition.
- Grok Build stops after the bounded source drop. Codex Luna stops after publishing the
  evidence and Git change for reviewer inspection. Only the reviewer may accept it.
- No actor may use `rm -rf` or another recursive deletion command with a variable,
  substitution, glob, symlink-derived path, or unresolved target. Local tool, binary,
  SBOM, and falsification files may remain under `/tmp` for operating-system cleanup;
  GitHub runner state is discarded with the ephemeral runner.

## Acceptance Criteria

- The red, green, and immutable-pin falsification evidence is complete and non-vacuous.
- All Codex Luna acceptance commands pass with no suppressed finding.
- CI triggers and path filters avoid duplicate or irrelevant runs.
- Routine CI neither builds nor uploads binaries; manual SBOM CI uploads only CycloneDX
  JSON.
- Tool and Action pins, permissions, and artifact retention match this ticket exactly.
- No unauthorized path or dependency changed.

## Reviewer Decision

Authorized on 2026-08-29. Complete durable prompts:

- `docs/handoff/GROK_BUILD_BBGO_SEC_001.md`
- `docs/handoff/CODEX_LUNA_BBGO_SEC_001.md`

Reviewer acceptance remains pending.

## Correction Cycle 1 — Authorized

Codex Luna's first green attempt ran 32 policy tests: 31 passed and
`RequiredCommandsTest.test_committed_workflows_satisfy_the_checker` errored with
`sbom.yml must build the daemon and SBOM in the same environment`. Integration stopped
before validators, scanners, builds, Git, or push, as required.

Reviewer inspection found that `check_sbom_workflow` compares the two result tuples
returned by `_find_step_containing` with `is`. Each lookup creates a different tuple even
when both tuples contain the same parsed workflow step. The existing test correctly
caught this wrong rejection.

Grok Build is authorized to correct only `scripts/security_policy.py` so the checker
compares the underlying step objects (or an equivalently exact same-step property), not
the wrapper tuples. No test change, workflow change, broader refactor, execution, install,
or Git operation is authorized. Durable correction prompt:
`docs/handoff/GROK_BUILD_BBGO_SEC_001_CORRECTION_01.md`.

## Integration Safety Interruption

During resumed integration, Codex Luna proposed a recursive removal whose target was
expressed indirectly. The owner rejected it because the destructive target was not
human-reviewable. The reviewer interrupted Luna before execution. The worktree and
evidence were unchanged.

Integration may resume only under these additional constraints:

- do not run `rm -rf`, `rm -r`, `find -delete`, or an equivalent recursive deletion;
- do not delete a path expressed through a variable, substitution, glob, or symlink;
- do not clean temporary scanner, binary, SBOM, or falsification state recursively;
- leave temporary state in `/tmp` or on the ephemeral runner for system cleanup; and
- perform immutable-pin falsification with the existing in-memory unittest
  `CheckerRejectionTest.test_mutable_action_tag_is_rejected`, not a temporary file tree.

## Local Resource Safety Interruption

On the next resumed attempt, Codex Luna began installing pinned scanner binaries to
`/tmp/bbgo-sec-tools-20260829`. The owner stopped the run because local `/tmp` is a tmpfs
RAM drive and several Go tool builds could exhaust memory. The reviewer interrupted Luna.
No scanner had run and no repository change or Git operation occurred.

Four completed binaries (`actionlint`, `gitleaks`, `gosec`, and `govulncheck`; about 115
MiB total) were moved without copying or deletion from the RAM-backed path to this exact
ext4-backed directory:

`/home/lars/OpenBazaar/.security-tools/bbgo-sec-tools-20260829`

Resumed local acceptance must:

- reuse those four binaries and not reinstall them;
- install only the missing `cyclonedx-gomod@v1.12.0` into that exact tool directory;
- set `GOCACHE=/home/lars/OpenBazaar/.security-cache/go-build-20260829` and
  `GOTMPDIR=/home/lars/OpenBazaar/.security-tmp/go-tmp-20260829` for any remaining Go
  install, build, test, or scan operation;
- write the local daemon and SBOM only under
  `/home/lars/OpenBazaar/.security-artifacts/bb-go-sec-001-20260829`; and
- never use local `/tmp`, `mktemp`, or an unresolved directory for this ticket's tools,
  caches, work files, binary, or SBOM. Leave all disk-backed task directories in place.

## Local Toolchain Correction 1 — Authorized

The first real `govulncheck -test ./...` attempt stopped during package loading because
the pinned scanner binary was built with Go 1.26 while the maintained module requires Go
1.27.0. This was not a vulnerability result. Embedded build metadata then confirmed that
`govulncheck`, `gosec`, `gitleaks`, and `actionlint` were all built with Go 1.26;
`cyclonedx-gomod` was already built correctly with Go 1.27.0.

Codex Luna may rebuild exactly these four existing pinned binaries with
`GOTOOLCHAIN=go1.27.0` in the already authorized disk-backed tool/cache/temp directories:

- `golang.org/x/vuln/cmd/govulncheck@v1.7.0`
- `github.com/securego/gosec/v2/cmd/gosec@v2.29.0`
- `github.com/zricethezav/gitleaks/v8@v8.30.1`
- `github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`

Do not rebuild CycloneDX or change a version. Before resuming scanners, verify with
`go version -m` that all five tool binaries report Go 1.27.0. All destructive-action,
tmpfs, exact-path, no-cleanup, finding, and stop rules remain in force.

## Govulncheck Network Execution Correction 1 — Authorized

After all five binaries were verified at Go 1.27.0 and exact pinned versions, the resumed
`govulncheck -test ./...` attempt stopped before analysis because sandbox DNS/network
access to `vuln.go.dev` was blocked. This was not a vulnerability result.

Codex Luna may rerun the exact source Govulncheck command with network access outside the
network-restricted sandbox. If it passes without a finding, Luna may later run the exact
ticketed `govulncheck -mode binary` command against the explicit disk-backed daemon path
with the same network access. No other command receives expanded network authority.
Govulncheck may contact its official vulnerability database; it must not receive secrets
or credentials. All existing exact-path, disk-backed, no-cleanup, finding, and stop rules
remain in force.

## Reachable Finding Correction 1 — Authorized

The authorized source scan reached the advisory database and reported `GO-2024-3218`
against `github.com/libp2p/go-libp2p-kad-dht@v0.42.1`. Integration stopped before every
later acceptance command and before Git. Reviewer triage established:

- the Go vulnerability record currently marks the module from version `0` with no fixed
  version, so a dependency-only upgrade cannot make Govulncheck green;
- GitHub's reviewed `GHSA-mqr9-hjr8-2m9w` describes affected versions as `<=0.20.0`;
- upstream's January 2026 maintainer disposition says this attack is mitigated by IP
  diversity filters; and
- the maintained BitBook `network.New` constructs a single DHT without installing
  `NewRTPeerDiversityFilter`, so the relevant mitigation is absent. The
  `AllowPrivateAddresses` branch also explicitly supplies a nil diversity filter.

Sr Dev — Grok Build is authorized to author a test-first correction limited to:

- `modern/network/node_test.go`
- `modern/network/node.go`
- `modern/go.mod`

The correction must first add a test that fails because the routing-table IP-diversity
filter is absent, then install the upstream DHT diversity filter using the upstream Amino
limits in every network mode. Allowing loopback/RFC1918 addresses for local or LAN use
must not disable routing-table diversity. Update the direct DHT requirement from
`v0.42.1` to the current upstream release `v0.42.2`; Luna, not Grok, will regenerate
`modern/go.sum` with the pinned Go 1.27 toolchain.

Grok must not change scanner policy, suppress or allowlist the advisory, edit workflows
or evidence, run commands, install tools, or use Git. The existing scanner finding is
expected to remain until a separate reviewer decision accounts for the contradictory
advisory metadata; this correction addresses the concrete missing mitigation first.

Codex Luna must regenerate `modern/go.sum` with Go 1.27, reconstruct the focused red and
falsification by temporarily removing only the diversity-filter production hunks with
bounded patch edits, restore and hash-check them, then run the same test green and run
the maintained race suite. The exact targeted command is:

```text
GOTOOLCHAIN=go1.27.0 go test ./network -run '^TestDHTRoutingTableEnforcesIPDiversity$' -count=1
```

After green and race success, Luna must rerun source Govulncheck. Record the exact
resolved DHT version and reachable finding count, then stop again on any finding. This
cycle does not authorize Gosec, Gitleaks, binary build/scan, SBOM, Git, or a scanner
exception.

## Localhost Socket Execution Correction 1 — Authorized

The bounded red/falsification command stopped before the intended assertion because the
sandbox denied binding `127.0.0.1:0`. Luna restored the exact production hunks and
verified the delivered `modern/network/node.go` hash. This is an execution-environment
restriction, not a source or test result.

Codex Luna may repeat only the exact targeted red/falsification and green commands, plus
the existing maintained Go race-suite command, outside the socket-restricted sandbox so
the in-process nodes can bind loopback ephemeral ports. No public peer, credential,
privileged port, or other expanded authority is authorized. All commands must retain the
exact disk-backed GOCACHE and GOTMPDIR. The same bounded patch removal, exact restoration,
hash verification, stop-on-failure, no-cleanup, and no-Git rules remain in force.

## Reviewed Govulncheck Exception 1 — Authorized

Reachable Finding Correction 1 proved the mitigation test red in both modes without the
filter, green with it, and passed the maintained race suite. Govulncheck still reports
`GO-2024-3218` on DHT `v0.42.2` because its Go record has no fixed version. This reviewed
exception is narrower than the scanner record and must not make any other finding
non-blocking.

Exception owner: Lead Engineer/Reviewer — Codex.

Affected dependency and source boundary: exactly
`github.com/libp2p/go-libp2p-kad-dht@v0.42.2`, reached from the maintained networking
paths including `modern/network/node.go` and `modern/direct/service.go`.

Rationale: GitHub's reviewed `GHSA-mqr9-hjr8-2m9w` limits affected versions to
`<=0.20.0`, while the Go record marks all versions with no fix. Upstream's maintainer
states that the attack is mitigated by routing-table IP-diversity filters. BitBook now
installs that upstream filter with Amino limits in every mode, and a real multi-node
regression test proves the defense is present and enforcing both table and CPL limits.

Expiry: 2026-11-29. Removal/re-review is required on that date, on any DHT dependency
version change, when the Go advisory gains corrected version metadata, when upstream
publishes a distinct patched mechanism/version, or if the mitigation test or filter
configuration changes—whichever occurs first.

Sr Dev — Grok Build is authorized to implement a test-first SARIF adjudicator and wire
it into source and binary Govulncheck execution. It must:

- invoke pinned Govulncheck without `continue-on-error` and distinguish scanner exit 0,
  reachable-finding exit 3, and every execution failure;
- accept exit 3 only when SARIF is valid Govulncheck v1.7.0 output from official
  `https://vuln.go.dev`, the only `error` result is `GO-2024-3218`, every vulnerable DHT
  trace identifies exactly `github.com/libp2p/go-libp2p-kad-dht@v0.42.2`, and the
  exception is not expired;
- reject any additional error, wrong/missing module version, malformed/empty output,
  wrong scanner/database/mode, invocation failure, or expired exception;
- keep note-level non-reachable module results visible without treating them as a
  reachable finding;
- run the focused IP-diversity regression test immediately before source adjudication;
  and
- preserve normal hard-failure behavior for Gosec, Gitleaks, Actionlint, policy tests,
  SBOM generation/validation, and all other checks.

Authorized developer paths:

- `scripts/govulncheck_policy_test.py`
- `scripts/govulncheck_policy.py`
- `scripts/security_policy_test.py`
- `scripts/security_policy.py`
- `.github/workflows/security.yml`
- `.github/workflows/sbom.yml`

No Go source, module, sum, other workflow, governance, evidence, command execution,
install, Git, commit, push, or GitHub state change is authorized for Grok.

### Exception Source Correction 1 — Authorized

Reviewer source inspection found two acceptance defects before execution:

1. `.github/workflows/sbom.yml` deletes `"${binary}"`. Although non-recursive, this is a
   variable-resolved deletion target and violates the ticket's standing safety rule.
   The binary is already outside the worktree, is never uploaded, and must be left for
   the ephemeral GitHub runner to discard.
2. Successful adjudication prints the full SARIF. The real source result is roughly
   220,000 lines, so this would waste CI log capacity. A clear validated summary,
   including note-level result IDs/messages and reviewed-exception metadata, is enough.

Grok may correct only `scripts/govulncheck_policy_test.py`,
`scripts/govulncheck_policy.py`, `scripts/security_policy_test.py`,
`scripts/security_policy.py`, and `.github/workflows/sbom.yml`. Tests must be authored
first. Policy tests must reject variable/substitution/glob/symlink-derived deletion in
the SBOM workflow. The adjudicator must not print raw SARIF on success, while retaining
its fail-closed validation and clear summary. No other behavior or path may change.

After a conforming correction, Luna must run both complete policy suites (42 workflow
policy tests and 49 Govulncheck-policy tests), targeted rejection falsification,
Actionlint, the real source adjudicator, Gosec, redacted Gitleaks, the maintained race
suite, disk-backed daemon build and binary adjudicator, CycloneDX generation/validation,
and `git diff --check`, in that order. The actual source and binary adjudicators may use
official advisory network access. Every local Go command and artifact retains the exact
disk-backed paths already authorized. No local `/tmp`, cleanup, deletion, or unresolved
target is allowed. Any non-reviewed reachable error or other command failure stops
before Git.

### Targeted Unittest Selector Correction 1 — Authorized

Both complete suites passed (42 and 49 tests), proving every rejection fixture green.
The additional targeted selector failed before loading because its dotted `scripts.*`
module form is not importable in this repository. This is an invocation-addressing
failure, not a test result or source defect.

Luna may rerun exactly these file-path selectors from the repository root:

```text
python3 -m unittest -k additional_error scripts/govulncheck_policy_test.py
python3 -m unittest -k wrong_dht_version scripts/govulncheck_policy_test.py
python3 -m unittest -k exception_on_expiry_date scripts/govulncheck_policy_test.py
python3 -m unittest -k sbom_variable_deletion scripts/security_policy_test.py
python3 -m unittest -k mutable_action_tag scripts/security_policy_test.py
```

Each must load and pass exactly one rejection test. On success, resume at Actionlint.
No source/test edit, package initializer, alternate selector, or Git action is authorized.

## Gosec Finding Correction 1 — Authorized

After policy and source-adjudicator success, pinned Gosec v2.29.0 stopped integration on
two maintained-source findings:

- G115, high/medium, `modern/direct/service.go:716`: `int` frame length converted to
  `uint32`; and
- G304, medium/high, `modern/network/identity.go:17`: identity key read through a
  caller-supplied path.

No suppression or baseline is authorized. Sr Dev — Grok Build must author tests first
and then correct only:

- `modern/direct/service_test.go`
- `modern/direct/service.go`
- `modern/network/identity_test.go`
- `modern/network/identity.go`
- `modern/network/open.go`

Direct framing must derive the encoded `uint32` only after a testable checked bound that
proves the size fits both the protocol's `maxFrameBytes` and `uint32`; boundary tests
must cover immediately below, at, and above the protocol limit plus a value above
`math.MaxUint32` without allocating that payload. Existing wire format must not change.

Identity storage must replace arbitrary full-path reading with a root-scoped API using
Go 1.27 `os.Root` and the fixed `identity.key` name beneath the supplied data directory.
All reads, writes, and final rename must remain confined to that opened root. Tests must
retain persistence/0600 coverage and prove that an `identity.key` symlink escaping the
root is rejected without reading or altering the outside file. `network.Open` must use
the new root-scoped contract.

Grok must not run commands beyond read-only hashes/counts, change any scanner policy,
add a suppression, change dependencies, edit other paths, install, use Git, commit, push,
or change GitHub state. Luna owns red/green/falsification, Gosec rerun, broader race
testing, evidence, and Git.

Luna must reconstruct red with bounded production-hunk reversal, restore exact hashes,
run focused direct/network green, falsify the frame boundary and identity escape
mechanisms, restore/hash-check again, and rerun pinned Gosec. Every Go command uses the
named disk-backed GOCACHE/GOTMPDIR. On clean Gosec, resume at redacted Gitleaks; on any
failure/finding, stop before later checks and Git without repair or suppression.

## Reviewed Gitleaks Baseline 1 — Authorized

Pinned Gitleaks scanned 3,379 inherited commits and stopped on 25 redacted findings across
24 immutable commits. Reviewer triage used only `--redact=100` reports stored at the
explicit disk-backed artifact path. All 24 `generic-api-key` results are identified by
their redacted match context as dummy keys, test identity/private-key fixtures,
documentation/example cookies/passwords/rating keys, or a vendored CLI `--key` example.
The single `private-key` result is `.travis/sign.key.gpg`; content-safe file-type analysis
identifies its 6,835-byte blob as a PGP **public** key block. None of the 25 findings is in
the maintained `modern/` module, and no credential requiring rotation was identified.

The reviewed source report is exactly:

`/home/lars/OpenBazaar/.security-artifacts/bb-go-sec-001-20260829/gitleaks-redacted.json`

SHA-256: `ac71e27a9f2954f7d148b8dd9d630c587abbb92b2a183f53b267e7739f418e00`
Size: 21,758 bytes. Count: 25. Every `Secret` field is exactly `REDACTED`.

Baseline owner: Lead Engineer/Reviewer — Codex. Expiry/re-review date: 2026-11-29.
Removal/re-review occurs on expiry, Gitleaks version/rule change, baseline format change,
root-tree retirement/history rewrite, any baseline-entry change, or evidence that an
entry is a live credential—whichever comes first.

Sr Dev — Grok Build is authorized to author a test-first exact redacted baseline and
fail-closed validator limited to:

- `security/gitleaks-baseline.json`
- `scripts/gitleaks_baseline_test.py`
- `scripts/gitleaks_baseline.py`
- `scripts/security_policy_test.py`
- `scripts/security_policy.py`
- `.github/workflows/security.yml`

The validator must require exact reviewed SHA/content, 25 unique entries, only the exact
reviewed rule/file/commit/line identities, literal redaction in every secret/match,
zero `modern/` entries, owner/expiry constants, and failure on the review date or later.
Workflow policy must require baseline validation immediately before one Gitleaks command
using `--redact=100` and `--baseline-path security/gitleaks-baseline.json`; it must reject
any other baseline, broad path/regex/commit allowlist, missing validation/redaction,
report upload, or non-blocking exit behavior. New findings remain blocking.

Grok may read only the exact verified redacted artifact above as baseline input. It must
not read unredacted reports or matched source values, run scanners/tests/validators,
change Go/dependencies/other paths, install, use Git, commit, push, or change GitHub
state. Final read-only hashes/counts are allowed.

The delivered source report and exact hashes are preserved in
`docs/handoff/GROK_BUILD_BBGO_SEC_001_GITLEAKS_01.md`. Reviewer read-only inspection
confirmed the baseline is byte-identical to the approved redacted artifact and found no
widened allowlist. Codex Luna may now execute the baseline/policy suites, bounded
mutation selectors, actual validator, Actionlint, and exact pinned Gitleaks command. On
zero new findings it resumes the remaining race, disk-backed daemon/binary adjudication,
CycloneDX validation, and final diff sequence. All prior stop, no-secret-output,
disk-backed-path, no-local-`/tmp`, no-cleanup, and no-Git-before-green rules remain in
force.

## Reviewed Gitleaks Baseline Correction 1 — Authorized

Codex Luna verified the delivered source hashes and ran the complete 25-test baseline
suite. One test failed: the validator accepted the non-redacted match mutation
`UNREDACTED_MATCH` because its substring check found `REDACTED` inside the larger word.
Luna stopped before the workflow-policy suite, validator, Actionlint, Gitleaks, race,
build, SBOM, or Git.

Sr Dev — Grok Build may edit only `scripts/gitleaks_baseline_test.py` and
`scripts/gitleaks_baseline.py` under the exact instructions in
`docs/handoff/GROK_BUILD_BBGO_SEC_001_GITLEAKS_CORRECTION_01.md`. Existing failing test
source must be preserved; any boundary tests are authored before production correction.
The predicate must require `REDACTED` as a complete marker rather than a substring of an
ASCII identifier and reject prefix/suffix forms without emitting the mutated value.
The baseline JSON and every other path/invariant remain unchanged. Grok does not execute;
Luna owns rerun and continuation. All prior stop and resource-safety rules remain in
force.

Grok delivered the two-path correction test-first, and reviewer read-only inspection
accepted its explicit `[A-Za-z0-9_]` marker boundaries and non-echoing error behavior.
The exact delivered hashes and Luna continuation are recorded in
`docs/handoff/GROK_BUILD_BBGO_SEC_001_GITLEAKS_CORRECTION_01.md`. Luna may verify those
hashes and resume at the complete baseline suite; no later gate or Git is authorized
unless the corrected suite and targeted boundary selectors pass.
