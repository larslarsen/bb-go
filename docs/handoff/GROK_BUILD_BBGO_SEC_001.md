# Grok Build Handoff — BBGO-SEC-001

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Source-content baseline: `5289c564490a54f1adc5be1d451277d2576f7090`

Read completely before editing:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `tickets/BBGO-SEC-001.md`
5. `.github/workflows/go.yml`

Implement exactly `BBGO-SEC-001`. Author `scripts/security_policy_test.py` first, then
the checker and three workflow files. Preserve the existing Go workflow commands while
pinning its Actions and restricting its triggers exactly as ticketed. Use only Python's
standard library in policy code. Do not change any dependency or other path.

Do not run tests, scanners, builds, validators, or acceptance commands. Do not install
tools. Do not edit governance, tickets, handoffs, or evidence. Do not use Git, commit, or
push.

When finished, stop and report:

- every added/modified path, SHA-256, and line count;
- test-source-first confirmation;
- the invariants independently checked by each policy test;
- the exact workflow triggers, permissions, pins, tool versions, and uploaded paths;
- confirmation that routine jobs do not build/upload binaries and SBOM is manual-only;
- any ambiguity or unimplemented requirement; and
- confirmation that no command, install, Git operation, or out-of-scope edit occurred.

## Delivered Source Report

Execution date: 2026-08-29

Grok Build reported test-source-first authoring and no test, scanner, build, validator,
install, or Git execution. The bounded drop contains:

| Path | Change | Lines | SHA-256 |
|---|---|---:|---|
| `scripts/security_policy_test.py` | added; 32 tests in 11 classes | 578 | `61602730b62f8e681f4be7eb655c8d0342c73cdb0fe924a18026a11f7c68b1ff` |
| `scripts/security_policy.py` | added | 1,010 | `f03becbf8e73c1a05a97b62a0115b833bdd5bdf9ac35bbd76e311db2eff74f38` |
| `.github/workflows/go.yml` | modified | 47 | `f286aef359933fbd7de54b65649fb7f59ec2190c951604c2dd7319695fad3041` |
| `.github/workflows/security.yml` | added | 61 | `e02be0dff35e493f9d5a5fd0d959c01ee076f88637cb133c585e30bf4f08386f` |
| `.github/workflows/sbom.yml` | added | 59 | `26b12aa444af74ea32cc883c03ce7e5e69c88fdf3cb2e1ebe48c18a4f381c1bf` |

The tests cover all seven ticket invariants plus explicit rejection of mutable Action
tags, security push triggers, binary artifact uploads, documentation paths in Go CI,
`continue-on-error`, write permissions, missing Gitleaks redaction, and unpinned tool
installs. Independent CycloneDX fixture tests reject wrong format/root and empty
component/dependency graphs.

The security workflow is pull-request/manual only, has complete-history checkout, and
installs the four pinned scanners into runner-temporary `GOBIN`. The SBOM workflow is
manual-only, builds and scans one temporary Linux amd64 daemon, deletes it, and uploads
only `bitbookd.cdx.json` for seven days. The Go workflow retains its commands while
pinning Actions and filtering documentation-only changes.

Grok reported no unimplemented ticket requirement and no out-of-scope edit. During
source authoring it corrected two test issues before production completion: a
wrong-reason environment identity assertion and Gitleaks matching against its install
module rather than only its invocation.

## Reviewer Source Inspection

Codex independently read all five changed files, confirmed the reported hashes and line
counts, and found no scope violation or obvious workflow-security defect. The custom
standard-library YAML/policy implementation is substantial (1,010 lines); source review
does not accept it by inspection. Integration is contingent on its complete 32-test
suite, immutable-pin falsification, Actionlint, all real scanners, maintained race tests,
and successful generation and validation of an actual CycloneDX document. Any failure
returns the drop for correction or opens a separate finding ticket; Luna must not patch
the source.
