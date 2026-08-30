# Payment Protocol Routing

This document records the model assignment for wallet-adjacent daemon work. It changes no
daemon behavior and does not make the daemon a wallet.

Reviewer: Lead Engineer/Reviewer — Codex at XHigh

## Assignment

- `BBGO-PAY-001` is a bounded Grok Build ticket because BBD-WAL-001 and accepted
  BBD-WAL-002 fixtures freeze the object schemas, domain separators, validation order,
  trust direction, and negative rate/wallet contract. Grok authors tests first and stops.
- Codex Sol is reserved for a correction that changes signature trust, canonicalization,
  durable replay semantics, attacker-controlled framing, or concurrency beyond the fixed
  ticket. The reviewer must explicitly re-route such a correction before source changes.
- Codex Spark may later own only mechanical API response/view-model/table scaffolding
  after the signed payment service is accepted. Spark does not own JCS, identity binding,
  signatures, transport verification, persistence, replay, or framing.
- Codex Luna integrates every accepted source drop, runs red/green/race/fuzz/security
  commands, records evidence, and owns Git.

The desktop broker remains in `../bb-desktop`; `../go-ipfs` is deprecated. The daemon
never receives coin keys, wallet credentials, quote-provider access, transaction bytes,
or broadcast authority.
