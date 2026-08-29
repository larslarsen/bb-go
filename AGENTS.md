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
  independent validation, acceptance or rejection, commits, and pushes. After this
  governance baseline, the reviewer does not author production or test implementation
  for work delegated to the senior developer. The reviewer may author and publish
  governance, architecture, task, and review documents.
- **Sr Dev — Grok Build:** agentic, using Grok 4.6 High. Authors only the production and
  test source bounded by the active ticket. It may run only commands explicitly listed
  in that ticket. It does not edit governance or review records, change architecture,
  use Git, commit, push, access secrets, or widen scope.
- **Owner:** makes product decisions and relays task prompts and completion reports. The
  owner is not the engineering acceptance authority.

Only the reviewer accepts a developer drop or authorizes another implementation task.

## Workflow

1. Read `docs/handoff/CURRENT_TASK.md` and the referenced ticket.
2. Read `TESTING.md`; every implementation ticket follows its test-first and
   test-falsification rules.
3. Verify the exact source baseline before editing.
4. Modify only the ticket's authorized paths.
5. Run only its explicitly authorized commands.
6. Report changed paths, hashes, line counts, test counts, and exact command results.
7. Stop for reviewer inspection without Git operations.

If `CURRENT_TASK.md` says no implementation is authorized, inspect or discuss only; do
not edit production or test source.
