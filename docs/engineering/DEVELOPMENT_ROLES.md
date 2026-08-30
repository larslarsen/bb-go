# Development Roles and Routing Policy

This document records the agent roles used for BitBook daemon work. It is governance
only and changes no code, protocol, data, or acceptance state.

## Roles

- **Lead Engineer/Reviewer — Codex:** fixes architecture and task boundaries, selects the
  minimum-usage capable source actor, reviews integrated work, accepts or rejects it, and
  authorizes the next ticket. It may directly publish only a small reviewer-authored
  governance/review change with exact authorized paths.
- **Implementation Dev — Codex Spark:** uses GPT-5.3-Codex-Spark High for bounded
  low/medium-risk boilerplate, scaffolding, mechanical adapters, and their test source.
  It does not decide architecture or sensitive semantics and does not execute tests,
  integrate, maintain records, or use Git.
- **Sr Dev — Grok Build:** uses Grok 4.6 High for architecture-sensitive, protocol,
  privacy, cryptography, concurrency, persistence, corrective, and other senior source
  and test-source work. It does not execute tests, integrate, maintain records, or use
  Git.
- **Jr Dev — Codex Luna:** uses `gpt-5.6-luna`. It owns source-drop integration, test and
  acceptance-command execution,
  implementation/evidence records, and the corresponding Git, commit, and push work. It
  does not design or author tests.
- **Owner:** makes product decisions and relays one-way prompts, reports, repository
  hashes, URLs, and source drops. The owner is not an engineering acceptance authority.

## Routing

1. The reviewer writes the bounded ticket and selects exactly one source actor.
2. Codex Spark receives mechanical work whose design and semantics are already fixed.
3. Grok Build receives senior or corrective work where accepted-result risk dominates
   nominal model cost.
4. Codex Luna integrates every developer drop, runs the ticket's commands, records evidence,
   and publishes the resulting Git change.
5. The reviewer alone accepts or rejects the result and authorizes what follows.

Selection is based on engineering risk, reliability, and total usage through an accepted
result—not nominal per-token price. Roles do not widen an active ticket's paths or
authority.
