# Payment Protocol Routing

This document records the model assignment for wallet-adjacent daemon work. It changes no
daemon behavior and does not make the daemon a wallet.

Reviewer: Lead Engineer/Reviewer — Codex at XHigh

## Assignment

- Grok Build authored the initial bounded `BBGO-PAY-001` test source after BBD-WAL-001
  and the accepted fixture froze the object schema and negative rate/wallet contract.
- Codex Sol corrected the trust-critical tests and owns the production source because it
  implements signature trust and validation order, closed-domain canonicalization,
  durable replay and persistence, attacker-controlled framing, and handler concurrency.
  The exact re-route and source boundary are frozen in the active ticket and handoff.
- Codex Spark may later own only mechanical API response/view-model/table scaffolding
  after the signed payment service is accepted. Spark does not own JCS, identity binding,
  signatures, transport verification, persistence, replay, or framing.
- Codex Luna integrates every accepted source drop, runs red/green/race/fuzz/security
  commands, records evidence, and owns Git.

The desktop broker remains in `../bb-desktop`; `../go-ipfs` is deprecated. The daemon
never receives coin keys, wallet credentials, quote-provider access, transaction bytes,
or broadcast authority.
