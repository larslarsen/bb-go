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
artifact-aware vulnerability scanning; it uploads only the SBOM and destroys the binary
with the runner.

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
3. Falsify the high-value immutable-pin check in a temporary copy by replacing one
   40-character Action SHA with a mutable tag. Record that the policy check rejects it,
   restore the untouched source, and prove green again.

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
