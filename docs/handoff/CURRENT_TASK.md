# Current Task

Ticket: BBGO-PAY-001

State: PARTIAL GREEN RECOVERED — LUNA FUZZ 2–6 CONTINUATION AUTHORIZED

Reviewer: Lead Engineer/Reviewer — Codex at XHigh

Source actor: Principal Dev — Codex Sol (`gpt-5.6-sol`, High)

Production source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Integrated frozen-test baseline: `403df23a63f413c11e13085719fc7e767c2f15be`

Active handoff: [CODEX_LUNA_BBGO_PAY_001_CONTINUE_FUZZ_01.md](CODEX_LUNA_BBGO_PAY_001_CONTINUE_FUZZ_01.md)

Evidence: [BBGO-PAY-001-EXPECTED-RED.md](../testing/BBGO-PAY-001-EXPECTED-RED.md)

Gap evidence: [BBGO-PAY-001-GAP-EXPECTED-RED-03.md](../testing/BBGO-PAY-001-GAP-EXPECTED-RED-03.md)

[BBGO-PAY-001](../../tickets/BBGO-PAY-001.md) remains the only authorized task in this
repository. Grok Build authored the original seven-path test drop in the foreground;
Codex XHigh rejected five files before execution; Codex Sol corrected only those five
files. The final 49 ordinary tests and six fuzz entrypoints are accepted in
[BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md](../testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md).
The desktop oracle remains byte-identical, the payment protocol test remains frozen, and
the full drop is format-clean.

The two authorized expected-red commands produced only missing-production API
diagnostics and are accepted in
[BBGO-PAY-001-EXPECTED-RED-REVIEW.md](../testing/BBGO-PAY-001-EXPECTED-RED-REVIEW.md).
Sol's seven-path production drop was rejected before execution in
[BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md](../testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-01.md).
Sol's three gap tests are statically accepted in
[BBGO-PAY-001-GAP-TEST-SOURCE-REVIEW-01.md](../testing/BBGO-PAY-001-GAP-TEST-SOURCE-REVIEW-01.md),
and the seven uncommitted production paths remain byte-identical to their rejected review
inventory. Luna's first focused run was rejected before test execution in
[BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-01.md](../testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-01.md)
because two frozen transport assertions bound two results from the four-result
`DecodePaymentStatus` API. Sol's exact two-line correction is accepted in
[BBGO-PAY-001-TEST-COMPILE-CORRECTION-REVIEW-01.md](../testing/BBGO-PAY-001-TEST-COMPILE-CORRECTION-REVIEW-01.md).
Luna's second attempt reached the two offline assertions but the restricted sandbox
denied the two-node test's ephemeral loopback bind, as recorded in
[BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-02.md](../testing/BBGO-PAY-001-GAP-EXPECTED-RED-ATTEMPT-02.md).
The final focused command ran with a loopback-only sandbox override and produced the
exact three intended missing-protection failures. Codex XHigh accepts that result in
[BBGO-PAY-001-GAP-EXPECTED-RED-REVIEW-03.md](../testing/BBGO-PAY-001-GAP-EXPECTED-RED-REVIEW-03.md).
Sol's typed UTF-8, 32-container parse bound, linked nonce separation, and linear
stored-record correction is accepted as a complete seven-path production drop in
[BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-02.md](../testing/BBGO-PAY-001-PRODUCTION-SOURCE-REVIEW-02.md).
Tidy, focused/full tests, three falsifications, vet, race, and fuzz target 1 completed
successfully before Luna's agent turn stuck at fuzz target 2. The recovered state and
non-duplicative boundary are recorded in
[BBGO-PAY-001-GREEN-RECOVERY-01.md](../testing/BBGO-PAY-001-GREEN-RECOVERY-01.md).
Luna may now run only fuzz targets 2–6. Security, Git integration, public network,
wallet, rate, transaction, hardware, device, release binary, SBOM, and other work remain
unauthorized.

BBGO-SEC-001 and BBGO-SEC-002 remain accepted. Their existing reviewed exceptions and
re-review dates are unchanged. `../go-ipfs` is deprecated and receives no wallet work.
