# BitBook Daemon Testing Strategy

This policy applies SQLite's reliability-oriented testing strategy, as adopted by Keel,
to BitBook's daemon and distributed protocols. Automated evidence, not reviewer
intuition, is the primary bug-finding layer. SQLite's scale and 100% branch coverage are
inspirations, not claims about this repository's current maturity.

## Standing Rules

1. **Tests lead implementation.** Every behavior change starts with a focused test that
   fails for the intended reason. The active ticket names the authorized red and green
   commands. Production code follows only after the red result is understood.
2. **Falsify important tests before trusting them.** Temporarily suppress or break the
   mechanism under test and prove the test fails. Do not commit the falsification change;
   report the method and result. A test that passes with the mechanism disabled proves
   nothing.
3. **One regression test per bug.** A bug fix is incomplete until a test reproduces the
   bug before the fix and passes after it.
4. **Assert non-vacuous outcomes.** Tests must prove the exercised path can produce the
   claimed event or state. A polling loop that times out with zero work, an empty result,
   or an ignored callback is not success unless emptiness is the behavior under test.
5. **Exercise the real path.** A discovery test may not manually dial a peer; a
   replication test may not pre-seed the answer; an API test may not bypass the handler
   whose behavior it claims to prove.

## Required Techniques

Use these where the ticket touches the corresponding boundary:

- native Go fuzzing for parsers, wire formats, record decoding, API inputs, and other
  attacker-controlled data;
- boundary tests immediately below, at, and above defined size, count, time, and numeric
  limits;
- property tests for determinism, round trips, ordering, idempotence, monotonicity,
  signature verification, privacy invariants, and bounded resource behavior;
- failure injection for disconnects, timeouts, truncated frames, unavailable storage,
  partial writes, cancellation, and restart recovery;
- compound-failure tests when another fault occurs during cleanup or recovery;
- in-process multi-node tests for discovery, propagation, reconciliation, partition,
  node loss, and eventual convergence; and
- deterministic fixtures for API compatibility and malformed or hostile inputs.

Security- and protocol-critical results should have an independent oracle where
practical: a canonical fixture, a second implementation, or an invariant checked by a
separate path. Tests must also detect leaked goroutines, file descriptors, sockets, and
temporary state when the touched boundary owns those resources.

Tests must be offline, reproducible, credential-free, and independent of public peers,
wall-clock luck, or mutable third-party services.

## Ticket and Review Contract

Each implementation ticket must state:

- the invariant or user-visible behavior being proved;
- authorized test paths before production paths;
- the exact targeted red command and expected failure;
- the exact targeted green command and broader acceptance commands;
- how at least one high-value test will be falsified; and
- relevant fuzz, property, failure, race, and multi-node coverage.

If dependencies, build inputs, network parsing, cryptography, storage, or release content
can change, the ticket must also name the applicable security scans, their exact commands,
and the finding threshold that blocks acceptance.

The ticket-authorized implementation developer authors bounded test source before
production source, but does not execute tests. Jr Dev — Hermes integrates the test-only
drop first, records the expected red result, integrates the production drop, runs the
targeted and broader acceptance commands, and publishes the implementation evidence.
The reviewer independently inspects the tests, rejects tautological or shortcut proofs,
and accepts or rejects the integrated evidence.

## CI and Coverage

- Run maintained packages under the race detector where concurrency is involved.
- Keep fuzz seeds as ordinary regression inputs and run fuzz targets for a bounded time.
- Coverage instrumentation tests the test suite. Re-run release-critical behavior in the
  configuration actually delivered to users.
- Coverage is a ratchet, not a vanity target. Record meaningful per-package floors for
  maintained security, protocol, storage, and social packages; raise them as tests land.
  Never lower a floor without an explicit rationale in the governing ticket and review.
- Do not build release binaries on every change merely to claim test coverage. Native
  packaging remains an explicit release or manual verification concern.

Human or model review remains necessary for architecture and semantics, but is not a
substitute for executable tests.

## Security and Supply-Chain Evidence

- Scan Go source and resolved dependencies with pinned, ecosystem-appropriate tools.
  Scan committed content for secrets before accepting a change.
- Establish a reviewed baseline for inherited findings. New findings fail the ratchet;
  critical or plausibly exploitable findings require immediate review regardless of the
  baseline.
- A suppression requires a ticketed rationale, owner, affected version or path, and
  expiry or removal condition. Severity labels alone do not replace exploitability
  review.
- Generate a machine-readable SPDX or CycloneDX SBOM from the exact resolved inputs of
  each release. Attach it to the release and scan the built artifact or its SBOM before
  publication. Routine pushes do not need to regenerate release SBOMs.
- Pin scanner and SBOM-generator versions. Preserve their reports as CI or release
  evidence. Network access is limited to fetching tools and signed advisory data; tests
  must not depend on a mutable service response.

Lineage: [How SQLite Is Tested](https://www.sqlite.org/testing.html).
