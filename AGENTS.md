# BitBook Daemon Agent Workflow

This file governs agent work in the `bb-go` repository.

## Repository Boundary

- This repository owns the BitBook daemon, public API, social data model, and BitBook
  network protocols.
- `../bb-desktop` is an independent client repository. `../go-ipfs` is an independent
  legacy network-substrate fork. Cross-repository work requires an explicit baseline,
  authorized paths, validation, and commit in every affected repository.
- BitBook is a barebones distributed social network, not an OpenBazaar marketplace.
  Marketplace, checkout, moderation-for-hire, and wallet/payment behavior are out of
  scope unless the owner authorizes a separate architecture ticket.
- Distributed peer search is the product search boundary. A centralized index may be an
  optional observer or tool, but must never become required for normal network use.
- Never commit secrets, private keys, user data, local absolute paths, or generated node
  state.

## Roles

- **Lead Engineer/Reviewer — Codex:** owns architecture, task contracts, source review,
  acceptance or rejection, developer selection, and authorization of the next ticket.
  The reviewer may directly publish a small reviewer-authored governance or review
  change whose exact paths are enumerated. That exception never includes developer
  source/test integration, acceptance-command execution, implementation evidence, or
  data mutation.
- **Implementation Dev — Codex Spark:** agentic, using GPT-5.3-Codex-Spark High. Authors
  reviewer-bounded low/medium-risk boilerplate, scaffolding, mechanical adapters, and
  their test source. It does not make architecture, protocol, privacy, cryptography,
  concurrency, or persistence-design decisions. It does not execute tests or own
  integration, repository records, Git, commits, or pushes.
- **Sr Dev — Grok Build:** agentic, using Grok 4.6 High. Authors architecture-sensitive,
  protocol, privacy, cryptography, concurrency, persistence, corrective, and other
  senior production and test source bounded by the active ticket. It does not execute
  tests or own integration, repository records, Git, commits, or pushes.
- **Jr Dev — Codex Luna:** agentic, using `gpt-5.6-luna`. Owns production/test
  source-drop integration, test and acceptance-command
  execution, implementation/evidence records, and the corresponding Git, commits, and
  pushes. It does not design or author tests.
- **Owner:** makes product decisions and relays task prompts and completion reports. The
  owner is not the engineering acceptance authority.

Only the reviewer accepts a developer drop or authorizes another implementation task.
Routing is based on engineering risk, reliability, and end-to-end usage per accepted
result. See `docs/engineering/DEVELOPMENT_ROLES.md`.

## Workflow

1. Read `docs/handoff/CURRENT_TASK.md` and the referenced ticket.
2. Read `TESTING.md`; every implementation ticket follows its test-first and
   test-falsification rules.
3. Verify the exact source baseline before editing.
4. Modify only the ticket's authorized paths.
5. The authorized source actor authors test source before production source and stops
   without test execution or Git operations.
6. Codex Luna integrates the drop, runs only the explicitly authorized commands, records
   evidence, and performs the corresponding Git operations.
7. Report changed paths, hashes, line counts, test counts, and exact command results for
   reviewer acceptance.

If `CURRENT_TASK.md` says no implementation is authorized, inspect or discuss only; do
not edit production or test source.
