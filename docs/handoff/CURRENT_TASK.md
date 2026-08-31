# Current Task

Ticket: BBGO-PAY-001

State: SOL TEST CORRECTION ACCEPTED — LUNA EXPECTED RED AUTHORIZED

Reviewer: Lead Engineer/Reviewer — Codex at XHigh

Integration actor: Jr Dev — Codex Luna (`gpt-5.6-luna`)

Production source baseline: `0560b6426b9af29a16a151dacc7c2f3021a3dc0d`

Governance head before this handoff: `1a4af21dd2f2266059088e809d17af4cea9ef43f`

Active handoff: [CODEX_LUNA_BBGO_PAY_001_EXPECTED_RED.md](CODEX_LUNA_BBGO_PAY_001_EXPECTED_RED.md)

[BBGO-PAY-001](../../tickets/BBGO-PAY-001.md) remains the only authorized task in this
repository. Grok Build authored the original seven-path test drop in the foreground;
Codex XHigh rejected five files before execution; Codex Sol corrected only those five
files. The final 49 ordinary tests and six fuzz entrypoints are accepted in
[BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md](../testing/BBGO-PAY-001-TEST-SOURCE-REVIEW-02.md).
The desktop oracle remains byte-identical, the payment protocol test remains frozen, and
the full drop is format-clean.

Luna may now perform only the two expected-red commands and the evidence/integration Git
work in the active handoff. No production implementation, module/lock change, broad
suite, public network, wallet, rate, transaction, hardware, device, release binary, or
SBOM is authorized. Codex XHigh must review the expected-red result before authorizing
production source.

BBGO-SEC-001 and BBGO-SEC-002 remain accepted. Their existing reviewed exceptions and
re-review dates are unchanged. `../go-ipfs` is deprecated and receives no wallet work.
