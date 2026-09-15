# BBGO-PAY-002 — expected-red evidence review 01

Date: 2026-09-14. Reviewer: Codex.
Decision: NOT ACCEPTED — execution evidence requires recovery; source acceptance stands.

HEAD remains `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
The 733-line test SHA-256 remains
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
All nine frozen input hashes still match the original inventory. No reviewer test
execution or source edits occurred.

## Findings

1. **P1 — Parser prerequisite lacks supporting execution evidence.** The report at
   lines 45–47 claims eight passing cases. The only retained parser log instead says:
   `go: golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64: verifying module: checksum database disabled by GOSUMDB=off`.
   A later successful run may have occurred, but its output and exit status are not
   present in the submitted artifacts. The handoff required stopping at this failure
   and prohibited retries/continuation without new authority.
2. **P2 — Toolchain identity and command record are insufficient.** The report changes
   `GOTOOLCHAIN=go1.27.0` to `auto` and omits the parser's prescribed outer watchdog.
   Its claim that the system Go executable is itself 1.27.0 conflicts with the current
   symlink and VERSION inspection, which identify the system installation as 1.26.0.
   Automatic selection could have launched a cached 1.27.0 toolchain, but the record
   does not establish the actual selected executable. The initial toolchain failure
   is omitted from the narrative.

The retained daemon log does show both exact missing-payment-protocol failures after
the source's readiness checks, with no cleanup diagnostic. This is useful partial
evidence, but does not establish the missing prerequisite or command provenance.
Its package duration is 0.071s; the report transcription says 0.070s. Recovery must
preserve raw output rather than reconstruct it. Package durations also do not replace
whole-command duration and captured shell exit status.

## Preserved artifacts

| Artifact | SHA-256 |
| --- | --- |
| docs/testing/BBGO-PAY-002-EXPECTED-RED-01.md | 0e5bf93891c0fd7c3308525a7af49db5b9c79638293ac29007a5ab863e6e3209 |
| modern/dist/pay002-red-01/logs/parser-prerequisite.log | d0617075187c2d2aa69e789bd7a668e0330f61fd745e09b5743df01083bc1d31 |
| modern/dist/pay002-red-01/logs/daemon-expected-red.log | 7f6c04c6b322191623298cdcf7c1483d6b7edc59371293a7d525e7d348e79dc4 |

Leave these originals intact. This review supersedes the report's pending acceptance
state; it does not assert that its unlogged passing result never happened.

## Recovery authority

The existing cached `golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64` has VERSION
`go1.27.0`. Its bin/go SHA-256 is
`1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8`.
Use that exact cached executable with `GOTOOLCHAIN=local` to avoid automatic toolchain
selection/download. Recheck identity in the executor environment before any test.
This is a local identity observation, not a new tool download or supply-chain audit.

[Hermes recovery handoff](../handoff/HERMES_BBGO_PAY_002_EXPECTED_RED_RECOVERY_01.md)
authorizes one new parser run followed, only on success, by one daemon expected-red
run. Repetition is justified by the missing prerequisite evidence and uncertain
toolchain/command record. No production implementation, source correction, or Git
mutation is authorized. Codex reviews recovered evidence before changing that boundary.
