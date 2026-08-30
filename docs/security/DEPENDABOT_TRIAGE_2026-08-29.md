# Dependabot Triage — 2026-08-29

Repository baseline inspected: `f62d7879fca86db6080ea2ce4e83f6709f1351eb`

Reviewer: Lead Engineer/Reviewer — Codex

## Finding

GitHub reports four open moderate Dependabot alerts. All four resolve to the same direct
dependency, `requests==2.20.0`, in `qa/requirements.txt`:

| Alert | Advisory | Affected range | Fixed version |
|---|---|---:|---:|
| 1 | GHSA-j8r2-6x86-q33q / CVE-2023-32681 | `>=2.3.0,<2.31.0` | `2.31.0` |
| 2 | GHSA-9wx4-h78v-vm56 / CVE-2024-35195 | `<2.32.0` | `2.32.0` |
| 3 | GHSA-9hjg-9r4m-mvj7 / CVE-2024-47081 | `<2.32.4` | `2.32.4` |
| 4 | GHSA-gc5v-m9x4-r6x2 / CVE-2026-25645 | `<2.33.0` | `2.33.0` |

The manifest pins Requests 2.20.0 and `python-bitcoinlib==0.8.0`. The `qa/` tree contains
90 tracked files (about 800 KiB) implementing inherited OpenBazaar marketplace tests:
listings, purchases, escrow, refunds, disputes, moderation, Bitcoin, and Ethereum. It is
not referenced by `.github/workflows/go.yml`, the maintained `modern/` module, or current
BitBook documentation.

## Exploitability and Product Relevance

The alerts do not describe the Go daemon's resolved dependency graph. They affect an
obsolete Python QA harness which would execute only when manually invoked. Some advisory
preconditions may not occur in that harness, but retaining and upgrading it would keep
unused marketplace/payment code and dependencies inside a social-only product.

## Decision

Retire the complete inherited `qa/` tree under `BBGO-SEC-002`; do not dismiss alerts and
do not upgrade Requests merely to preserve unused marketplace tests. Add a small
repository regression test proving the retired tree is absent. Keep the maintained Go
tests and the social-only boundary tests as the functional safety net.

This is deliberately narrower than deleting the complete legacy Go/GX tree. The root Go
tree still participates in compatibility compilation and social-only boundary tests, so
its removal requires an inventory and parity ticket. `BBGO-SEC-001` remains responsible
for source, dependency, secret, and SBOM evidence for `modern/` after this alert cleanup.

## Evidence Source

Alert metadata was read from GitHub's Dependabot alerts API on 2026-08-29. No alert was
dismissed, mutated, or marked fixed during triage.

## Resolution

`BBGO-SEC-002` removed the obsolete manifest and complete inherited marketplace QA tree.
GitHub dependency-graph run `33283910919` succeeded and the Dependabot API then reported
zero open alerts. The alerts closed from repository state; none was dismissed.
