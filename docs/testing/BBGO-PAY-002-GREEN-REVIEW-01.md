# BBGO-PAY-002 phase A — integrated green review 01

Date: 2026-09-15 UTC. Reviewer: Codex.
Decision: PHASE A RUNTIME ACCEPTED. Publication requires the staged-content secret
scan and exact-path Git checks in the publication handoff. No further implementation
is authorized under phase A.

## Source and evidence identity

HEAD before publication: `801f5d55d80fe02c6eb512ff35f8c09acfd679af`.
Accepted main.go: 225 lines, SHA-256
`6333d04275944872f2c1ba533642fe16f06b807e7c8e6af537cecdd504c1269b`.
Frozen payment_test.go: 733 lines, SHA-256
`f3e2188ebd2b3706e27903c26f6f8064dc5bf97e84d14b5015b82ef91b5314a2`.
All eight other frozen inputs match. The backup and current main.go both match the
accepted hash. The reviewer inspected source restoration, all three focused logs,
and the full race JSON through a read-only parser. No reviewer test execution occurred.

Evidence: [Hermes integrated green 01](BBGO-PAY-002-GREEN-01.md).
All four retained log hashes match that record exactly. The recorded commands use
the accepted cached Go 1.27.0 executable and offline settings with bounded execution.
Shell exit statuses and whole-command timestamps are executor-reported metadata.

## Accepted results

- Integrated focused green: both daemon behavior tests pass, exit 0, package 0.153s.
- Registration-removal falsification: executor reports the exact original 219-line
  source hash; both tests fail on unsupported payment protocol, exit 1, package 0.067s.
  There is no compile, readiness, fixture, or cleanup error in the retained output.
- Restored focused green: accepted source restored exactly, both tests pass, exit 0,
  package **0.154s**. The current-task report's earlier 0.153s value was a transcription
  error; the retained Stage 3 log is authoritative.
- Race suite: all five package results pass, with zero failed/skipped test events and
  no race/panic/timeout/cleanup diagnostics in the retained JSON.

Corrected race inventory, excluding package-level pass events:

| Package | Top-level test entries | Subtest entries | Total test pass entries | Package seconds |
| --- | ---: | ---: | ---: | ---: |
| api | 5 | 0 | 5 | 1.321 |
| cmd/bitbookd | 4 | 8 | 12 | 5.591 |
| direct | 6 | 4 | 10 | 1.250 |
| network | 10 | 2 | 12 | 1.818 |
| payment | 57 | 221 | 278 | 2.421 |

The original evidence counted one package-level pass as an additional top-level test
in every package. This review corrects the summary without changing the raw evidence
or requiring another run. Totals are 82 top-level entries and 235 subtest entries.
The daemon total includes the guarded child helper; the payment total includes six
fuzz entrypoints executing ordinary seeds. These are not native fuzz campaigns.

## Acceptance boundary

The accepted behavior is daemon startup registration using the existing identity and
datastore, real signed request delivery, duplicate idempotence, persistence after
restart before resend, and remote wrong-payer rejection without invalid persistence.
The six-line production diff gives payment cleanup the required defer order before
shared node/datastore close. Successful cleanup is supported by the test paths and
absence of cleanup diagnostics, not an independent host-wide process audit.

The payment core, wire parsing, cryptography, dependencies, and build configuration
are unchanged; existing accepted security reviews retain their scope and exceptions.
The staged-content secret scan below covers this publication's new content. This is
not approval of coin custody/settlement, payment HTTP endpoints, desktop integration,
public-peer deployment, or a release binary. Those remain future task contracts.

## Publication authority

[Hermes publication 01](../handoff/HERMES_BBGO_PAY_002_PUBLISH_01.md) authorizes only
the exact listed source/governance/review paths, a pinned redacted staged-content
secret scan, one normal commit/push to origin master, and read-only observation of
that commit's CI. The owner-requested Hermes role replacement and Sr Dev focused-test
policy changes are included; historical Luna evidence is unchanged.
Any secret finding, source mismatch, unexpected staged path, remote divergence, or
failed publication gate stops publication. No new implementation or broad local
retesting is authorized. Codex reviews publication/CI results afterward.
